// Package module defines ingestion's Module contract and its dependency wiring.
// Modules are run by a host.Host; this package owns only what is ingestion-
// specific: the Deps bag and the Mount step that injects it.
package module

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
)

// Module is an ingestion behavior — scheduler, orchestrator, engine, tracker,
// stream. It is a host.Runnable (name + subscriptions) plus a Mount step that
// captures its providers from Deps. Modules never reference each other.
type Module interface {
	host.Runnable
	// Mount wires the module to its providers. Called once, before the host
	// runs it. Cheap setup only — no blocking, no bus reads.
	Mount(ctx context.Context, d Deps) error
}

// Deps is everything a module may need, injected at Mount. A module takes only
// the fields it uses; the rest stay nil. Infra providers are bound once at boot;
// the registries resolve workload providers (Source, Sink) per run.
type Deps struct {
	Bus       eventbus.Bus
	DataStore filament.DataStore
	Secrets   filament.Secrets
	Runtime   filament.Runtime
	Scheduler filament.Scheduler
	Dispatch  filament.Dispatcher

	Sources filament.SourceRegistry
	Sinks   filament.SinkRegistry

	Log     filament.Logger
	Metrics filament.Metrics
	Tracer  filament.Tracer
}

// MountAll injects d into each module and returns them as host.Runnables ready
// to hand to a host.Host.
func MountAll(ctx context.Context, d Deps, mods ...Module) ([]host.Runnable, error) {
	rs := make([]host.Runnable, 0, len(mods))
	for _, m := range mods {
		if err := m.Mount(ctx, d); err != nil {
			return nil, fmt.Errorf("mount %q: %w", m.Name(), err)
		}
		rs = append(rs, m)
	}
	return rs, nil
}
