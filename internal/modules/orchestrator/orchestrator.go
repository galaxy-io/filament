// Package orchestrator owns run intake and lifecycle. It turns a RunSubmission into
// a persisted, addressable run and dispatches it onto the bus as a run.requested
// fact for the engine to pick up. It is the real counterpart to the "orchestrator
// stand-in" the tests and demo previously inlined: assign an id, persist the
// request, publish the trigger
package orchestrator

import (
	"context"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/internal/runs"
	"github.com/galaxy-io/filament/module"
)

// Module is the run intake. It issues commands (Submit) rather than reacting to
// facts, so it declares no subscriptions; the composition root or an API layer
// calls Submit.
type Module struct {
	bus eventbus.Bus
	ds  filament.DataStore
	log filament.Logger
	mx  filament.Metrics
}

// New returns an unmounted orchestrator. Providers are injected by Mount.
func New() *Module { return &Module{} }

var _ module.Module = (*Module)(nil)

// Name identifies this module.
func (m *Module) Name() string { return "orchestrator" }

// Subscriptions returns none; the orchestrator is the command side of the plane.
func (m *Module) Subscriptions() []host.Subscription { return nil }

// Mount captures the providers this module uses. Cheap, no I/O.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.bus = d.Bus
	m.ds = d.DataStore
	if d.Log != nil {
		m.log = d.Log.With(filament.Field{Key: "component", Value: "orchestrator"})
	}
	m.mx = d.Metrics
	return nil
}

// Submit admits a run and dispatches its trigger, returning the assigned run
// id. It is idempotent on the request's IdempotencyKey: re-submitting the same
// (tenant, key) returns the existing run without persisting or publishing again,
// so a retried caller never starts a duplicate extraction.
func (m *Module) Submit(ctx context.Context, submission filament.RunSubmission) (filament.RunID, error) {
	req := submission.Request
	id, err := runs.Submit(ctx, m.bus, m.ds, submission)
	if err != nil {
		return "", err
	}
	if m.mx != nil {
		m.mx.Counter("filament_runs_submitted_total", filament.Label{Key: "tenant", Value: string(req.Tenant)}).Inc()
	}
	if m.log != nil {
		m.log.Debug("run submitted",
			filament.Field{Key: "event.name", Value: "orchestrator.run.submitted"},
			filament.Field{Key: "run_id", Value: string(id)},
			filament.Field{Key: "tenant_id", Value: string(req.Tenant)})
	}
	return id, nil
}
