package pagination

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// nextURLPaginator reads a full URL from a configured response body path and
// uses it as the next request's URL. Stops when the path is missing/empty.
type nextURLPaginator struct {
	path string
}

func newNextURL(spec manifest.PaginationSpec) (*nextURLPaginator, error) {
	if spec.NextURLPath == "" {
		return nil, fmt.Errorf("next_url pagination: next_url_path is required")
	}
	return &nextURLPaginator{path: spec.NextURLPath}, nil
}

func (p *nextURLPaginator) Initial() State { return State{} }

func (p *nextURLPaginator) Apply(req *http.Request, s State) (map[string]any, error) {
	if s.NextURL == "" {
		return nil, nil
	}
	resolved, err := resolveNextURL(req.URL, s.NextURL)
	if err != nil {
		return nil, fmt.Errorf("next_url: %w", err)
	}
	req.URL = resolved
	req.Host = resolved.Host
	return nil, nil
}

func (p *nextURLPaginator) Next(_ *http.Response, body map[string]any, _ int) (State, error) {
	next, _, err := paths.AsString(body, p.path)
	switch {
	case errors.Is(err, errs.ErrPathMissing), errors.Is(err, errs.ErrPathNull):
		return State{Done: true}, nil
	case err != nil:
		return State{}, fmt.Errorf("next_url: next_url_path %q: %w", p.path, err)
	}
	if next == "" {
		return State{Done: true}, nil
	}
	return State{NextURL: next}, nil
}

// resolveNextURL parses raw against base. Absolute URLs pass through;
// relative URLs (`/page/2`, `?page=2`) are resolved against the current
// request URL so the host is preserved.
func resolveNextURL(base *url.URL, raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", raw, err)
	}
	if parsed.IsAbs() {
		return parsed, nil
	}
	if base == nil {
		return nil, fmt.Errorf("relative next_url %q with no base request URL", raw)
	}
	return base.ResolveReference(parsed), nil
}
