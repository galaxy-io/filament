// Package engine is the in-process dispatch backend: it reacts to a
// run.requested fact and executes the run inside the calling process.
//
// It owns no run logic. The sequence lives in runner, which the worker binary
// also calls, so DISPATCH_MODE selects where a run executes and never how it
// behaves. This module is the bus adapter: subscribe, load, gate, delegate.
package engine

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/runner"
)

// Module is the extraction engine. One run.requested fact drives one extraction.
type Module struct {
	deps runner.Deps
	ds   filament.DataStore
}

// New returns an unmounted engine. Providers are injected by Mount.
func New() *Module { return &Module{} }

var _ module.Module = (*Module)(nil)

// Name identifies this module.
func (m *Module) Name() string { return "dispatch" }

// Subscriptions declares a durable consumer over run.requested across every tenant/run.
// The fact is only a trigger; the request payload is loaded from the DataStore.
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{
		{Pattern: events.SubjectPattern(events.RunRequested), Durable: m.Name(), Handler: events.Handler(events.RunRequested, m.onRunRequested)},
	}
}

// Mount captures the providers this module uses. Cheap, no I/O.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.ds = d.DataStore
	m.deps = runner.Deps{
		Bus:       d.Bus,
		DataStore: d.DataStore,
		Secrets:   d.Secrets,
		Sources:   d.Sources,
		Sinks:     d.Sinks,
		Log:       d.Log,
		Tracer:    d.Tracer,
	}
	return nil
}

// onRunRequested loads the requested run and executes it. A failure to load is
// transient (the request state may not be persisted yet) and is naked for
// redelivery; a run that fails for any other reason is reported as a fact and
// acked — blindly re-running a whole extraction would duplicate work.
func (m *Module) onRunRequested(ctx context.Context, ev events.Event[events.RunRequestedEvent]) error {
	state, err := m.ds.LoadRun(ctx, ev.Run)
	if err != nil {
		return fmt.Errorf("engine: load run %q: %w", ev.Run, err)
	}
	if !runner.ShouldRun(state) {
		return nil // already running or finished — ack and ignore
	}
	return runner.RunOne(ctx, m.deps, runner.SpecFromState(state))
}
