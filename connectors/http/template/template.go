package template

import (
	"strings"
)

// Render parses s and renders it against scope. Convenience wrapper —
// hot paths should call Parse once and reuse the Parsed.
//
// Returns errs.ErrTemplateSyntax for parse errors, errs.ErrTemplateMissingKey
// for unresolved placeholders without a default.
func Render(s string, scope Scope) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	parsed, err := Parse(s)
	if err != nil {
		return "", err
	}
	return parsed.Render(scope)
}

// RenderAny walks v recursively rendering string leaves. Maps and slices are
// rebuilt with rendered children. Non-string scalars pass through unchanged.
func RenderAny(v any, scope Scope) (any, error) {
	switch x := v.(type) {
	case string:
		return Render(x, scope)
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			rv, err := RenderAny(val, scope)
			if err != nil {
				return nil, err
			}
			out[k] = rv
		}
		return out, nil
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			rv, err := RenderAny(val, scope)
			if err != nil {
				return nil, err
			}
			out[i] = rv
		}
		return out, nil
	case map[string]string:
		out := make(map[string]string, len(x))
		for k, val := range x {
			rv, err := Render(val, scope)
			if err != nil {
				return nil, err
			}
			out[k] = rv
		}
		return out, nil
	default:
		return v, nil
	}
}

// RenderStringMap is a small convenience: render every value of m through
// Render, returning a fresh map (never mutates m).
func RenderStringMap(m map[string]string, scope Scope) (map[string]string, error) {
	if m == nil {
		return nil, nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		rv, err := Render(v, scope)
		if err != nil {
			return nil, err
		}
		out[k] = rv
	}
	return out, nil
}

// AllowedScopes lists every scope name the template engine recognises.
// Resource-level positions (path, query, headers, body) accept the full set
// because they're rendered per-request with full context.
//
// Subsets restrict context-sensitive positions — pass these to
// Parsed.Validate or Validate at manifest-load time to fail loud on a
// template that references a scope unavailable in its position.
var AllowedScopes = []string{"config", "parent", "state", "env", "cursor"}

// AuthScopes is the subset legal in connection.auth params. Auth runs at
// connection scope, not per-record, so `parent` (no parent record exists at
// connection-build time) and `cursor` (no pagination state yet) are forbidden.
// Catching these at manifest load surfaces a typo'd config key as a startup
// error rather than as a runtime template-render failure mid-extraction.
var AuthScopes = []string{"config", "state", "env"}
