// Package nats implements eventbus.Bus over NATS JetStream.
package nats

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	natsgo "github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament/eventbus"
)

const (
	defaultStream      = "EVENTBUS"
	defaultBuffer      = 64
	defaultBatch       = 64
	defaultAckWait     = 30 * time.Second
	defaultFetchWait   = 250 * time.Millisecond
	defaultRequestWait = 5 * time.Second
)

// Bus is a NATS JetStream-backed eventbus.Bus.
type Bus struct {
	nc      *natsgo.Conn
	js      natsgo.JetStreamContext
	codec   eventbus.Codec
	stream  string
	buffer  int
	batch   int
	ownConn bool
	logf    func(format string, args ...any)

	mu     sync.Mutex
	subs   map[*subscription]struct{}
	closed atomic.Bool
}

type config struct {
	url          string
	conn         *natsgo.Conn
	stream       string
	subjects     []string
	codec        eventbus.Codec
	buffer       int
	batch        int
	requestWait  time.Duration
	createStream bool
	logf         func(format string, args ...any)
}

// Option configures a Bus.
type Option func(*config)

// WithConn uses an existing NATS connection. The bus will not close it.
func WithConn(conn *natsgo.Conn) Option {
	return func(c *config) { c.conn = conn }
}

// WithStream sets the JetStream stream name.
func WithStream(name string) Option {
	return func(c *config) { c.stream = name }
}

// WithSubjects sets the stream subjects when the provider creates the stream.
func WithSubjects(subjects ...string) Option {
	return func(c *config) { c.subjects = append([]string(nil), subjects...) }
}

// WithBuffer sets the per-subscription delivery channel buffer.
func WithBuffer(n int) Option {
	return func(c *config) { c.buffer = n }
}

// WithMaxInFlight sets the default pull batch size and max ack pending.
func WithMaxInFlight(n int) Option {
	return func(c *config) { c.batch = n }
}

// WithRequestWait sets the timeout used for JetStream management requests.
func WithRequestWait(d time.Duration) Option {
	return func(c *config) { c.requestWait = d }
}

// WithCreateStream controls whether New creates the stream if it is missing.
func WithCreateStream(ok bool) Option {
	return func(c *config) { c.createStream = ok }
}

// WithLogf sets a log sink for subscription pump errors. Without it, fetch
// failures are retried silently.
func WithLogf(f func(format string, args ...any)) Option {
	return func(c *config) {
		if f != nil {
			c.logf = f
		}
	}
}

