package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type configStore struct{ path string }

func defaultConfigPath() (string, error) {
	if root := os.Getenv("XDG_CONFIG_HOME"); root != "" {
		return filepath.Join(root, "filament", "filament.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "filament", "filament.yaml"), nil
}

func (s configStore) load() (configDocument, *yaml.Node, error) {
	doc := newDocument()
	data, err := os.ReadFile(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return doc, emptyYAMLDocument(), nil
	}
	if err != nil {
		return doc, nil, fmt.Errorf("read config: %w", err)
	}
	var root yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil {
		return doc, nil, fmt.Errorf("parse %s: %w", s.path, err)
	}
	if err := yaml.Unmarshal(data, &root); err != nil {
		return doc, nil, fmt.Errorf("parse YAML tree: %w", err)
	}
	doc.normalize()
	return doc, &root, nil
}

func emptyYAMLDocument() *yaml.Node {
	root := &yaml.Node{Kind: yaml.DocumentNode}
	mapping := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	root.Content = []*yaml.Node{mapping}
	setMappingValue(mapping, "version", scalarNode(configVersion))
	setMappingValue(mapping, "sources", mappingNode())
	setMappingValue(mapping, "sinks", mappingNode())
	setMappingValue(mapping, "pipelines", mappingNode())
	return root
}

func (s configStore) put(section, name string, value any) error {
	_, root, err := s.load()
	if err != nil {
		return err
	}
	top := root.Content[0]
	sectionNode := ensureMapping(top, section)
	encoded := &yaml.Node{}
	if err := encoded.Encode(value); err != nil {
		return fmt.Errorf("encode %s %q: %w", section, name, err)
	}
	setMappingValue(sectionNode, name, encoded)
	return s.write(root)
}

func (s configStore) delete(section, name string) error {
	_, root, err := s.load()
	if err != nil {
		return err
	}
	sectionNode := ensureMapping(root.Content[0], section)
	for i := 0; i+1 < len(sectionNode.Content); i += 2 {
		if sectionNode.Content[i].Value == name {
			sectionNode.Content = append(sectionNode.Content[:i], sectionNode.Content[i+2:]...)
			return s.write(root)
		}
	}
	return fmt.Errorf("%s %q does not exist", singular(section), name)
}

func (s configStore) write(root *yaml.Node) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".filament-*.yaml")
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
	if err := os.Rename(tmpName, s.path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	committed = true
	return nil
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
		if parent.Content[i].Value == key {
			value.HeadComment = parent.Content[i+1].HeadComment
			value.LineComment = parent.Content[i+1].LineComment
			value.FootComment = parent.Content[i+1].FootComment
			parent.Content[i+1] = value
			return
		}
	}
	parent.Content = append(parent.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, value)
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
