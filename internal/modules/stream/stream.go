// Package stream is the client-facing read side of the plane. Snapshot returns a
// run's current state from the DataStore; Tail streams its facts live off the bus
// so a UI can render progress without polling. It declares no subscriptions and
// opens an ad-hoc subscription per tail.
package stream

import (
	"context"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/module"
)

// Module is the client-facing read side.
type Module struct {
	bus eventbus.Bus
	ds  ingestion.DataStore
}

// New returns an unmounted stream module. Providers are injected by Mount.
func New() *Module { return &Module{} }

var _ module.Module = (*Module)(nil)

func (m *Module) Name() string { return "stream" }

// Subscriptions returns none; tails are opened per request, not at mount.
func (m *Module) Subscriptions() []host.Subscription { return nil }

func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.bus = d.Bus
	m.ds = d.DataStore
	return nil
}

// Snapshot returns the run's current persisted state and resource states.
func (m *Module) Snapshot(ctx context.Context, run ingestion.RunID) (ingestion.SyncSnapshot, error) {
	rs, err := m.ds.LoadRun(ctx, run)
	if err != nil {
		return ingestion.SyncSnapshot{}, err
	}
	return ingestion.SyncSnapshot{Run: rs, Resources: rs.Resources}, nil
}

// Tail streams a run's facts live, returning a channel that closes when ctx is
// cancelled or the subscription drops. Each fact is acked on read, so a slow
// reader only backpressures the tail, never producers.
func (m *Module) Tail(ctx context.Context, tenant ingestion.TenantID, run ingestion.RunID) (<-chan ingestion.Event, error) {
	sub, err := m.bus.Subscribe(ingestion.RunPattern(tenant, run), eventbus.SubOpts{})
	if err != nil {
		return nil, err
	}
	out := make(chan ingestion.Event)
	go func() {
		defer close(out)
		defer func() { _ = sub.Close() }()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-sub.C():
				if !ok {
					return
				}
				ev, err := ingestion.EventOf(msg)
				_ = msg.Ack()
				if err != nil {
					continue
				}
				select {
				case out <- ev:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}
