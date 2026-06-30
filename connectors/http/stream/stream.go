// Package stream implements long-lived streaming response readers for the HTTP
// connector. Three formats are supported:
//
//	ndjson         — newline-delimited JSON objects, one per record
//	sse            — Server-Sent Events; each `data:` block becomes one record
//	chunked_array  — JSON document `[ {...}, {...}, ... ]` decoded incrementally
//
// All readers honor ctx cancellation between records.
//
// # Adding a new format
//
// Implement the Reader interface and call Register from an init() in your
// package. The manifest's `stream.type` field then accepts the new name.
//
//	func init() {
//	    stream.Register("csv", func(spec *manifest.StreamSpec, opts stream.Options) (stream.Reader, error) {
//	        return &csvReader{logger: opts.Logger}, nil
//	    })
//	}
//
// The JSON schema in manifest/schema.json must also be updated to allow the
// new value.
package stream

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// EmitFunc is invoked for each parsed record. Returning an error aborts the
// stream and surfaces from Reader.Read.
type EmitFunc func(record map[string]any) error

// Reader consumes a streaming response body and emits decoded records.
// Implementations must respect ctx cancellation.
type Reader interface {
	Read(ctx context.Context, body io.Reader, emit EmitFunc) error
}

// Options carries optional cross-cutting concerns. A zero Options uses safe
// defaults: slog.Default() logger, NDJSON max line 10MB, strict malformed-line
// policy (maxErrors=0 → fail on first bad line).
type Options struct {
	// Logger receives WARN entries for skipped/malformed lines. nil falls
	// back to slog.Default(); a logger is always active so skip events
	// always emit.
	Logger *slog.Logger

	// NDJSONMaxLine overrides the per-line size cap (default 10MB).
	NDJSONMaxLine int

	// NDJSONMaxErrors caps malformed lines:
	//   0  (default) — strict: first bad line errors.
	//   N > 0        — allow N bad lines, then error.
	//  -1            — unbounded (log but never error).
	NDJSONMaxErrors int

	// SSEMaxErrors mirrors NDJSONMaxErrors for SSE blocks.
	SSEMaxErrors int
}

// New selects a Reader by stream spec. nil spec defaults to NDJSON.
func New(spec *manifest.StreamSpec) (Reader, error) {
	return NewWithOptions(spec, Options{})
}

// Factory builds a Reader from a spec and Options. Returned by registry
// lookups and passed to Register by third-party packages.
type Factory func(spec *manifest.StreamSpec, opts Options) (Reader, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{
		"": func(_ *manifest.StreamSpec, opts Options) (Reader, error) {
			return &ndjsonReader{logger: opts.Logger, maxLine: opts.NDJSONMaxLine, maxErrors: opts.NDJSONMaxErrors}, nil
		},
		"ndjson": func(_ *manifest.StreamSpec, opts Options) (Reader, error) {
			return &ndjsonReader{logger: opts.Logger, maxLine: opts.NDJSONMaxLine, maxErrors: opts.NDJSONMaxErrors}, nil
		},
		"sse": func(spec *manifest.StreamSpec, opts Options) (Reader, error) {
			return &sseReader{eventFilter: spec.EventFilter, logger: opts.Logger, maxErrors: opts.SSEMaxErrors}, nil
		},
		"chunked_array": func(spec *manifest.StreamSpec, opts Options) (Reader, error) {
			return &chunkedArrayReader{arrayPath: spec.ArrayPath, maxElementBytes: spec.MaxElementBytes, logger: opts.Logger}, nil
		},
	}
)

// Register adds (or replaces) a Factory for the named stream type. Safe to
// call from init(). Replacing a built-in is permitted but discouraged
// outside tests — it makes manifest behavior depend on init order.
func Register(name string, f Factory) {
	if f == nil {
		panic("stream.Register: factory is nil for " + name)
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = f
}

// Registered returns names of all currently-registered formats. Useful for
// error messages and tooling.
func Registered() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		if k == "" {
			continue
		}
		out = append(out, k)
	}
	return out
}

// NewWithOptions is New with cross-cutting options.
func NewWithOptions(spec *manifest.StreamSpec, opts Options) (Reader, error) {
	var typeName string
	if spec != nil {
		typeName = spec.Type
	}
	registryMu.RLock()
	f, ok := registry[typeName]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("stream: unknown type %q (registered: %v)",
			typeName, Registered())
	}
	return f(spec, opts)
}
