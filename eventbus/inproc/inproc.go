// Package inproc implements the in-memory eventbus provider: channel-backed
// delivery, ack/nak redelivery, and replay from an in-memory log. Payloads are
// passed by reference, never marshaled.
package inproc

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/galaxy-io/filament/eventbus"
)

const (
	defaultBuffer        = 64
	defaultMaxDeliveries = 16 // safety cap so an always-failing handler can't spin forever
)

// Bus is an in-memory eventbus.Bus.
type Bus struct {
	buffer        int
	maxDeliveries int
	maxLog        int // 0 = unbounded replay log

	mu     sync.Mutex
	seq    uint64
	log    []record
	subs   map[*subscription]struct{}
	closed bool
}

// Resolve is in-proc's eventbus.Resolver — the worked example of a provider
// implementing the contract. With no broker, every pattern resolves to a pure
// client-side route: Target is empty, so all matching happens on ClientFilter. A
// broker-backed provider would instead push as much as it can into Target (see
// eventbus.PassthroughResolver / eventbus.PrefixTopicResolver).
func (b *Bus) Resolve(pattern string) (eventbus.Route, error) {
	f, err := eventbus.Compile(pattern)
	if err != nil {
		return eventbus.Route{}, err
	}
	return eventbus.Route{ClientFilter: f}, nil
}

type record struct {
	seq     uint64
	subject string
	tokens  []string // subject split once at publish, shared across matches
	payload any
}

// Option configures a Bus.
type Option func(*Bus)

// WithBuffer sets the per-subscription channel buffer (default 64).
func WithBuffer(n int) Option { return func(b *Bus) { b.buffer = n } }

// WithMaxDeliveries caps redelivery attempts per message; 0 = unlimited (default 16).
func WithMaxDeliveries(n int) Option { return func(b *Bus) { b.maxDeliveries = n } }

// WithMaxLog bounds the retained replay log to ~n events (memory ≈ 2n, amortized
// O(1) eviction of the oldest). 0 = unbounded (default). Replay from an evicted
// sequence returns only what is still retained.
func WithMaxLog(n int) Option { return func(b *Bus) { b.maxLog = n } }

// New returns a ready-to-use in-memory bus.
func New(opts ...Option) *Bus {
	b := &Bus{
		buffer:        defaultBuffer,
		maxDeliveries: defaultMaxDeliveries,
		subs:          make(map[*subscription]struct{}),
	}
	for _, o := range opts {
		o(b)
	}
	return b
}

var (
	_ eventbus.Bus              = (*Bus)(nil)
	_ eventbus.ReadinessChecker = (*Bus)(nil)
	_ eventbus.Replayable       = (*Bus)(nil)
)

// Name identifies this bus implementation.
func (b *Bus) Name() string { return "inproc" }

// Ready reports whether the in-process bus is still accepting work.
func (b *Bus) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return eventbus.ErrBusClosed
	}
	return nil
}

// Publish assigns a monotonic sequence, logs the payload for replay, and fans
// it out to every matching subscription. The subject is split once and reused
// across candidates. Delivery is synchronous and ordered.
func (b *Bus) Publish(ctx context.Context, subject string, payload any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if subject == "" {
		return fmt.Errorf("%w: empty subject", eventbus.ErrInvalidToken)
	}

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return eventbus.ErrBusClosed
	}
	b.seq++
	rec := record{seq: b.seq, subject: subject, tokens: strings.Split(subject, eventbus.Separator), payload: payload}
	b.log = append(b.log, rec)
	if b.maxLog > 0 && len(b.log) >= 2*b.maxLog {
		b.log = append(b.log[:0], b.log[len(b.log)-b.maxLog:]...) // amortized eviction
	}
	targets := b.matchingLocked(rec)
	b.mu.Unlock()

	for _, s := range targets {
		s.deliver(b.newMessage(rec, s))
	}
	return nil
}

// Subscribe registers interest in a pattern, replaying the backlog from
// opts.FromSeq first if it is > 0.
func (b *Bus) Subscribe(pattern string, opts eventbus.SubOpts) (eventbus.Subscription, error) {
	return b.subscribe(pattern, opts.Durable, opts.FromSeq)
}

// Replay subscribes from the matching backlog at fromSeq, then tails live.
// fromSeq <= 1 replays everything.
func (b *Bus) Replay(ctx context.Context, pattern string, fromSeq uint64) (eventbus.Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if fromSeq == 0 {
		fromSeq = 1
	}
	return b.subscribe(pattern, "", fromSeq)
}

