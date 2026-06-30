package request

import "strings"

// MergeOverrides merges one or more `map[string]any` override maps into a base
// body template. Last-write-wins for duplicate keys. The base template must be
// nil or a `map[string]any`; other types are returned unchanged (overrides
// silently discarded).
//
// Used by the orchestration loop to combine pagination + incremental body
// overrides with the user-templated body before encoding.
func MergeOverrides(template any, overrides ...map[string]any) any {
	if len(overrides) == 0 {
		return template
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
		return template
	}
	for _, o := range overrides {
		for k, v := range o {
			setPath(base, k, v)
		}
	}
	return base
}

func setPath(dst map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	if len(parts) == 1 {
		dst[path] = value
		return
	}
	cur := dst
	for _, part := range parts[:len(parts)-1] {
		next, ok := cur[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[part] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = value
}