// New connects to NATS, opens JetStream, and returns a ready event bus.
func New(url string, codec eventbus.Codec, opts ...Option) (*Bus, error) {
	cfg := config{
		url:          url,
		stream:       defaultStream,
		subjects:     []string{eventbus.TailWildcard},
		codec:        codec,
		buffer:       defaultBuffer,
		batch:        defaultBatch,
		requestWait:  defaultRequestWait,
		createStream: true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.codec == nil {
		return nil, errors.New("eventbus/nats: nil codec")
	}
	if cfg.stream == "" {
		return nil, errors.New("eventbus/nats: empty stream")
	}
	if cfg.buffer <= 0 {
		cfg.buffer = defaultBuffer
	}
	if cfg.batch <= 0 {
		cfg.batch = defaultBatch
	}
	if cfg.requestWait <= 0 {
		cfg.requestWait = defaultRequestWait
	}

	nc := cfg.conn
	ownConn := false
	if nc == nil {
		if cfg.url == "" {
			return nil, errors.New("eventbus/nats: empty url")
		}
		var err error
		nc, err = natsgo.Connect(cfg.url,
			natsgo.RetryOnFailedConnect(true),
			natsgo.MaxReconnects(-1),
			natsgo.ReconnectWait(2*time.Second),
		)
		if err != nil {
			return nil, fmt.Errorf("eventbus/nats: connect: %w", err)
		}
		ownConn = true
		deadline := time.Now().Add(5 * time.Minute)
		for !nc.IsConnected() {
			if time.Now().After(deadline) {
				nc.Close()
				return nil, errors.New("eventbus/nats: connect: timed out waiting for nats")
			}
			time.Sleep(time.Second)
		}
	}

	js, err := nc.JetStream(natsgo.MaxWait(cfg.requestWait))
	if err != nil {
		if ownConn {
			nc.Close()
		}
		return nil, fmt.Errorf("eventbus/nats: jetstream: %w", err)
	}

	logf := cfg.logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	b := &Bus{
		nc:      nc,
		js:      js,
		codec:   cfg.codec,
		stream:  cfg.stream,
		buffer:  cfg.buffer,
		batch:   cfg.batch,
		ownConn: ownConn,
		logf:    logf,
		subs:    make(map[*subscription]struct{}),
	}
	if cfg.createStream {
		if err := b.ensureStream(cfg.subjects); err != nil {
			if ownConn {
				nc.Close()
			}
			return nil, err
		}
	}
	return b, nil
}

var (
	_ eventbus.Bus        = (*Bus)(nil)
	_ eventbus.Replayable = (*Bus)(nil)
)

// Name identifies this bus implementation.
func (b *Bus) Name() string { return "nats" }

// Resolve maps eventbus patterns directly to NATS subjects.
func (b *Bus) Resolve(pattern string) (eventbus.Route, error) {
	return eventbus.PassthroughResolver{}.Resolve(pattern)
}

// EnsureStream creates the named JetStream stream capturing subjects if it does
// not already exist. It is idempotent: an existing stream is left untouched.
// This is the optional provisioning capability the control plane discovers by
// type assertion; providers without a stream concept do not implement it.
func (b *Bus) EnsureStream(ctx context.Context, name string, subjects []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}
	if name == "" {
		return errors.New("eventbus/nats: empty stream name")
	}
	if len(subjects) == 0 {
		return fmt.Errorf("eventbus/nats: stream %q has no subjects", name)
	}
	if _, err := b.js.StreamInfo(name); err == nil {
		return nil
	} else if !errors.Is(err, natsgo.ErrStreamNotFound) {
		return fmt.Errorf("eventbus/nats: stream info %q: %w", name, err)
	}
	if _, err := b.js.AddStream(&natsgo.StreamConfig{
		Name:      name,
		Subjects:  subjects,
		Retention: natsgo.LimitsPolicy,
		Storage:   natsgo.FileStorage,
	}); err != nil {
		return fmt.Errorf("eventbus/nats: add stream %q: %w", name, err)
	}
	return nil
}

func (b *Bus) ensureStream(subjects []string) error {
	if len(subjects) == 0 {
		subjects = []string{eventbus.TailWildcard}
	}
	if _, err := b.js.StreamInfo(b.stream); err == nil {
		return nil
	} else if !errors.Is(err, natsgo.ErrStreamNotFound) {
		return fmt.Errorf("eventbus/nats: stream info: %w", err)
	}
	_, err := b.js.AddStream(&natsgo.StreamConfig{
		Name:      b.stream,
		Subjects:  subjects,
		Retention: natsgo.LimitsPolicy,
		Storage:   natsgo.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("eventbus/nats: add stream: %w", err)
	}
	return nil
}

// Publish encodes payload and publishes it under a concrete subject.
func (b *Bus) Publish(ctx context.Context, subject string, payload any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}
	if err := validSubject(subject); err != nil {
		return err
	}
	data, err := b.codec.Encode(payload)
	if err != nil {
		return fmt.Errorf("eventbus/nats: encode: %w", err)
	}
	_, err = b.js.PublishMsg(&natsgo.Msg{Subject: subject, Data: data}, natsgo.Context(ctx))
	if err != nil {
		return fmt.Errorf("eventbus/nats: publish: %w", err)
	}
	return nil
}

// Subscribe creates a live subscription. If opts.FromSeq is set, it replays from
// that stream sequence before tailing live.
func (b *Bus) Subscribe(pattern string, opts eventbus.SubOpts) (eventbus.Subscription, error) {
	return b.subscribe(context.Background(), pattern, opts)
}