func (b *Bus) subscribe(pattern, durable string, fromSeq uint64) (eventbus.Subscription, error) {
	// in-proc has no broker: resolve to a client-side route (Target is empty,
	// matching happens entirely on ClientFilter). Bad patterns fail fast here.
	route, err := b.Resolve(pattern)
	if err != nil {
		return nil, err
	}
	filter := route.ClientFilter

	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, eventbus.ErrBusClosed
	}
	s := b.newSubscription(pattern, durable)
	s.filter = filter

	var history []record
	if fromSeq > 0 {
		for _, rec := range b.log {
			if rec.seq >= fromSeq && filter.Match(rec.tokens) {
				history = append(history, rec)
			}
		}
	}
	b.subs[s] = struct{}{}
	b.mu.Unlock()

	if len(history) > 0 {
		go func() {
			for _, rec := range history {
				s.deliver(b.newMessage(rec, s))
			}
		}()
	}
	return s, nil
}

// Close stops the bus and closes every subscription.
func (b *Bus) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	subs := make([]*subscription, 0, len(b.subs))
	for s := range b.subs {
		subs = append(subs, s)
	}
	b.mu.Unlock()

	for _, s := range subs {
		_ = s.Close()
	}
	return nil
}

func (b *Bus) matchingLocked(rec record) []*subscription {
	var out []*subscription
	for s := range b.subs {
		if s.filter.Match(rec.tokens) {
			out = append(out, s)
		}
	}
	return out
}

func (b *Bus) remove(s *subscription) {
	b.mu.Lock()
	delete(b.subs, s)
	b.mu.Unlock()
}

func (b *Bus) newSubscription(pattern, durable string) *subscription {
	return &subscription{
		bus:     b,
		pattern: pattern,
		durable: durable,
		ch:      make(chan eventbus.Message, b.buffer),
		done:    make(chan struct{}),
	}
}

func (b *Bus) newMessage(rec record, s *subscription) *message {
	return &message{sub: s, subject: rec.subject, seq: rec.seq, payload: rec.payload, tries: 1}
}

// ── Subscription ────────────────────────────────────────────────────────────

type subscription struct {
	bus     *Bus
	pattern string          // canonical string form (debug/reference)
	filter  eventbus.Filter // compiled fast-path matcher
	durable string

	ch   chan eventbus.Message
	done chan struct{}

	mu       sync.Mutex
	closed   bool
	inflight sync.WaitGroup
}

var _ eventbus.Subscription = (*subscription)(nil)

func (s *subscription) C() <-chan eventbus.Message { return s.ch }

// deliver sends m to the subscriber, blocking on backpressure but bailing if the
// subscription is closed. The inflight WaitGroup lets Close drain senders before
// closing the channel, so a send-on-closed-channel panic is impossible.
func (s *subscription) deliver(m eventbus.Message) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.inflight.Add(1)
	s.mu.Unlock()
	defer s.inflight.Done()

	select {
	case s.ch <- m:
	case <-s.done:
	}
}

// Close unsubscribes, unblocks any in-flight deliver, waits for them to drain,
// then closes the channel so consumers ranging over C() exit cleanly.
func (s *subscription) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	close(s.done)
	s.mu.Unlock()

	s.bus.remove(s)
	s.inflight.Wait()
	close(s.ch)
	return nil
}

// ── Message ─────────────────────────────────────────────────────────────────

type message struct {
	sub     *subscription
	subject string
	seq     uint64
	payload any
	tries   int
}

var _ eventbus.Message = (*message)(nil)

func (m *message) Subject() string { return m.subject }
func (m *message) Payload() any    { return m.payload }
func (m *message) Seq() uint64     { return m.seq }

// Ack marks the message consumed. In-memory delivery is fire-and-forget, so this
// is a no-op — it exists to satisfy the at-least-once contract.
func (m *message) Ack() error { return nil }

// Nak requests redelivery. The same message is re-enqueued asynchronously, up to
// the bus's max-deliveries cap (after which it is dropped — this dev bus has no
// dead-letter queue).
func (m *message) Nak() error {
	maxDel := m.sub.bus.maxDeliveries
	if maxDel > 0 && m.tries >= maxDel {
		return nil
	}
	m.tries++
	go m.sub.deliver(m)
	return nil
}
