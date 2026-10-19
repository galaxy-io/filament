package manifest

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

// ReadFS resolves local resource and field-set fragments into one manifest.
// Includes are relative to the including file and confined to the root
// manifest's directory. Fragments cannot override connection or config settings.
// Order is deterministic: root declarations, then includes in declaration order.
func ReadFS(files fs.FS, name string) ([]byte, error) {
	rootFS, err := fs.Sub(files, path.Dir(name))
	if err != nil {
		return nil, err
	}
	loader := fragmentLoader{files: rootFS, visited: map[string]bool{}, active: map[string]bool{}}
	root, err := loader.read(path.Base(name), true, 0)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(root)
}

// LoadFS loads either a single file or a composed manifest from an fs.FS.
func LoadFS(files fs.FS, name string) (*Manifest, error) {
	data, err := ReadFS(files, name)
	if err != nil {
		return nil, err
	}
	m, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return m, nil
}

type fragmentLoader struct {
	files           fs.FS
	visited, active map[string]bool
	bytes           int
}

func (l *fragmentLoader) read(name string, root bool, depth int) (*yaml.Node, error) {
	if depth > 16 || len(l.visited) >= 128 {
		return nil, fmt.Errorf("%s: manifest include limit exceeded", name)
	}
	if l.active[name] {
		return nil, fmt.Errorf("%s: manifest include cycle", name)
	}
	if l.visited[name] {
		return nil, fmt.Errorf("%s: manifest fragment included more than once", name)
	}
	l.active[name], l.visited[name] = true, true
	defer delete(l.active, name)
	if !root {
		current := "."
		for _, component := range strings.Split(name, "/") {
			entries, err := fs.ReadDir(l.files, current)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", name, err)
			}
			for _, entry := range entries {
				if entry.Name() == component && entry.Type()&fs.ModeSymlink != 0 {
					return nil, fmt.Errorf("%s: included fragments cannot be symlinks", name)
				}
			}
			current = path.Join(current, component)
		}
	}
	data, err := fs.ReadFile(l.files, name)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", name, err)
	}
	l.bytes += len(data)
	if l.bytes > 32<<20 {
		return nil, fmt.Errorf("manifest includes exceed 32 MiB")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: manifest must be an object", name)
	}
	node := doc.Content[0]
	// Decode once to reject duplicate YAML keys and invalid aliases before merge.
	var check map[string]any
	if err := node.Decode(&check); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	includes, err := fragmentIncludes(node, name, root)
	if err != nil {
		return nil, err
	}
	for _, ref := range includes {
		next := path.Clean(path.Join(path.Dir(name), ref))
		if path.IsAbs(ref) || strings.ContainsAny(ref, "\\:") || !fs.ValidPath(next) || next == "." {
			return nil, fmt.Errorf("%s: include %q escapes the manifest directory or is not a local path", name, ref)
		}
		if path.Ext(next) != ".yaml" && path.Ext(next) != ".yml" {
			return nil, fmt.Errorf("%s: include %q must be YAML", name, ref)
		}
		fragment, err := l.read(next, false, depth+1)
		if err != nil {
			return nil, err
		}
		if err := mergeFragment(node, fragment, next); err != nil {
			return nil, err
		}
	}
	return node, nil
}

func fragmentIncludes(node *yaml.Node, name string, root bool) ([]string, error) {
	var includes []string
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i].Value, node.Content[i+1]
		if !root && key != "includes" && key != "resources" && key != "field_sets" {
			return nil, fmt.Errorf("%s: fragments may only declare includes, resources, and field_sets (got %s)", name, key)
		}
		if key == "includes" {
			if value.Kind != yaml.SequenceNode {
				return nil, fmt.Errorf("%s: includes must be a list of paths", name)
			}
			for _, entry := range value.Content {
				if entry.Kind != yaml.ScalarNode || entry.Tag != "!!str" || entry.Value == "" {
					return nil, fmt.Errorf("%s: includes must contain nonempty paths", name)
				}
				includes = append(includes, entry.Value)
			}
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			i -= 2
		}
	}
	return includes, nil
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

func mergeFragment(root, fragment *yaml.Node, name string) error {
	for _, key := range []string{"field_sets", "resources"} {
		incoming := mappingValue(fragment, key)
		if incoming == nil {
			continue
		}
		kind := yaml.MappingNode
		if key == "resources" {
			kind = yaml.SequenceNode
		}
		if incoming.Kind != kind {
			return fmt.Errorf("%s: invalid %s shape", name, key)
		}
		target := mappingValue(root, key)
		if target == nil {
			target = &yaml.Node{Kind: kind, Tag: incoming.Tag}
			root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, target)
		}
		if target.Kind != kind {
			return fmt.Errorf("invalid root %s shape", key)
		}
		names := map[string]bool{}
		resourceName := func(n *yaml.Node) string {
			if n.Kind != yaml.MappingNode {
				return ""
			}
			v := mappingValue(n, "name")
			if v == nil {
				return ""
			}
			return v.Value
		}
		if key == "field_sets" {
			for i := 0; i < len(target.Content); i += 2 {
				names[target.Content[i].Value] = true
			}
			for i := 0; i < len(incoming.Content); i += 2 {
				n := incoming.Content[i].Value
				if names[n] {
					return fmt.Errorf("%s: duplicate field set %q", name, n)
				}
				names[n] = true
			}
		} else {
			for _, r := range target.Content {
				names[resourceName(r)] = true
			}
			for _, r := range incoming.Content {
				n := resourceName(r)
				if n != "" && names[n] {
					return fmt.Errorf("%s: duplicate resource %q", name, n)
				}
				names[n] = true
			}
		}
		target.Content = append(target.Content, incoming.Content...)
	}
	return nil
}
