package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/textproto"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func init() { Register("chain", newChain) }

// chainAuth applies multiple authenticators in declaration order. Useful for
// schemes that demand both a header and a query token, or static API key plus
// dynamic OAuth.
//
// Params:
//
//	steps: [
//	  { type: bearer, token: "..." },
//	  { type: header, name: "X-Account", value: "..." },
//	]
type chainAuth struct {
	steps []Authenticator
}

func newChain(params map[string]any) (Authenticator, error) {
	stepsRaw, ok := params["steps"]
	if !ok {
		return nil, fmt.Errorf("chain: param \"steps\" is required")
	}
	stepsList, ok := stepsRaw.([]any)
	if !ok {
		return nil, fmt.Errorf("chain: \"steps\" must be a list")
	}
	if len(stepsList) == 0 {
		return nil, fmt.Errorf("chain: \"steps\" must not be empty")
	}
	out := make([]Authenticator, 0, len(stepsList))
	for i, s := range stepsList {
		stepMap, ok := s.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("chain: steps[%d] must be a map", i)
		}
		kind, _ := stepMap["type"].(string)
		if kind == "" {
			return nil, fmt.Errorf("chain: steps[%d].type is required", i)
		}
		// Strip "type" before forwarding so the inner factory only sees its params.
		sub := make(map[string]any, len(stepMap)-1)
		for k, v := range stepMap {
			if k == "type" {
				continue
			}
			sub[k] = v
		}
		// buildSubAuth: outer Build already validated nested templates against
		// creds; re-validating here would fail because we no longer have the
		// creds map at this layer.
		a, err := buildSubAuth(kind, sub)
		if err != nil {
			return nil, fmt.Errorf("chain: steps[%d]: %w", i, err)
		}
		out = append(out, a)
	}
	if err := validateChainCollisions(out); err != nil {
		return nil, err
	}
	return &chainAuth{steps: out}, nil
}

// validateChainCollisions errors when two steps both write the same header or
// query param. Header names are compared case-insensitively (RFC 9110). Auths
// that don't implement Declarer are treated as opaque — no collision is
// reported against them, but their declared peers still collide as expected.
//
// This shifts an entire class of "second auth silently overwrote first" bugs
// from runtime mystery to manifest-load error with explicit step indices.
func validateChainCollisions(steps []Authenticator) error {
	type origin struct {
		stepIdx int
		write   Write
	}
	headers := map[string]origin{}
	params := map[string]origin{}
	for i, s := range steps {
		for _, w := range declaredWrites(s) {
			switch w.Kind {
			case WriteHeader:
				key := textproto.CanonicalMIMEHeaderKey(w.Name)
				if prev, ok := headers[key]; ok {
					return fmt.Errorf(
						"chain: steps[%d] and steps[%d] both write header %q (collision)",
						prev.stepIdx, i, w.Name)
				}
				headers[key] = origin{stepIdx: i, write: w}
			case WriteQueryParam:
				if prev, ok := params[w.Name]; ok {
					return fmt.Errorf(
						"chain: steps[%d] and steps[%d] both write query param %q (collision)",
						prev.stepIdx, i, w.Name)
				}
				params[w.Name] = origin{stepIdx: i, write: w}
			}
		}
	}
	return nil
}

// DeclaredWrites flattens declarations from every step so nested chains
// participate in collision detection at the outer Build.
func (a *chainAuth) DeclaredWrites() []Write {
	var out []Write
	for _, s := range a.steps {
		out = append(out, declaredWrites(s)...)
	}
	return out
}

func (a *chainAuth) Apply(ctx context.Context, req *http.Request, scope template.Scope) error {
	for i, s := range a.steps {
		if err := s.Apply(ctx, req, scope); err != nil {
			return fmt.Errorf("chain step %d: %w", i, err)
		}
	}
	return nil
}
