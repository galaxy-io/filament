// Package nats implements eventbus.Bus over NATS JetStream.
package nats

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament/eventbus"
)

const (
	defaultStream      = "EVENTBUS"
	defaultBuffer      = 64
	defaultBatch       = 64
	defaultAckWait     = 30 * time.Second
	defaultFetchWait   = 250 * time.Millisecond
	defaultRequestWait = 5 * time.Second
	defaultConnectWait = 90 * time.Second
	defaultTTL         = 7 * 24 * time.Hour

	// nakRedeliveryDelay spaces redeliveries of a nak'd message. A plain NAK
	// redelivers immediately and the consumer BackOff schedule applies only to
	// AckWait expiry, so without it a handler failing on a broken dependency
	// spins the consumer.
	nakRedeliveryDelay = time.Second
	// ephemeralInactiveThreshold lets the server garbage-collect ephemeral
	// consumers abandoned by a crashed process; live ones stay active by
	// fetching and clean ones are deleted explicitly on Close.
	ephemeralInactiveThreshold = 5 * time.Minute
)

// Bus is a NATS JetStream-backed eventbus.Bus.
type Bus struct {
	nc          *natsgo.Conn
	js          jetstream.JetStream
	codec       eventbus.Codec
	stream      string
	buffer      int
	batch       int
	requestWait time.Duration
	ownConn     bool
	logf        func(format string, args ...any)

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
	ttl, err := ttlFromEnv()
	if err != nil {
		return nil, err
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
		deadline := time.Now().Add(defaultConnectWait)
		for !nc.IsConnected() {
			if time.Now().After(deadline) {
				nc.Close()
				return nil, errors.New("eventbus/nats: connect: timed out waiting for nats")
			}
			time.Sleep(time.Second)
		}
	}

	js, err := jetstream.New(nc, jetstream.WithDefaultTimeout(cfg.requestWait))
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
		nc:          nc,
		js:          js,
		codec:       cfg.codec,
		stream:      cfg.stream,
		buffer:      cfg.buffer,
		batch:       cfg.batch,
		requestWait: cfg.requestWait,
		ownConn:     ownConn,
		logf:        logf,
		subs:        make(map[*subscription]struct{}),
	}
	if cfg.createStream {
		if err := b.ensureStream(context.Background(), b.stream, cfg.subjects, ttl); err != nil {
			if ownConn {
				nc.Close()
			}
			return nil, err
		}
	}
	return b, nil
}

// ttlFromEnv returns the process-wide NATS retention setting. Empty uses the
// seven-day default; zero explicitly disables age-based expiration.
func ttlFromEnv() (time.Duration, error) {
	raw := os.Getenv("NATS_TTL_SECONDS")
	if raw == "" {
		return defaultTTL, nil
	}
	seconds, err := strconv.ParseInt(raw, 10, 64)
	const maxDurationSeconds = int64((1<<63 - 1) / int64(time.Second))
	if err != nil || seconds < 0 || seconds > maxDurationSeconds {
		return 0, fmt.Errorf("eventbus/nats: NATS_TTL_SECONDS must be a non-negative integer, got %q", raw)
	}
	return time.Duration(seconds) * time.Second, nil
}

var (
	_ eventbus.Bus              = (*Bus)(nil)
	_ eventbus.ReadinessChecker = (*Bus)(nil)
	_ eventbus.Replayable       = (*Bus)(nil)
)

// Name identifies this bus implementation.
func (b *Bus) Name() string { return "nats" }

// Ready verifies both the NATS connection and the JetStream control plane.
func (b *Bus) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.closed.Load() {
		return eventbus.ErrBusClosed
	}
	if b.nc == nil || !b.nc.IsConnected() {
		return errors.New("eventbus/nats: disconnected")
	}
	if b.js == nil {
		return errors.New("eventbus/nats: jetstream unavailable")
	}
	if _, err := b.js.AccountInfo(ctx); err != nil {
		return fmt.Errorf("eventbus/nats: readiness: %w", err)
	}
	return nil
}

