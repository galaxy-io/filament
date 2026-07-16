// Package orchestrator owns run intake and lifecycle. It turns a RunRequest into
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
	m.log = d.Log
	return nil
}

// Submit registers a run and dispatches its trigger, returning the assigned run
// id. It is idempotent on the request's IdempotencyKey: re-submitting the same
// (tenant, key) returns the existing run without persisting or publishing again,
// so a retried caller never starts a duplicate extraction.
func (m *Module) Submit(ctx context.Context, req filament.RunRequest) (filament.RunID, error) {
	id, err := runs.Submit(ctx, m.bus, m.ds, req)
	if err != nil {
		return "", err
	}
	if m.log != nil {
		m.log.Info("run submitted", filament.Field{Key: "run", Value: string(id)}, filament.Field{Key: "tenant", Value: string(req.Tenant)})
	}
	return id, nil
}
