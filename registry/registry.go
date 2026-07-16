// Package registry maps provider names to Source and Sink factories for per-run
// resolution. The dynamic-provider routing key (connector_name / sink_name) in a
// run request is looked up here to produce a fresh provider instance per run.
package registry

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/galaxy-io/filament"
)

// ErrUnknownProvider is returned by Resolve when no factory is registered.
var ErrUnknownProvider = errors.New("registry: unknown provider")

// Sources is the source-provider registry.
type Sources struct {
	mu        sync.RWMutex
	factories map[string]filament.SourceFactory
}

// NewSources returns an empty source registry.
func NewSources() *Sources { return &Sources{factories: map[string]filament.SourceFactory{}} }

var _ filament.SourceRegistry = (*Sources)(nil)

// Register adds a factory under name. It panics on a nil factory or a duplicate
func (r *Sources) Register(name string, f filament.SourceFactory) {
	if f == nil {
		panic("registry: nil source factory for " + name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic("registry: source already registered: " + name)
	}
	r.factories[name] = f
}

// Resolve constructs a fresh source instance for name.
func (r *Sources) Resolve(name string) (filament.Source, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: source %q", ErrUnknownProvider, name)
	}
	return f(), nil
}

// Specs returns every registered source's spec, sorted by name. Powers the
// DiscoveryService catalog.
func (r *Sources) Specs() []filament.ConnectorSpec {
	facs := r.snapshot()
	out := make([]filament.ConnectorSpec, len(facs))
	for i, f := range facs {
		out[i] = f().Spec()
	}
	return out
}

func (r *Sources) snapshot() []filament.SourceFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]filament.SourceFactory, len(names))
	for i, n := range names {
		out[i] = r.factories[n]
	}
	return out
}

// Sinks is the sink-provider registry.
type Sinks struct {
	mu        sync.RWMutex
	factories map[string]filament.SinkFactory
}

// NewSinks returns an empty sink registry.
func NewSinks() *Sinks { return &Sinks{factories: map[string]filament.SinkFactory{}} }

var _ filament.SinkRegistry = (*Sinks)(nil)

// Register adds a factory under name. Panics on nil factory or duplicate name.
func (r *Sinks) Register(name string, f filament.SinkFactory) {
	if f == nil {
		panic("registry: nil sink factory for " + name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic("registry: sink already registered: " + name)
	}
	r.factories[name] = f
}

// Resolve constructs a fresh sink instance for name.
func (r *Sinks) Resolve(name string) (filament.Sink, error) {
	r.mu.RLock()
	f, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: sink %q", ErrUnknownProvider, name)
	}
	return f(), nil
}

// Specs returns every registered sink's spec, sorted by name.
func (r *Sinks) Specs() []filament.SinkSpec {
	facs := r.snapshot()
	out := make([]filament.SinkSpec, len(facs))
	for i, f := range facs {
		out[i] = f().Spec()
	}
	return out
}

func (r *Sinks) snapshot() []filament.SinkFactory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]filament.SinkFactory, len(names))
	for i, n := range names {
		out[i] = r.factories[n]
	}
	return out
}