// Resolve maps eventbus patterns directly to NATS subjects.
func (b *Bus) Resolve(pattern string) (eventbus.Route, error) {
	return eventbus.PassthroughResolver{}.Resolve(pattern)
}

// ensureStream creates the named JetStream stream or reconciles its message TTL
// when it already exists. Other existing stream settings remain untouched.
func (b *Bus) ensureStream(ctx context.Context, name string, subjects []string, ttl time.Duration) error {
	if len(subjects) == 0 {
		subjects = []string{eventbus.TailWildcard}
	}
	if stream, err := b.js.Stream(ctx, name); err == nil {
		info := stream.CachedInfo()
		if info.Config.MaxAge == ttl {
			return nil
		}
		cfg := info.Config
		cfg.MaxAge = ttl
		if _, err := b.js.UpdateStream(ctx, cfg); err != nil {
			return fmt.Errorf("eventbus/nats: update stream %q ttl: %w", name, err)
		}
		return nil
	} else if !errors.Is(err, jetstream.ErrStreamNotFound) {
		return fmt.Errorf("eventbus/nats: stream info %q: %w", name, err)
	}
	if _, err := b.js.CreateStream(ctx, jetstream.StreamConfig{
		Name:      name,
		Subjects:  subjects,
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		MaxAge:    ttl,
	}); err != nil {
		return fmt.Errorf("eventbus/nats: add stream %q: %w", name, err)
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
	if _, err := b.js.Publish(ctx, subject, data); err != nil {
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

	cfg := jetstream.ConsumerConfig{
		Durable:       opts.Durable,
		FilterSubject: route.Target,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       defaultAckWait,
		MaxAckPending: maxInFlight(opts.MaxInFlight, b.batch),
	}
	if opts.AckWait > 0 {
		cfg.AckWait = opts.AckWait
	}
	switch {
	case opts.FromSeq > 0:
		cfg.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
		cfg.OptStartSeq = opts.FromSeq
	case opts.Replay:
		cfg.DeliverPolicy = jetstream.DeliverAllPolicy
	default:
		cfg.DeliverPolicy = jetstream.DeliverNewPolicy
	}
	if opts.Durable == "" {
		cfg.InactiveThreshold = ephemeralInactiveThreshold
	}

	cons, err := b.ensureConsumer(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("eventbus/nats: subscribe pattern %q durable %q stream %q: %w", pattern, opts.Durable, b.stream, err)
	}

	s := &subscription{
		bus:    b,
		cons:   cons,
		cfg:    cfg,
		route:  route,
		ch:     make(chan eventbus.Message, b.buffer),
		done:   make(chan struct{}),
		batch:  maxInFlight(opts.MaxInFlight, b.batch),
		filter: route.ClientFilter,
	}

	b.mu.Lock()
	if b.closed.Load() {
		b.mu.Unlock()
		s.deleteEphemeral()
		return nil, eventbus.ErrBusClosed
	}
	b.subs[s] = struct{}{}
	b.mu.Unlock()

	s.wg.Add(1)
	go s.pump()
	return s, nil
}

// ensureConsumer creates or binds the consumer. CreateOrUpdateConsumer
// reconciles updatable fields in place; changing a non-updatable field (e.g.
// deliver policy) errors at subscribe, and the fix is deleting the consumer
// so the next boot recreates it.
func (b *Bus) ensureConsumer(ctx context.Context, cfg jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	return b.js.CreateOrUpdateConsumer(ctx, b.stream, cfg)
}

func consumerGone(err error) bool {
	return errors.Is(err, jetstream.ErrConsumerNotFound) ||
		errors.Is(err, jetstream.ErrConsumerDeleted) ||
		errors.Is(err, natsgo.ErrNoResponders)
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
	bus    *Bus
	cfg    jetstream.ConsumerConfig
	route  eventbus.Route
	filter eventbus.Filter
	ch     chan eventbus.Message
	done   chan struct{}
	batch  int

	mu   sync.Mutex
	cons jetstream.Consumer

	closeOnce sync.Once
	wg        sync.WaitGroup
}

var _ eventbus.Subscription = (*subscription)(nil)

func (s *subscription) C() <-chan eventbus.Message { return s.ch }

// Close stops the pump. Ephemeral consumers are deleted; durable ones
// deliberately survive so other processes stay bound and the kept position
// resumes on the next subscribe.
func (s *subscription) Close() error {
	s.closeOnce.Do(func() {
		close(s.done)
		s.deleteEphemeral()
		s.bus.remove(s)
		s.wg.Wait()
		close(s.ch)
	})
	return nil
}

func (s *subscription) deleteEphemeral() {
	if s.cfg.Durable != "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.bus.requestWait)
	defer cancel()
	_ = s.bus.js.DeleteConsumer(ctx, s.bus.stream, s.consumer().CachedInfo().Name)
}

func (s *subscription) consumer() jetstream.Consumer {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cons
}

func (s *subscription) stopped() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// recreate rebuilds the server-side consumer after it disappeared. A durable
// recreated this way resumes per its deliver policy (DeliverAll replays and
// relies on the consumer's dedup; DeliverNew tails).
func (s *subscription) recreate() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.bus.requestWait)
	defer cancel()
	cons, err := s.bus.ensureConsumer(ctx, s.cfg)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.cons = cons
	s.mu.Unlock()
	return nil
}

