// Package engine is the in-process dispatch backend: it reacts to a
// run.requested fact and executes the run inside the calling process.
//
// It owns no run logic. The sequence lives in runner, which the worker binary
// also calls, so DISPATCH_MODE selects where a run executes and never how it
// behaves. This module is the bus adapter: subscribe, load, gate, delegate.
package engine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/heartbeat"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/runner"
)

// Module is the extraction engine. One run.requested fact drives one extraction.
type Module struct {
	deps          runner.Deps
	ds            filament.DataStore
	mx            filament.Metrics
	log           filament.Logger
	heartbeatFreq time.Duration
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
	m.mx = d.Metrics
	if d.Log != nil {
		m.log = d.Log.With(filament.Field{Key: "component", Value: "dispatch"})
	}
	m.deps = runner.Deps{
		Bus:           d.Bus,
		DataStore:     d.DataStore,
		Secrets:       d.Secrets,
		Sources:       d.Sources,
		Sinks:         d.Sinks,
		Log:           d.Log,
		Tracer:        d.Tracer,
		Functions:     d.Functions,
		FunctionBatch: d.FunctionBatch,
	}
	return nil
}

// onRunRequested loads the requested run and executes it. Missing runs are
// stale events and are acknowledged; other load failures retry on redelivery.
// Execution failures are reported as facts to avoid duplicating extraction.
func (m *Module) onRunRequested(ctx context.Context, ev events.Event[events.RunRequestedEvent]) error {
	if m.log != nil {
		m.log.Trace("run request received",
			filament.Field{Key: "event.name", Value: "dispatch.run_request.received"},
			filament.Field{Key: "tenant_id", Value: string(ev.Tenant)},
			filament.Field{Key: "run_id", Value: string(ev.Run)})
	}
	state, err := m.ds.LoadRun(ctx, ev.Tenant, ev.Run)
	// Runs are persisted before publication, but may be deleted before delivery.
	if errors.Is(err, filament.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("engine: load run %q: %w", ev.Run, err)
	}
	if state.Request.Options.Execution.Normalize() == filament.ExecutionContinuous {
		return nil
	}
	if !runner.ShouldRun(state) {
		if m.log != nil {
			m.log.Debug("run dispatch skipped",
				filament.Field{Key: "event.name", Value: "dispatch.run.skipped"},
				filament.Field{Key: "run_id", Value: string(ev.Run)},
				filament.Field{Key: "status", Value: int(state.Status)},
				filament.Field{Key: "reason", Value: "not_runnable"})
		}
		return nil // already running or finished — ack and ignore
	}
	spec := runner.SpecFromState(state)
	spec.ExecutionID = ev.At.UTC().Format(time.RFC3339Nano)
	freq := m.heartbeatFreq
	if freq <= 0 {
		freq = heartbeat.Interval()
	}
	var hbLog filament.Logger
	if m.deps.Log != nil {
		hbLog = m.deps.Log.With(
			filament.Field{Key: "component", Value: "heartbeat"},
			filament.Field{Key: "run_id", Value: string(state.Run)},
		)
	}
	hb := heartbeat.New(m.deps.Bus, m.mx, hbLog, state.Tenant, state.Run, state.Request.PipelineID)
	stopHB := hb.Start(ctx, freq)
	defer stopHB()
	return runner.RunOne(ctx, m.deps, spec)
}
