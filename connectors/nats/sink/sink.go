// Package sink publishes append-only JSON records to NATS JetStream.
package sink

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/nats/internal/connection"
	"github.com/galaxy-io/filament/connectors/nats/internal/subject"
	"github.com/galaxy-io/filament/internal/stream"
)

type publisher interface {
	PublishMsgAsync(*nats.Msg, ...jetstream.PublishOpt) (jetstream.PubAckFuture, error)
	CleanupPublisher()
}

// Sink serializes Apply calls, and never retains borrowed Arrow batches.
// Already acknowledged messages survive aborts. Failed publishes may have
// unknown outcomes, which prevent reporting a quiescent streaming session.
type Sink struct {
	mu                           sync.Mutex
	conn                         *nats.Conn
	publisher                    publisher
	publications                 atomic.Pointer[publishTracker]
	cfg                          config
	js                           jetstream.JetStream
	destinations                 map[string]*destination
	namespace                    string
	maxPayload                   int64
	opened, closed, continuous   bool
	failure, uncertain, closeErr error
	lifecycle                    stream.SinkLifecycle
	receipts                     map[string]filament.EpochReceipt
}

// New returns a sink ready to open for one run.
func New() *Sink { return &Sink{} }

// Name identifies the registered sink.
func (*Sink) Name() string { return "nats" }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.StreamingSink     = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
)

// TestConnection requires only connection settings, not a pipeline stream.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	conn, err := connection.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()
	js, err := jetstream.New(conn)
	if err != nil {
		return err
	}
	_, err = js.AccountInfo(ctx)
	return err
}

// Open connects a run to an existing stream with publish acknowledgments enabled.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opened {
		return errors.New("nats sink: already opened; create a new sink for another run")
	}
	if err := run.ValidateStreamAttempt(); err != nil {
		return err
	}
	cfg := filament.NewConfig(run.Sink.Config)
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := resolve(cfg)
	if err != nil {
		return err
	}
	for _, policy := range run.WritePolicies {
		if policy.Capability.Mode != filament.WriteAppend {
			return errors.New("nats sink: only append is supported")
		}
	}
	continuous := run.Options.Execution.Normalize() == filament.ExecutionContinuous
	var lifecycle stream.SinkLifecycle
	if continuous {
		lifecycle, err = stream.NewSinkLifecycle(*run.StreamAttempt)
		if err != nil {
			return err
		}
	}
	conn, err := connection.Connect(ctx, cfg)
	if err != nil {
		return err
	}
	// Open failures must release their temporary connection.
	ready := false
	defer func() {
		if !ready {
			conn.Close()
		}
	}()
	js, err := jetstream.New(conn,
		jetstream.WithPublishAsyncMaxPending(resolved.maxInFlight),
		jetstream.WithPublishAsyncTimeout(resolved.timeout),
		jetstream.WithPublishAsyncAckHandler(s.publishAck),
		jetstream.WithPublishAsyncErrHandler(s.publishError),
	)
	if err != nil {
		return err
	}
	s.js, s.cfg, s.maxPayload = js, resolved, conn.MaxPayload()
	s.destinations = map[string]*destination{}
	// A shared stream is verified before any write, using * for the resource so
	// the check covers every subject the template can produce. Per-resource
	// streams are resolved when their first batch is routed.
	if !resolved.stream.perResource() {
		if _, err := s.destination(ctx, "*"); err != nil {
			return err
		}
	}
	namespace, _ := json.Marshal([]string{string(run.Tenant), run.PipelineID, run.SinkConnectionID})
	s.namespace = string(namespace)
	s.conn, s.publisher = conn, js
	s.continuous, s.lifecycle = continuous, lifecycle
	s.opened, ready = true, true
	return nil
}

// destination resolves and caches the stream serving a resource. The lookup
// fails when the stream is missing, cannot acknowledge, or does not capture
// the subject the template produces for that resource.
func (s *Sink) destination(ctx context.Context, resource string) (*destination, error) {
	name := s.cfg.stream.expand(resource)
	if d, ok := s.destinations[name]; ok {
		return d, nil
	}
	if err := validStreamName(name); err != nil {
		return nil, fmt.Errorf("%w for resource %q", err, resource)
	}
	lookupCtx, cancel := context.WithTimeout(ctx, s.cfg.timeout)
	defer cancel()
	jsStream, err := s.js.Stream(lookupCtx, name)
	if errors.Is(err, jetstream.ErrStreamNotFound) && s.cfg.createStream {
		jsStream, err = s.createStream(lookupCtx, name, resource)
	}
	if err != nil {
		return nil, fmt.Errorf("nats sink: stream %q lookup: %w", name, err)
	}
	info := jsStream.CachedInfo()
	if info.Config.NoAck {
		return nil, fmt.Errorf("nats sink: stream %q must enable publish acknowledgments", name)
	}
	d := &destination{name: name, filters: info.Config.Subjects, maxPayload: s.maxPayload}
	if info.Config.MaxMsgSize > 0 {
		d.maxPayload = min(d.maxPayload, int64(info.Config.MaxMsgSize))
	}
	// Open probes a shared stream with * as the resource, so check coverage of
	// the expanded pattern here; route validates the concrete subject per batch.
	if pattern := s.cfg.subject.expand(resource); !subject.Covered(d.filters, pattern) {
		return nil, fmt.Errorf("nats sink: stream %q subjects %v do not capture %q", name, d.filters, pattern)
	}
	s.destinations[name] = d
	return d, nil
}

