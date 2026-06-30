// Package template renders `{{ scope.key }}` placeholders in manifest strings
// using a typed Scope assembled at request-build time.
//
// Supported scopes:
//
//	{{ config.<key> }}   — credentials / settings injected by registry
//	{{ parent.<key> }}   — captured fields from parent record (child resources)
//	{{ state.<key> }}    — watermark / checkpoint values
//	{{ env.<key> }}      — process environment (opt-in per manifest, future)
//	{{ cursor }}         — current pagination cursor (no sub-key)
//
// Default values: {{ config.key | default "fallback" }}
//
// # Worked example
//
//	scope := template.Scope{
//	    Config: map[string]string{"api_key": "sk_test_x"},
//	    Cursor: "page2",
//	}
//	out, err := template.Render("/v1/items?key={{ config.api_key }}&cursor={{ cursor }}", scope)
//	// out == "/v1/items?key=sk_test_x&cursor=page2"
//
// Hot paths (rendered per request) should Parse once and reuse the Parsed —
// see parser.go.
//
// # Validation at manifest load
//
// Use Validate at manifest-load time to catch syntax errors and unknown
// scopes before any request is built. AllowedScopes lists every legal scope
// name; subset slices restrict context-sensitive positions (e.g. auth params
// disallow cursor/parent — future work).
//
//	if err := template.Validate("{{ config.api_key }}", template.AllowedScopes); err != nil {
//	    // surfaces errs.ErrTemplateSyntax or "unknown scope" issues
//	}
//
// # Failure modes
//
//   - errs.ErrTemplateSyntax       — bad braces, unterminated default, unknown filter
//   - errs.ErrTemplateMissingKey   — render-time scope lookup missed (and no default)
//   - "cursor scope takes no sub-key" — `{{ cursor.foo }}` is rejected
package template

// Scope carries the per-request template resolution context.
// Any nil map is treated as empty.
type Scope struct {
	Config map[string]string
	Parent map[string]string
	State  map[string]string
	Env    map[string]string
	Cursor string
}

func (s Scope) lookup(scope, key string) (string, bool) {
	switch scope {
	case "config":
		v, ok := s.Config[key]
		return v, ok
	case "parent":
		v, ok := s.Parent[key]
		return v, ok
	case "state":
		v, ok := s.State[key]
		return v, ok
	case "env":
		v, ok := s.Env[key]
		return v, ok
	case "cursor":
		if key != "" {
			return "", false
		}
		return s.Cursor, s.Cursor != ""
	}
	return "", false
}
