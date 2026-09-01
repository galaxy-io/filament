// Package host runs modules against an [eventbus.Bus]: it subscribes each
// Runnable's declared patterns and pumps deliveries to handlers with
// at-least-once ack/nak. It is domain-free — wiring happens in the domain
// before Run, so any service can share one Host.
package host

import (
	"context"
	"fmt"
	"sync"

	"github.com/galaxy-io/filament/eventbus"
)

// Handler processes one delivered message. nil ⇒ ack; non-nil ⇒ nak (redeliver).
type Handler func(ctx context.Context, msg eventbus.Message) error

// Subscription declares one pattern a module consumes and its handler.
type Subscription struct {
	Pattern string // e.g. "ingestion.v1.run.*.*.requested"
	Durable string // durable consumer name; "" = ephemeral
	Replay  bool   // deliver the retained backlog on first creation (idempotent folds only)

	// MaxInFlight caps unacked deliveries across every process bound to the
	// durable (0 = transport default). 1 serializes the consumer: one message
	// at a time, in stream order, no matter how many replicas subscribe —
	// required by handlers that fold with read-modify-write.
	MaxInFlight int

	Handler Handler
}

// Runnable is anything a Host can mount: a name and the subjects it consumes.
type Runnable interface {
	Name() string
	Subscriptions() []Subscription
}

// Host subscribes Runnables to a bus and pumps deliveries to their handlers.
type Host struct {
	bus          eventbus.Bus
	logf         func(format string, args ...any)
	handlerError func(module, pattern, durable string, seq uint64, err error)

	mu   sync.Mutex
	mods []Runnable
	subs []eventbus.Subscription
	wg   sync.WaitGroup
}

// Option configures a Host.
type Option func(*Host)

// WithLogf sets a log sink for handler errors and lifecycle messages.
func WithLogf(f func(format string, args ...any)) Option {
	return func(h *Host) {
		if f != nil {
			h.logf = f
		}
	}
}

// WithHandlerError sets a structured sink for handler failures. When unset,
// handler failures continue through WithLogf for backward compatibility.
func WithHandlerError(f func(module, pattern, durable string, seq uint64, err error)) Option {
	return func(h *Host) { h.handlerError = f }
}

// New returns a Host bound to bus.
func New(bus eventbus.Bus, opts ...Option) *Host {
	h := &Host{bus: bus, logf: func(string, ...any) {}}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Run subscribes every module's subscriptions and spawns one pump goroutine per
// subscription. Pumps run until ctx is cancelled or Close is called. Safe to
// call again to add modules to a live Host.
func (h *Host) Run(ctx context.Context, mods ...Runnable) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.bus == nil {
		return fmt.Errorf("host: cannot Run without a bus")
	}
	for _, m := range mods {
		h.mods = append(h.mods, m)
		for _, s := range m.Subscriptions() {
			sub, err := h.bus.Subscribe(s.Pattern, eventbus.SubOpts{Durable: s.Durable, Replay: s.Replay, MaxInFlight: s.MaxInFlight})
			if err != nil {
				return fmt.Errorf("subscribe %q for module %q: %w", s.Pattern, m.Name(), err)
			}
			h.subs = append(h.subs, sub)
			h.wg.Add(1)
			go h.pump(ctx, m, s, sub)
		}
		h.logf("mounted module %q", m.Name())
	}
	return nil
}

// pump delivers from one subscription to its handler, ack-ing on success and
// nak-ing on error. This is the at-least-once edge; effectively-once comes from
// handlers de-duping on the stream sequence.
func (h *Host) pump(ctx context.Context, m Runnable, s Subscription, sub eventbus.Subscription) {
	defer h.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-sub.C():
			if !ok {
				return
			}
			if err := s.Handler(ctx, msg); err != nil {
				if h.handlerError != nil {
					h.handlerError(m.Name(), s.Pattern, s.Durable, msg.Seq(), err)
				} else {
					h.logf("module %q handler error on %q: %v", m.Name(), s.Pattern, err)
				}
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}
}

// Mounted returns the names of mounted modules, in mount order.
func (h *Host) Mounted() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	names := make([]string, len(h.mods))
	for i, m := range h.mods {
		names[i] = m.Name()
	}
	return names
}

// Close unsubscribes everything and waits for the pump goroutines to drain.
func (h *Host) Close() error {
	h.mu.Lock()
	subs := h.subs
	h.subs = nil
	h.mu.Unlock()
	for _, s := range subs {
		_ = s.Close()
	}
	h.wg.Wait()
	return nil
}
