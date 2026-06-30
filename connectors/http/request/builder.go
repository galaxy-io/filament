// Package request builds outgoing HTTP requests from a v2 manifest.Resource.
//
// The Builder renders the resource against a template.Scope, encodes the body
// (JSON, form, multipart, raw), merges connection-level + per-resource
// headers, and applies the configured Authenticator.
//
// Two-phase build is supported via RenderResource + BuildRendered: callers
// that need to mutate the rendered body template before encoding (pagination
// or incremental override merges) render once, mutate, then build.
//
// # Worked example
//
//	scope := template.Scope{Config: creds, Cursor: state.Cursor}
//	rendered, err := request.RenderResource(res, scope)   // template substitutions
//	rendered.Body.Template = request.MergeOverrides(rendered.Body.Template, pagOverrides, nil)
//	req, err := builder.BuildRendered(ctx, rendered, scope) // encode + auth
//
// Single-phase callers use Build, which wraps RenderResource + BuildRendered
// for the case where no override merging is needed.
//
// # Failure modes
//
//   - Template render failure       → "render path/query/headers/body: %w"
//     (wrapped errs.ErrTemplateMissingKey or errs.ErrTemplateSyntax)
//   - Unknown body encoding         → returned by EncoderFor
//   - Encoder failure (form non-scalar value, multipart open, ...) → wrapped
//   - http.NewRequestWithContext failure (bad URL/method) → wrapped
//   - Auth.Apply failure            → "auth: %w"
package request

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/auth"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Builder constructs HTTP requests from a manifest.Resource. Connection-level
// concerns (base URL, default headers, auth) are bound at construction time;
// per-request concerns (resource, scope) are passed to Build.
type Builder struct {
	BaseURL           string
	ConnectionHeaders map[string]string
	Auth              auth.Authenticator
}

// Build renders res against scope and constructs a request. Convenience
// wrapper for callers that don't need post-render mutation.
//
// Two-phase callers (e.g. paginate.fetchPage that needs to merge body
// overrides into the rendered template) should call RenderResource then
// BuildRendered.
func (b *Builder) Build(ctx context.Context, res manifest.Resource, scope template.Scope) (*http.Request, error) {
	rendered, err := RenderResource(res, scope)
	if err != nil {
		return nil, fmt.Errorf("render resource %q: %w", res.Name, err)
	}
	return b.BuildRendered(ctx, rendered, scope)
}

// RenderResource returns a copy of r with every templatable field rendered
// against scope. The input is not mutated. Lives in the request package
// (rather than template) because manifest.Resource is request-shaped and
// having template depend on manifest creates an import cycle now that
// manifest validates templates at load time.
func RenderResource(r manifest.Resource, scope template.Scope) (manifest.Resource, error) {
	out := r
	rendered, err := template.Render(r.Path, scope)
	if err != nil {
		return r, fmt.Errorf("render path: %w", err)
	}
	out.Path = rendered

	if r.Query != nil {
		q, err := template.RenderStringMap(r.Query, scope)
		if err != nil {
			return r, fmt.Errorf("render query: %w", err)
		}
		out.Query = q
	}
	if r.Headers != nil {
		h, err := template.RenderStringMap(r.Headers, scope)
		if err != nil {
			return r, fmt.Errorf("render headers: %w", err)
		}
		out.Headers = h
	}
	if r.Body.Template != nil {
		body, err := template.RenderAny(r.Body.Template, scope)
		if err != nil {
			return r, fmt.Errorf("render body: %w", err)
		}
		out.Body.Template = body
	}
	return out, nil
}

// BuildRendered constructs a request from an already-rendered Resource. Use
// this when the caller needs to post-render the body template (e.g. to merge
// pagination/incremental overrides) before encoding. scope is still required
// because connection-level headers + auth render per-request.
func (b *Builder) BuildRendered(ctx context.Context, rendered manifest.Resource, scope template.Scope) (*http.Request, error) {
	method := strings.ToUpper(rendered.Method)
	if method == "" {
		method = http.MethodGet
	}

	enc, err := EncoderFor(rendered.Body.Encoding, rendered.Body.Template)
	if err != nil {
		return nil, err
	}
	body, contentType, err := enc.Encode(rendered.Body.Template)
	if err != nil {
		return nil, err
	}

	url := strings.TrimRight(b.BaseURL, "/") + "/" + strings.TrimLeft(rendered.Path, "/")

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}

	// Connection-default headers first; resource overrides win. Both go
	// through the template engine so connection-level templates (e.g.
	// `{{ state.* }}`) resolve per request.
	for k, v := range b.ConnectionHeaders {
		rv, err := template.Render(v, scope)
		if err != nil {
			return nil, fmt.Errorf("render connection header %s: %w", k, err)
		}
		req.Header.Set(k, rv)
	}
	for k, v := range rendered.Headers {
		req.Header.Set(k, v)
	}
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	if len(rendered.Query) > 0 {
		q := req.URL.Query()
		for k, v := range rendered.Query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	if b.Auth != nil {
		if err := b.Auth.Apply(ctx, req, scope); err != nil {
			return nil, fmt.Errorf("auth: %w", err)
		}
	}
	return req, nil
}
