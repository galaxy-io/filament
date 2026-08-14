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
	factories map[string]sourceRegistration
}

type sourceRegistration struct {
	factory  filament.SourceFactory
	maturity filament.ConnectorMaturity
}

// NewSources returns an empty source registry.
func NewSources() *Sources { return &Sources{factories: map[string]sourceRegistration{}} }

var _ filament.SourceRegistry = (*Sources)(nil)

// Register adds a factory under name. It panics on a nil factory or a duplicate
func (r *Sources) Register(name string, f filament.SourceFactory) {
	r.RegisterWithMaturity(name, filament.MaturityAlpha, f)
}

// RegisterWithMaturity adds a source factory and its catalog maturity under
// name. It panics on an invalid maturity, a nil factory, or a duplicate name.
func (r *Sources) RegisterWithMaturity(name string, maturity filament.ConnectorMaturity, f filament.SourceFactory) {
	switch maturity {
	case filament.MaturityAlpha, filament.MaturityBeta, filament.MaturityStable:
	default:
		panic("registry: invalid source maturity for " + name + ": " + string(maturity))
	}
	if f == nil {
		panic("registry: nil source factory for " + name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic("registry: source already registered: " + name)
	}
	r.factories[name] = sourceRegistration{factory: f, maturity: maturity}
}

// Resolve constructs a fresh source instance for name.
func (r *Sources) Resolve(name string) (filament.Source, error) {
	r.mu.RLock()
	registration, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: source %q", ErrUnknownProvider, name)
	}
	return registration.factory(), nil
}

// Spec returns one registered source's catalog spec with its maturity marker.
func (r *Sources) Spec(name string) (filament.ConnectorSpec, error) {
	r.mu.RLock()
	registration, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return filament.ConnectorSpec{}, fmt.Errorf("%w: source %q", ErrUnknownProvider, name)
	}
	spec := registration.factory().Spec()
	spec.Maturity = registration.maturity
	return spec, nil
}

// Specs returns every registered source's spec, sorted by name. Powers the
// DiscoveryService catalog.
func (r *Sources) Specs() []filament.ConnectorSpec {
	registrations := r.snapshot()
	out := make([]filament.ConnectorSpec, len(registrations))
	for i, registration := range registrations {
		out[i] = registration.factory().Spec()
		out[i].Maturity = registration.maturity
	}
	return out
}

func (r *Sources) snapshot() []sourceRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]sourceRegistration, len(names))
	for i, n := range names {
		out[i] = r.factories[n]
	}
	return out
}

// Sinks is the sink-provider registry.
type Sinks struct {
	mu        sync.RWMutex
	factories map[string]sinkRegistration
}

type sinkRegistration struct {
	factory  filament.SinkFactory
	maturity filament.ConnectorMaturity
}

// NewSinks returns an empty sink registry.
func NewSinks() *Sinks { return &Sinks{factories: map[string]sinkRegistration{}} }

var _ filament.SinkRegistry = (*Sinks)(nil)

// Register adds a factory under name. Panics on nil factory or duplicate name.
func (r *Sinks) Register(name string, f filament.SinkFactory) {
	r.RegisterWithMaturity(name, filament.MaturityAlpha, f)
}

// RegisterWithMaturity adds a sink factory and its catalog maturity under name.
// It panics on an invalid maturity, a nil factory, or a duplicate name.
func (r *Sinks) RegisterWithMaturity(name string, maturity filament.ConnectorMaturity, f filament.SinkFactory) {
	switch maturity {
	case filament.MaturityAlpha, filament.MaturityBeta, filament.MaturityStable:
	default:
		panic("registry: invalid sink maturity for " + name + ": " + string(maturity))
	}
	if f == nil {
		panic("registry: nil sink factory for " + name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.factories[name]; dup {
		panic("registry: sink already registered: " + name)
	}
	r.factories[name] = sinkRegistration{factory: f, maturity: maturity}
}

// Resolve constructs a fresh sink instance for name.
func (r *Sinks) Resolve(name string) (filament.Sink, error) {
	r.mu.RLock()
	registration, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: sink %q", ErrUnknownProvider, name)
	}
	return registration.factory(), nil
}

// Spec returns one registered sink's catalog spec with its maturity marker.
func (r *Sinks) Spec(name string) (filament.SinkSpec, error) {
	r.mu.RLock()
	registration, ok := r.factories[name]
	r.mu.RUnlock()
	if !ok {
		return filament.SinkSpec{}, fmt.Errorf("%w: sink %q", ErrUnknownProvider, name)
	}
	spec := registration.factory().Spec()
	spec.Maturity = registration.maturity
	return spec, nil
}

// Specs returns every registered sink's spec, sorted by name.
func (r *Sinks) Specs() []filament.SinkSpec {
	registrations := r.snapshot()
	out := make([]filament.SinkSpec, len(registrations))
	for i, registration := range registrations {
		out[i] = registration.factory().Spec()
		out[i].Maturity = registration.maturity
	}
	return out
}

func (r *Sinks) snapshot() []sinkRegistration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.factories))
	for n := range r.factories {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]sinkRegistration, len(names))
	for i, n := range names {
		out[i] = r.factories[n]
	}
	return out
}
