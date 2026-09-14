package request

import (
	"fmt"
	"strconv"
	"strings"
)

// MergeOverrides merges one or more `map[string]any` override maps into a base
// body template. Last-write-wins for duplicate keys. The base template must be
// nil or a `map[string]any`; other types are returned unchanged (overrides
// silently discarded).
//
// Used by the orchestration loop to combine pagination + incremental body
// overrides with the user-templated body before encoding. Paths may traverse
// existing array elements; invalid array indices return an error.
func MergeOverrides(template any, overrides ...map[string]any) (any, error) {
	if len(overrides) == 0 {
		return template, nil
	}
	var base map[string]any
	switch t := template.(type) {
	case nil:
		base = map[string]any{}
	case map[string]any:
		base = make(map[string]any, len(t)+len(overrides))
		for k, v := range t {
			base[k] = v
		}
	default:
		// Not a mergeable shape (e.g. raw string, slice). Drop overrides.
		return template, nil
	}
	for _, o := range overrides {
		for k, v := range o {
			if err := setPath(base, k, v); err != nil {
				return nil, err
			}
		}
	}
	return base, nil
}

func setPath(dst map[string]any, path string, value any) error {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		dst[path] = value
		return nil
	}
	var cur any = dst
	for _, part := range parts[:len(parts)-1] {
		switch node := cur.(type) {
		case map[string]any:
			next := node[part]
			switch next.(type) {
			case map[string]any, []any:
			default:
				next = map[string]any{}
				node[part] = next
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(part)
			if err != nil || idx < 0 || idx >= len(node) {
				return fmt.Errorf("body override %q: invalid array index %q", path, part)
			}
			cur = node[idx]
		}
	}
	if node, ok := cur.(map[string]any); ok {
		node[parts[len(parts)-1]] = value
		return nil
	}
	return fmt.Errorf("body override %q: expected object", path)
}
