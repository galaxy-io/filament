package local

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"gopkg.in/yaml.v3"
)

// Store reads and atomically updates a local YAML configuration document.
type Store struct {
	Path string
}

// Load reads the typed document and its comment-preserving YAML tree.
func (s Store) Load() (model.Document, *yaml.Node, error) {
	doc, root, _, err := s.LoadSnapshot()
	return doc, root, err
}

// LoadSnapshot reads the document together with an opaque revision of the
// bytes from which it was decoded.
func (s Store) LoadSnapshot() (model.Document, *yaml.Node, string, error) {
	doc := model.NewDocument()
	data, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc, emptyYAMLDocument(), revision(nil), nil
	}
	if err != nil {
		return doc, nil, "", fmt.Errorf("read config: %w", err)
	}
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return doc, nil, "", fmt.Errorf("parse %s: %w", s.Path, err)
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return doc, nil, "", fmt.Errorf("parse YAML tree: %w", err)
	}
	doc.Normalize()
	return doc, &root, revision(data), nil
}

// Create adds a named value only when it does not already exist.
func (s Store) Create(section, name string, value any) error {
	_, root, _, err := s.LoadSnapshot()
	if err != nil {
		return err
	}
	top := root.Content[0]
	sectionNode := ensureMapping(top, section)
	if mappingContains(sectionNode, name) {
		return fmt.Errorf("%s %q already exists", singular(section), name)
	}
	return s.set(section, name, value, root, sectionNode)
}

// Update replaces an existing named value when its revision is current.
func (s Store) Update(section, name string, value any, expectedRevision string) error {
	_, root, currentRevision, err := s.LoadSnapshot()
	if err != nil {
		return err
	}
	sectionNode := ensureMapping(root.Content[0], section)
	if !mappingContains(sectionNode, name) {
		return fmt.Errorf("%s %q does not exist", singular(section), name)
	}
	if err := checkRevision(expectedRevision, currentRevision); err != nil {
		return err
	}
	return s.set(section, name, value, root, sectionNode)
}

func (s Store) set(section, name string, value any, root, sectionNode *yaml.Node) error {
	encoded := &yaml.Node{}
	if err := encoded.Encode(value); err != nil {
		return fmt.Errorf("encode %s %q: %w", section, name, err)
	}
	setMappingValue(sectionNode, name, encoded)
	return s.Write(root)
}

// Delete removes one named value from a top-level document section.
func (s Store) Delete(section, name, expectedRevision string) error {
	_, root, currentRevision, err := s.LoadSnapshot()
	if err != nil {
		return err
	}
	if err := checkRevision(expectedRevision, currentRevision); err != nil {
		return err
	}
	sectionNode := ensureMapping(root.Content[0], section)
	for i := 0; i+1 < len(sectionNode.Content); i += 2 {
		if sectionNode.Content[i].Value == name {
			sectionNode.Content = append(sectionNode.Content[:i], sectionNode.Content[i+2:]...)
			return s.Write(root)
		}
	}
	return fmt.Errorf("%s %q does not exist", singular(section), name)
}

func checkRevision(expected, current string) error {
	if expected == "" {
		return fmt.Errorf("entity revision is required")
	}
	if expected != current {
		return fmt.Errorf("entity changed since it was read; reload and retry")
	}
	return nil
}

func revision(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("local:%x", sum[:])
}

// Write atomically replaces the local document while preserving owner-only
// permissions.
func (s Store) Write(root *yaml.Node) error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.Path), ".filament-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		return err
	}
	encoder := yaml.NewEncoder(tmp)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.Path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	committed = true
	return nil
}

func emptyYAMLDocument() *yaml.Node {
	root := &yaml.Node{Kind: yaml.DocumentNode}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	root.Content = []*yaml.Node{mapping}
	setMappingValue(mapping, "version", scalarNode(model.ConfigVersion))
	setMappingValue(mapping, "sources", mappingNode())
	setMappingValue(mapping, "sinks", mappingNode())
	setMappingValue(mapping, "pipelines", mappingNode())
	return root
}

func ensureMapping(parent *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			if parent.Content[i+1].Kind != yaml.MappingNode {
				parent.Content[i+1] = mappingNode()
			}
			return parent.Content[i+1]
		}
	}
	value := mappingNode()
	setMappingValue(parent, key, value)
	return value
}

func setMappingValue(parent *yaml.Node, key string, value *yaml.Node) {
	// An empty mapping may have been written as `{}`. Once populated, switch it
	// back to block style so targeted additions retain the document's readable
	// declaration shape instead of collapsing the whole section onto one line.
	parent.Style = 0
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value != key {
			continue
		}
		value.HeadComment = parent.Content[i+1].HeadComment
		value.LineComment = parent.Content[i+1].LineComment
		value.FootComment = parent.Content[i+1].FootComment
		parent.Content[i+1] = value
		return
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
}

func mappingContains(parent *yaml.Node, key string) bool {
	for i := 0; i+1 < len(parent.Content); i += 2 {
		if parent.Content[i].Value == key {
			return true
		}
	}
	return false
}

func mappingNode() *yaml.Node {
	return &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
}

func scalarNode(value any) *yaml.Node {
	n := &yaml.Node{}
	_ = n.Encode(value)
	return n
}

func singular(section string) string {
	if len(section) > 1 && section[len(section)-1] == 's' {
		return section[:len(section)-1]
	}
	return section
}