func (s *subscription) pump() {
	defer s.wg.Done()
	var lastErr string
	for {
		if s.stopped() {
			return
		}

		batch, err := s.consumer().Fetch(s.batch, jetstream.FetchMaxWait(defaultFetchWait))
		if err == nil {
			for msg := range batch.Messages() {
				if !s.route.Exact && !s.filter.MatchSubject(msg.Subject()) {
					_ = msg.Ack()
					continue
				}
				payload, derr := s.bus.codec.Decode(msg.Data())
				if derr != nil {
					s.bus.logDecodeFailure(msg.Subject(), derr)
					_ = msg.Term()
					continue
				}
				em := &message{msg: msg, subject: msg.Subject(), payload: payload}
				if md, merr := msg.Metadata(); merr == nil {
					em.seq = md.Sequence.Stream
				}
				select {
				case s.ch <- em:
				case <-s.done:
					_ = msg.Nak()
					return
				}
			}
			err = batch.Error()
			if err == nil {
				lastErr = ""
				continue
			}
		}

		if s.stopped() {
			return
		}
		if err.Error() != lastErr {
			lastErr = err.Error()
			s.bus.logf("eventbus/nats: fetch %q (durable %q): %v", s.route.Target, s.cfg.Durable, err)
		}
		if s.cfg.Durable != "" && consumerGone(err) {
			if rerr := s.recreate(); rerr == nil {
				s.bus.logf("eventbus/nats: recreated durable %q on %q", s.cfg.Durable, s.route.Target)
				lastErr = ""
				continue
			}
		}
		select {
		case <-s.done:
			return
		case <-time.After(defaultFetchWait):
		}
	}
}

func (b *Bus) logDecodeFailure(subject string, err error) {
	if b.logf == nil {
		return
	}
	b.logf("eventbus/nats: decode %q: %v; terminating poison message", subject, err)
}

type message struct {
	msg     jetstream.Msg
	subject string
	payload any
	seq     uint64
}

var (
	_ eventbus.Message          = (*message)(nil)
	_ eventbus.ProgressReporter = (*message)(nil)
)

func (m *message) Subject() string   { return m.subject }
func (m *message) Payload() any      { return m.payload }
func (m *message) Seq() uint64       { return m.seq }
func (m *message) InProgress() error { return m.msg.InProgress() }
func (m *message) Ack() error        { return m.msg.Ack() }
func (m *message) Nak() error        { return m.msg.NakWithDelay(nakRedeliveryDelay) }

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
