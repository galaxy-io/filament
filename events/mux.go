package events

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/galaxy-io/filament/eventbus"
)

// On subscribes to one event kind on its own subscription and pumps typed
// deliveries to fn. nil ⇒ ack; non-nil ⇒ nak (redeliver). Canceling ctx closes
// the subscription, as does the returned func. Modules consuming many kinds
// through one durable consumer use Mux instead.
func On[T any](ctx context.Context, bus eventbus.Bus, t EventType[T], opts eventbus.SubOpts, fn func(context.Context, Event[T]) error) (func(), error) {
	if !t.IsValid() {
		return nil, errors.New("events: on: invalid EventType")
	}
	sub, err := bus.Subscribe(SubjectPattern(t), opts)
	if err != nil {
		return nil, err
	}

	// Ends the subscription at the bus level on ctx cancel; done releases the
	// watcher when the pump exits first.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = sub.Close()
		case <-done:
		}
	}()

	go func() {
		defer close(done)
		for msg := range sub.C() {
			if ctx.Err() != nil {
				_ = msg.Nak() // canceled mid-flight: redeliver, don't execute
				continue
			}
			f, err := Decode(msg)
			if err != nil {
				_ = msg.Ack() // not a Fact: a foreign publisher on our pattern — skip
				continue
			}
			payload, ok := f.Data.(T)
			if !ok {
				_ = msg.Ack()
				continue
			}
			if err := fn(ctx, Event[T]{Envelope: f.Envelope, Data: payload}); err != nil {
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}()
	return func() { _ = sub.Close() }, nil
}

// Handler adapts a typed handler to the raw message signature, for declarative
// subscription tables (host.Subscription) that consume a single kind. Facts of
// another kind — or payloads that are not Facts — are acked and skipped.
func Handler[T any](t EventType[T], fn func(context.Context, Event[T]) error) func(context.Context, eventbus.Message) error {
	return func(ctx context.Context, msg eventbus.Message) error {
		f, err := Decode(msg)
		if err != nil {
			return nil
		}
		if f.Name != t.Name() {
			return nil
		}
		payload, ok := f.Data.(T)
		if !ok {
			return nil
		}
		return fn(ctx, Event[T]{Envelope: f.Envelope, Data: payload})
	}
}

// Mux fans one subscription out to typed handlers by event kind — the shape
// for modules that consume many kinds through a single durable consumer
// (one cursor, one dedup sequence). Wire Dispatch as the subscription handler.
type Mux struct {
	mu       sync.RWMutex
	handlers map[string][]func(context.Context, Envelope, any) error
	anys     []func(context.Context, Fact) error
}

// NewMux returns an empty Mux.
func NewMux() *Mux {
	return &Mux{handlers: map[string][]func(context.Context, Envelope, any) error{}}
}

// Handle attaches a typed handler for one event kind. (A free function: Go
// methods cannot take type parameters.)
func Handle[T any](m *Mux, t EventType[T], fn func(context.Context, Event[T]) error) {
	wrapped := func(ctx context.Context, env Envelope, data any) error {
		payload, ok := data.(T)
		if !ok {
			return fmt.Errorf("events: %s payload is %T", t.Name(), data)
		}
		return fn(ctx, Event[T]{Envelope: env, Data: payload})
	}
	m.mu.Lock()
	m.handlers[t.Name()] = append(m.handlers[t.Name()], wrapped)
	m.mu.Unlock()
}

// HandleAny attaches a handler for every event kind — for forwarding
// consumers that treat facts uniformly.
func (m *Mux) HandleAny(fn func(context.Context, Fact) error) {
	m.mu.Lock()
	m.anys = append(m.anys, fn)
	m.mu.Unlock()
}

// Dispatch extracts one delivered Fact and runs every matching handler. All
// handlers run even when an earlier one fails; errors are joined so the
// caller naks once. A payload that is not a Fact (a foreign publisher on the
// consumed pattern) is skipped — redelivery cannot fix it.
func (m *Mux) Dispatch(ctx context.Context, msg eventbus.Message) error {
	f, err := Decode(msg)
	if err != nil {
		return nil
	}

	m.mu.RLock()
	typed := append([]func(context.Context, Envelope, any) error(nil), m.handlers[f.Name]...)
	anys := append([]func(context.Context, Fact) error(nil), m.anys...)
	m.mu.RUnlock()

	var errs []error
	for _, fn := range typed {
		if err := fn(ctx, f.Envelope, f.Data); err != nil {
			errs = append(errs, err)
		}
	}
	for _, fn := range anys {
		if err := fn(ctx, f); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