// Replay subscribes from the matching backlog at fromSeq, then tails live.
func (b *Bus) Replay(ctx context.Context, pattern string, fromSeq uint64) (eventbus.Subscription, error) {
	if fromSeq == 0 {
		fromSeq = 1
	}
	return b.subscribe(ctx, pattern, eventbus.SubOpts{FromSeq: fromSeq})
}

func (b *Bus) subscribe(ctx context.Context, pattern string, opts eventbus.SubOpts) (eventbus.Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if b.closed.Load() {
		return nil, eventbus.ErrBusClosed
	}
	route, err := b.Resolve(pattern)
	if err != nil {
		return nil, err
	}

	subOpts := []natsgo.SubOpt{
		natsgo.BindStream(b.stream),
		natsgo.AckExplicit(),
		natsgo.ReplayInstant(),
		natsgo.MaxAckPending(maxInFlight(opts.MaxInFlight, b.batch)),
		natsgo.Context(ctx),
	}
	if opts.AckWait > 0 {
		subOpts = append(subOpts, natsgo.AckWait(opts.AckWait))
	} else {
		subOpts = append(subOpts, natsgo.AckWait(defaultAckWait))
	}
	switch {
	case opts.FromSeq > 0:
		subOpts = append(subOpts, natsgo.StartSequence(opts.FromSeq))
	case opts.Replay:
		subOpts = append(subOpts, natsgo.DeliverAll())
	default:
		subOpts = append(subOpts, natsgo.DeliverNew())
	}

	nsub, err := b.pullSubscribe(route.Target, opts.Durable, subOpts)
	if err != nil {
		return nil, fmt.Errorf("eventbus/nats: subscribe: %w", err)
	}

	s := &subscription{
		bus:     b,
		nsub:    nsub,
		route:   route,
		durable: opts.Durable,
		subOpts: subOpts,
		ch:      make(chan eventbus.Message, b.buffer),
		done:    make(chan struct{}),
		batch:   maxInFlight(opts.MaxInFlight, b.batch),
		filter:  route.ClientFilter,
	}

	b.mu.Lock()
	if b.closed.Load() {
		b.mu.Unlock()
		if opts.Durable == "" {
			_ = nsub.Unsubscribe()
		}
		return nil, eventbus.ErrBusClosed
	}
	b.subs[s] = struct{}{}
	b.mu.Unlock()

	s.wg.Add(1)
	go s.pump()
	return s, nil
}

// pullSubscribe binds the durable consumer, recovering from config drift: a
// durable created by an older build (e.g. a different deliver policy) rejects
// the bind, so the stale consumer is deleted and recreated with the current
// options. Durable consumers here fold facts idempotently (dedup on Seq), so
// the replay a recreate can trigger is safe; its ack state is lost either way.
func (b *Bus) pullSubscribe(target, durable string, subOpts []natsgo.SubOpt) (*natsgo.Subscription, error) {
	nsub, err := b.js.PullSubscribe(target, durable, subOpts...)
	if err == nil || durable == "" || !isConfigMismatch(err) {
		return nsub, err
	}
	if derr := b.js.DeleteConsumer(b.stream, durable); derr != nil {
		return nil, fmt.Errorf("%w (delete for recreate: %v)", err, derr)
	}
	b.logf("eventbus/nats: recreating durable %q on %q: %v", durable, target, err)
	return b.js.PullSubscribe(target, durable, subOpts...)
}

// isConfigMismatch reports whether err is the client rejecting a bind because
// the existing consumer's config differs from the requested one. nats.go
// returns these as plain formatted errors ("nats: configuration requests X to
// be Y, but consumer's value is Z"), so string matching is the only handle.
func isConfigMismatch(err error) bool {
	return strings.Contains(err.Error(), "configuration requests")
}

// Close stops the bus and closes active subscriptions.
func (b *Bus) Close() error {
	if !b.closed.CompareAndSwap(false, true) {
		return nil
	}
	b.mu.Lock()
	subs := make([]*subscription, 0, len(b.subs))
	for s := range b.subs {
		subs = append(subs, s)
	}
	b.mu.Unlock()
	for _, s := range subs {
		_ = s.Close()
	}
	if b.ownConn {
		b.nc.Close()
	}
	return nil
}

