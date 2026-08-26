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

type registration[P any] struct {
	factory  func() P
	maturity filament.ConnectorMaturity
}

type providerRegistry[P, S any] struct {
	kind        string
	mu          sync.RWMutex
	factories   map[string]registration[P]
	specOf      func(P) S
	setMaturity func(*S, filament.ConnectorMaturity)
}

func newProviderRegistry[P, S any](
	kind string,
	specOf func(P) S,
	setMaturity func(*S, filament.ConnectorMaturity),
) *providerRegistry[P, S] {
	return &providerRegistry[P, S]{
		kind:        kind,
		factories:   make(map[string]registration[P]),
		specOf:      specOf,
		setMaturity: setMaturity,
	}
}

func (r *providerRegistry[P, S]) register(name string, maturity filament.ConnectorMaturity, factory func() P) {
	switch maturity {
	case filament.MaturityAlpha, filament.MaturityBeta, filament.MaturityStable:
	default:
		panic("registry: invalid " + r.kind + " maturity for " + name + ": " + string(maturity))
	}
	if factory == nil {
		panic("registry: nil " + r.kind + " factory for " + name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic("registry: " + r.kind + " already registered: " + name)
	}
	r.factories[name] = registration[P]{factory: factory, maturity: maturity}
}

func (r *providerRegistry[P, S]) resolve(name string) (P, error) {
	r.mu.RLock()
	registration, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		var zero P
		return zero, fmt.Errorf("%w: %s %q", ErrUnknownProvider, r.kind, name)
	}
	return registration.factory(), nil
}

func (r *providerRegistry[P, S]) spec(name string) (S, error) {
	r.mu.RLock()
	registration, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		var zero S
		return zero, fmt.Errorf("%w: %s %q", ErrUnknownProvider, r.kind, name)
	}
	spec := r.specOf(registration.factory())
	r.setMaturity(&spec, registration.maturity)
	return spec, nil
}

func (r *providerRegistry[P, S]) specs() []S {
	registrations := r.snapshot()
	out := make([]S, len(registrations))
	for i, registration := range registrations {
		out[i] = r.specOf(registration.factory())
		r.setMaturity(&out[i], registration.maturity)
	}
	return out
}

func (r *providerRegistry[P, S]) snapshot() []registration[P] {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]registration[P], len(names))
	for i, n := range names {
		out[i] = r.factories[n]
	}
	return out
}

// Sources is the source-provider registry.
type Sources struct {
	providers *providerRegistry[filament.Source, filament.ConnectorSpec]
}

// NewSources returns an empty source registry.
func NewSources() *Sources {
	return &Sources{providers: newProviderRegistry(
		"source",
		func(source filament.Source) filament.ConnectorSpec { return source.Spec() },
		func(spec *filament.ConnectorSpec, maturity filament.ConnectorMaturity) { spec.Maturity = maturity },
	)}
}

var _ filament.SourceRegistry = (*Sources)(nil)

// Register adds a factory under name. It panics on a nil factory or a duplicate
func (r *Sources) Register(name string, factory filament.SourceFactory) {
	r.RegisterWithMaturity(name, filament.MaturityAlpha, factory)
}

// RegisterWithMaturity adds a source factory and its catalog maturity under
// name. It panics on an invalid maturity, a nil factory, or a duplicate name.
func (r *Sources) RegisterWithMaturity(name string, maturity filament.ConnectorMaturity, factory filament.SourceFactory) {
	r.providers.register(name, maturity, factory)
}

// Resolve constructs a fresh source instance for name.
func (r *Sources) Resolve(name string) (filament.Source, error) {
	return r.providers.resolve(name)
}

// Spec returns one registered source's catalog spec with its maturity marker.
func (r *Sources) Spec(name string) (filament.ConnectorSpec, error) {
	return r.providers.spec(name)
}

// Specs returns every registered source's spec, sorted by name. Powers the
// DiscoveryService catalog.
func (r *Sources) Specs() []filament.ConnectorSpec {
	return r.providers.specs()
}

// Sinks is the sink-provider registry.
type Sinks struct {
	providers *providerRegistry[filament.Sink, filament.SinkSpec]
}

// NewSinks returns an empty sink registry.
func NewSinks() *Sinks {
	return &Sinks{providers: newProviderRegistry(
		"sink",
		func(sink filament.Sink) filament.SinkSpec { return sink.Spec() },
		func(spec *filament.SinkSpec, maturity filament.ConnectorMaturity) { spec.Maturity = maturity },
	)}
}

var _ filament.SinkRegistry = (*Sinks)(nil)

// Register adds a factory under name. Panics on nil factory or duplicate name.
func (r *Sinks) Register(name string, factory filament.SinkFactory) {
	r.RegisterWithMaturity(name, filament.MaturityAlpha, factory)
}

// RegisterWithMaturity adds a sink factory and its catalog maturity under name.
// It panics on an invalid maturity, a nil factory, or a duplicate name.
func (r *Sinks) RegisterWithMaturity(name string, maturity filament.ConnectorMaturity, factory filament.SinkFactory) {
	r.providers.register(name, maturity, factory)
}

// Resolve constructs a fresh sink instance for name.
func (r *Sinks) Resolve(name string) (filament.Sink, error) {
	return r.providers.resolve(name)
}

// Spec returns one registered sink's catalog spec with its maturity marker.
func (r *Sinks) Spec(name string) (filament.SinkSpec, error) {
	return r.providers.spec(name)
}

// Specs returns every registered sink's spec, sorted by name.
func (r *Sinks) Specs() []filament.SinkSpec {
	return r.providers.specs()
}
