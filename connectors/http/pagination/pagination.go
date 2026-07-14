// Package pagination implements pluggable pagination strategies for the HTTP
// connector.
//
// Caller pattern:
//
//	p, err := New(spec)
//	state := p.Initial()
//	for !state.Done {
//	    bodyOverrides, err := p.Apply(req, state)
//	    ...merge overrides into body template, send req...
//	    state, err = p.Next(resp, body, recordCount)
//	}
//
// # Adding a new strategy
//
// Implement the Paginator interface and call Register from an init() in
// your package:
//
//	func init() {
//	    pagination.Register("my_strategy", func(spec manifest.PaginationSpec) (pagination.Paginator, error) {
//	        return &myPaginator{...}, nil
//	    })
//	}
//
// The manifest's `pagination.type` field then accepts the new name. The
// JSON schema in manifest/schema.json must also be updated to allow the
// new value.
package pagination

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// State carries pagination progress between requests. Strategies populate the
// fields they care about; others stay zero-valued.
type State struct {
	Cursor  string
	NextURL string // absolute URL — overrides req.URL when non-empty
	Page    int
	Offset  int
	Done    bool
}

// Paginator advances through pages of an HTTP API.
type Paginator interface {
	// Initial returns the starting State (before the first request).
	Initial() State

	// Apply mutates req to carry the current pagination state. For body-inject
	// strategies, returns a body overrides map to be merged into the request
	// body template before encoding.
	Apply(req *http.Request, state State) (map[string]any, error)

	// Next consumes a response and returns the next State. When no more pages
	// remain, the returned State has Done == true.
	Next(resp *http.Response, body map[string]any, recordCount int) (State, error)
}

// Factory builds a Paginator from a spec. Returned by registry lookups and
// passed to Register by third-party packages.
type Factory func(spec manifest.PaginationSpec) (Paginator, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{
		"cursor":      func(s manifest.PaginationSpec) (Paginator, error) { return newCursor(s) },
		"offset":      func(s manifest.PaginationSpec) (Paginator, error) { return newOffset(s) },
		"page":        func(s manifest.PaginationSpec) (Paginator, error) { return newPage(s) },
		"link_header": func(s manifest.PaginationSpec) (Paginator, error) { return newLinkHeader(s), nil },
		"next_url":    func(s manifest.PaginationSpec) (Paginator, error) { return newNextURL(s) },
	}
)

// Register adds (or replaces) a Factory for the named pagination type. Safe
// to call from init(). Names are case-sensitive and must match what the
// manifest's `pagination.type` will carry.
//
// Replacing a built-in strategy is permitted but discouraged outside tests —
// it makes manifest behavior depend on init order.
func Register(name string, f Factory) {
	if f == nil {
		panic("pagination.Register: factory is nil for " + name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = f
}

// Registered returns the names of all currently-registered strategies, in
// no particular order. Useful for error messages and tooling.
func Registered() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// New constructs the Paginator for a given spec.
//
// An empty spec.Type means the resource declares no pagination — New returns
// (nil, nil) and the connector issues a single request. Non-empty unknown
// types are an error. There is no "none" strategy.
func New(spec manifest.PaginationSpec) (Paginator, error) {
	if spec.Type == "" {
		return nil, nil
	}
	registryMu.RLock()
	f, ok := registry[spec.Type]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("pagination: unknown type %q (registered: %v)",
			spec.Type, Registered())
	}
	return f(spec)
}

// ResumeWith returns a State pre-seeded with cursor for resumable extraction.
// Strategies that don't use Cursor ignore it.
func ResumeWith(cursor string) State { return State{Cursor: cursor} }
