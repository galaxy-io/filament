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
	aliases     map[string]string
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
		aliases:     make(map[string]string),
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
	if _, dup := r.aliases[name]; dup {
		panic("registry: " + r.kind + " already registered: " + name)
	}
	r.factories[name] = registration[P]{factory: factory, maturity: maturity}
}

func (r *providerRegistry[P, S]) resolve(name string) (P, error) {
	r.mu.RLock()
	if target, ok := r.aliases[name]; ok {
		name = target
	}
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
	if target, ok := r.aliases[name]; ok {
		name = target
	}
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

// WithOverrides returns an independent registry where overrides take precedence
// for matching concrete keys and aliases. Unmentioned registrations are retained.
// Neither input is mutated, and source factories still produce fresh instances.
func (r *Sources) WithOverrides(overrides *Sources) (*Sources, error) {
	merged := NewSources()
	for _, sources := range []*Sources{r, overrides} {
		if sources == nil {
			return nil, errors.New("registry: nil source registry")
		}
		p := sources.providers
		p.mu.RLock()
		for name, registration := range p.factories {
			delete(merged.providers.aliases, name)
			merged.providers.factories[name] = registration
		}
		for name, target := range p.aliases {
			delete(merged.providers.factories, name)
			merged.providers.aliases[name] = target
		}
		p.mu.RUnlock()
	}
	for name, target := range merged.providers.aliases {
		if _, exists := merged.providers.factories[target]; !exists {
			return nil, fmt.Errorf("registry: override leaves source alias %q without concrete target %q", name, target)
		}
	}
	return merged, nil
}

// Register adds a factory under name. It panics on a nil factory or a duplicate
func (r *Sources) Register(name string, factory filament.SourceFactory) {
	r.RegisterWithMaturity(name, filament.MaturityAlpha, factory)
}

// RegisterWithMaturity adds a source factory and its catalog maturity under
// name. It panics on an invalid maturity, a nil factory, or a duplicate name.
func (r *Sources) RegisterWithMaturity(name string, maturity filament.ConnectorMaturity, factory filament.SourceFactory) {
	r.providers.register(name, maturity, factory)
}

// RegisterAlias adds a lookup name for an existing concrete source registration.
// Aliases appear in Specs under their lookup names. Resolution preserves the
// target's concrete identity so new connections can pin the selected version.
// It panics on collisions or targets that are missing or themselves aliases.
func (r *Sources) RegisterAlias(name, target string) {
	p := r.providers
	p.mu.Lock()
	defer p.mu.Unlock()
	if name == "" {
		panic("registry: empty source alias")
	}
	if _, exists := p.factories[name]; exists {
		panic("registry: source already registered: " + name)
	}
	if _, exists := p.aliases[name]; exists {
		panic("registry: source already registered: " + name)
	}
	if _, exists := p.factories[target]; !exists {
		panic("registry: source alias target is not registered: " + target)
	}
	p.aliases[name] = target
}

// Resolve constructs a fresh source instance for name.
func (r *Sources) Resolve(name string) (filament.Source, error) {
	return r.providers.resolve(name)
}

// Spec returns one registered source's catalog spec with its maturity marker.
func (r *Sources) Spec(name string) (filament.ConnectorSpec, error) {
	return r.providers.spec(name)
}

// Specs returns concrete sources and aliases, sorted by name. Alias specs keep
// their lookup names and expose AliasTarget so catalog consumers can distinguish
// aliases from concrete registrations and validate saved unversioned refs.
func (r *Sources) Specs() []filament.ConnectorSpec {
	p := r.providers
	p.mu.RLock()
	registrations := make(map[string]registration[filament.Source], len(p.factories)+len(p.aliases))
	aliases := make(map[string]string, len(p.aliases))
	for name, registration := range p.factories {
		registrations[name] = registration
	}
	for name, target := range p.aliases {
		registrations[name] = p.factories[target]
		aliases[name] = target
	}
	p.mu.RUnlock()

	names := make([]string, 0, len(registrations))
	for name := range registrations {
		names = append(names, name)
	}
	sort.Strings(names)
	specs := make([]filament.ConnectorSpec, 0, len(names))
	for _, name := range names {
		registration := registrations[name]
		spec := registration.factory().Spec()
		spec.Name = name
		spec.AliasTarget = aliases[name]
		spec.Maturity = registration.maturity
		specs = append(specs, spec)
	}
	return specs
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
