package events

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament/eventbus"
)

// Emit validates the envelope and publishes the fact under its subject. The
// payload travels as a Fact value; framing happens only at a transport edge.
func Emit[T any](ctx context.Context, bus eventbus.Bus, t EventType[T], env Envelope, data T) error {
	if !t.IsValid() {
		return errors.New("events: emit: invalid EventType")
	}
	if err := env.Tenant.Valid(); err != nil {
		return fmt.Errorf("events: emit %s: tenant: %w", t.Name(), err)
	}
	if err := env.Run.Valid(); err != nil {
		return fmt.Errorf("events: emit %s: run: %w", t.Name(), err)
	}
	return bus.Publish(ctx, Subject(t, env.Tenant, env.Run), NewFact(t, env, data))
}

// Publish validates and publishes an already-built Fact — the path for
// producers that construct facts behind an emit callback (see NewFact). The
// fact's Name must be registered in the catalog.
func Publish(ctx context.Context, bus eventbus.Bus, f Fact) error {
	def, ok := Lookup(f.Name)
	if !ok {
		return fmt.Errorf("events: publish: unknown event type %q", f.Name)
	}
	if err := f.Tenant.Valid(); err != nil {
		return fmt.Errorf("events: publish %s: tenant: %w", f.Name, err)
	}
	if err := f.Run.Valid(); err != nil {
		return fmt.Errorf("events: publish %s: run: %w", f.Name, err)
	}
	return bus.Publish(ctx, join(def.Entity, string(f.Tenant), string(f.Run), def.Event), f)
}