// createStream provisions a missing destination with file storage and server
// defaults, capturing the subjects the template produces. A concurrent creator
// winning the race is fine: the stream is looked up again and verified like
// any existing one. Existing streams are never modified.
func (s *Sink) createStream(ctx context.Context, name, resource string) (jetstream.Stream, error) {
	if resource == "*" {
		// Open's shared-stream probe: the filter must cover real resources.
		resource = "resource"
	}
	cfg := jetstream.StreamConfig{Name: name, Subjects: []string{s.cfg.captureFilter(resource)}, Storage: jetstream.FileStorage}
	jsStream, err := s.js.CreateStream(ctx, cfg)
	if errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		return s.js.Stream(ctx, name)
	}
	if err != nil {
		return nil, fmt.Errorf("create: %w", err)
	}
	return jsStream, nil
}

func (s *Sink) fail(err error) error {
	if s.failure == nil {
		s.failure = err
	}
	if s.continuous {
		s.lifecycle.Fail(err)
	}
	return err
}

// Apply publishes one JSON message per row and waits for every acknowledgment.
// A failed batch can leave a subset of messages at the destination.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var empty filament.WriteReceipt
	if !s.opened || s.closed {
		return empty, errors.New("nats sink: session is not open")
	}
	if s.failure != nil {
		return empty, s.failure
	}
	if s.continuous {
		if err := s.lifecycle.CheckApply(opts.Epoch); err != nil {
			return empty, err
		}
	} else if opts.Epoch != nil {
		return empty, filament.ErrEpochMismatch
	}
	if err := ctx.Err(); err != nil {
		return empty, s.fail(err)
	}
	if opts.Policy.Capability.Mode != filament.WriteAppend {
		return empty, s.fail(errors.New("nats sink: only append is supported"))
	}
	if b == nil || b.Rows() == nil {
		return empty, s.fail(errors.New("nats sink: a record batch is required"))
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		return empty, s.fail(err)
	}
	// encode hashes the exact payload bytes handed to the SDK; the same slices
	// are published unchanged, so that CRC is the transport-boundary evidence.
	messages, encoded, err := s.encode(ctx, b)
	if err != nil {
		return empty, s.fail(err)
	}
	if err := s.publishMessages(ctx, messages); err != nil {
		return empty, s.fail(err)
	}
	encodedCRC := encoded.crc
	receipt := filament.WriteReceipt{URI: "nats://" + encoded.stream + "/" + b.Resource, Rows: b.NumRows(), Bytes: encoded.bytes, WriteCRC: b.IntegrityCRC(), EncodedCRC: &encodedCRC}
	if s.continuous {
		total := s.receipts[b.Resource]
		total.Resource = b.Resource
		total.Rows += int64(receipt.Rows)
		total.Bytes += receipt.Bytes
		s.receipts[b.Resource] = total
	}
	return receipt, nil
}

// Commit finalizes a bounded run. Every successful Apply is already durable.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.continuous {
		return filament.ErrEpochMismatch
	}
	if !s.opened {
		return errors.New("nats sink: session is not open")
	}
	if s.closed {
		return s.closeErr
	}
	return s.close(ctx)
}

// Abort stops a bounded run without deleting published messages. A failure
// already returned by Apply is not reported again; only cleanup errors are.
func (s *Sink) Abort(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.continuous {
		return filament.ErrEpochMismatch
	}
	if !s.opened || s.closed {
		return nil
	}
	_ = s.close(ctx) // Abort reports only cleanup errors, not prior publication failures.
	return ctx.Err()
}

func (s *Sink) close(ctx context.Context) error {
	s.cleanupPublisher()
	s.closed = true
	s.closeErr = errors.Join(s.failure, s.uncertain, ctx.Err())
	return s.closeErr
}

// cleanupPublisher releases SDK futures and reply subscriptions. This is local
// cleanup only; it cannot prove destination quiescence after a lost acknowledgment.
func (s *Sink) cleanupPublisher() {
	if s.conn != nil {
		s.conn.Close()
	}
	if s.publisher != nil {
		s.publisher.CleanupPublisher()
	}
}
