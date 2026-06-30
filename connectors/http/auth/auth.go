// Package auth provides a pluggable Authenticator interface for the HTTP
// connector. Implementations are registered by string `kind` and constructed
// from a free-form parameter map (typically the manifest's `connection.auth`
// fields minus `type`).
//
// Built-in kinds: bearer, header, query, basic, oauth2_cc, hmac, chain.
//
// # Worked example
//
// A bearer-token auth backed by a `{{ config.api_key }}` placeholder:
//
//	a, err := auth.Build("bearer", map[string]any{
//	    "token": "{{ config.api_key }}",
//	}, map[string]string{"api_key": "secret"})
//	if err != nil { return err }
//	a.Apply(ctx, req, scope) // sets Authorization: Bearer secret
//
// Build validates that every `{{ config.* }}` placeholder used in templated
// params resolves against the supplied creds — a typo'd config key fails
// connector startup rather than at first request. Pure-text params skip the
// check. Unknown kinds return an explicit error naming the kind.
//
// # Failure modes
//
//   - Unknown kind                      → "auth: unknown kind %q"
//   - Required param missing            → "auth %q: param %q is required"
//   - Templated param has missing key   → "auth %q: param %q: template references missing key …"
//   - Per-strategy syntax (basic, hmac) → see strategy file
//
// # Adding a new strategy
//
// Implement the Authenticator interface and call Register from an init() in
// your file. Optionally implement Declarer (declares.go) so chain.go can
// detect header-name collisions at Build time.
package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/galaxy-io/filament/connectors/http/template"
)

// Authenticator mutates an outbound request to satisfy the API's auth scheme.
// Implementations are safe for concurrent use.
type Authenticator interface {
	Apply(ctx context.Context, req *http.Request, scope template.Scope) error
}

// Factory builds an Authenticator from raw params. Any required fields
// missing from params should produce a clear error.
type Factory func(params map[string]any) (Authenticator, error)

var (
	mu        sync.RWMutex
	factories = map[string]Factory{}
)

// Register adds (or replaces) a Factory for kind. Safe to call from init().
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	factories[kind] = f
}

// Build instantiates the Authenticator registered for kind. Empty kind
// returns (nil, nil) — the connector treats that as "no auth" and skips
// Apply. Non-empty unknown kinds error. There is no "none" authenticator.
//
// creds populates the `config.*` template scope used to validate templated
// string params at Build time. A `{{ config.X }}` placeholder that doesn't
// resolve against creds (and has no `default`) fails fast with a clear
// error instead of surfacing as a 401 on the first live request.
//
// Pass nil creds to skip Build-time validation entirely — useful in tests
// that exercise Apply against a runtime-supplied scope. Production callers
// (the connector) always thread the connector's `config.*` map through.
func Build(kind string, params map[string]any, creds map[string]string) (Authenticator, error) {
	if kind == "" {
		return nil, nil
	}
	if err := validateParamTemplates(kind, params, creds); err != nil {
		return nil, err
	}
	mu.RLock()
	f, ok := factories[kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("auth: unknown kind %q", kind)
	}
	return f(params)
}

// buildSubAuth instantiates a child Authenticator for composite kinds
// (currently chain). The outer Build already validated every templated leaf —
// including nested ones, since validateLeaf descends through maps and lists —
// so this helper skips re-validation. Use Build, not buildSubAuth, for any
// caller that hasn't already validated against creds.
func buildSubAuth(kind string, params map[string]any) (Authenticator, error) {
	if kind == "" {
		return nil, nil
	}
	mu.RLock()
	f, ok := factories[kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("auth: unknown kind %q", kind)
	}
	return f(params)
}

// validateParamTemplates renders every templated string param against the
// supplied creds. The render result is discarded — Apply re-renders against
// the per-request scope (which has Config plus state/parent/cursor). The
// purpose here is to catch missing config keys at Configure time so manifest
// authors see a startup error, not a runtime 401.
//
// nil creds means "skip validation" — see Build's contract.
func validateParamTemplates(kind string, params map[string]any, creds map[string]string) error {
	if creds == nil || len(params) == 0 {
		return nil
	}
	scope := template.Scope{Config: creds}
	for k, v := range params {
		if err := validateLeaf(kind, k, v, scope); err != nil {
			return err
		}
	}
	return nil
}

func validateLeaf(kind, path string, v any, scope template.Scope) error {
	switch x := v.(type) {
	case string:
		if !strings.Contains(x, "{{") {
			return nil
		}
		if _, err := template.Render(x, scope); err != nil {
			return fmt.Errorf("auth %q: param %q: %w", kind, path, err)
		}
	case map[string]any:
		for k, val := range x {
			if err := validateLeaf(kind, path+"."+k, val, scope); err != nil {
				return err
			}
		}
	case []any:
		for i, val := range x {
			if err := validateLeaf(kind, fmt.Sprintf("%s[%d]", path, i), val, scope); err != nil {
				return err
			}
		}
	}
	return nil
}

// strParam returns a string-typed value from params or "" if absent.
func strParam(params map[string]any, key string) string {
	if params == nil {
		return ""
	}
	v, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// requireStrParam returns the string at key or an error naming the kind.
func requireStrParam(params map[string]any, kind, key string) (string, error) {
	s := strParam(params, key)
	if s == "" {
		return "", fmt.Errorf("auth %q: param %q is required", kind, key)
	}
	return s, nil
}