func (b *Bus) remove(s *subscription) {
	b.mu.Lock()
	delete(b.subs, s)
	b.mu.Unlock()
}

type subscription struct {
	bus     *Bus
	route   eventbus.Route
	durable string
	subOpts []natsgo.SubOpt
	filter  eventbus.Filter
	ch      chan eventbus.Message
	done    chan struct{}
	batch   int

	mu   sync.Mutex
	nsub *natsgo.Subscription

	closeOnce sync.Once
	wg        sync.WaitGroup
}

var _ eventbus.Subscription = (*subscription)(nil)

func (s *subscription) C() <-chan eventbus.Message { return s.ch }

// Close stops the pump. Ephemeral subscriptions unsubscribe; durable ones
// deliberately do not — Unsubscribe deletes the durable consumer server-side,
// which would strand every other process bound to it and lose its position.
func (s *subscription) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		if s.durable == "" {
			_ = s.sub().Unsubscribe()
		}
		s.bus.remove(s)
		s.wg.Wait()
		close(s.ch)
	})
	return nil
}

func (s *subscription) sub() *natsgo.Subscription {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.nsub
}

// resubscribe recreates the pull subscription after the server-side consumer
// disappeared. A durable recreated this way resumes per its deliver policy
// (DeliverAll replays and relies on the consumer's dedup; DeliverNew tails).
func (s *subscription) resubscribe() error {
	nsub, err := s.bus.pullSubscribe(s.route.Target, s.durable, s.subOpts)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.nsub = nsub
	s.mu.Unlock()
	return nil
}

func (s *subscription) pump() {
	defer s.wg.Done()
	var lastErr string
	for {
		select {
		case <-s.done:
			return
		default:
		}

		msgs, err := s.sub().Fetch(s.batch, natsgo.MaxWait(defaultFetchWait))
		if err != nil {
			if errors.Is(err, natsgo.ErrTimeout) {
				lastErr = ""
				continue
			}
			if err.Error() != lastErr {
				lastErr = err.Error()
				s.bus.logf("eventbus/nats: fetch %q (durable %q): %v", s.route.Target, s.durable, err)
			}
			if s.durable != "" && (errors.Is(err, natsgo.ErrConsumerNotFound) || errors.Is(err, natsgo.ErrConsumerDeleted)) {
				if rerr := s.resubscribe(); rerr == nil {
					s.bus.logf("eventbus/nats: recreated durable %q on %q", s.durable, s.route.Target)
					lastErr = ""
					continue
				}
			}
			select {
			case <-s.done:
				return
			case <-time.After(defaultFetchWait):
				continue
			}
		}
		for _, msg := range msgs {
			if !s.route.Exact && !s.filter.MatchSubject(msg.Subject) {
				_ = msg.Ack()
				continue
			}
			payload, err := s.bus.codec.Decode(msg.Data)
			if err != nil {
				_ = msg.Term()
				continue
			}
			em := &message{msg: msg, subject: msg.Subject, payload: payload}
			if md, err := msg.Metadata(); err == nil {
				em.seq = md.Sequence.Stream
			}
			select {
			case s.ch <- em:
			case <-s.done:
				_ = msg.Nak()
				return
			}
		}
	}
}

type message struct {
	msg     *natsgo.Msg
	subject string
	payload any
	seq     uint64
}

var _ eventbus.Message = (*message)(nil)

func (m *message) Subject() string { return m.subject }
func (m *message) Payload() any    { return m.payload }
func (m *message) Seq() uint64     { return m.seq }
func (m *message) Ack() error      { return m.msg.Ack() }
func (m *message) Nak() error      { return m.msg.Nak() }

func maxInFlight(opt, fallback int) int {
	if opt > 0 {
		return opt
	}
	if fallback > 0 {
		return fallback
	}
	return defaultBatch
}

func validSubject(subject string) error {
	if subject == "" {
		return fmt.Errorf("%w: empty subject", eventbus.ErrInvalidToken)
	}
	for _, tok := range strings.Split(subject, eventbus.Separator) {
		if err := eventbus.ValidToken(tok); err != nil {
			return err
		}
	}
	return nil
}
