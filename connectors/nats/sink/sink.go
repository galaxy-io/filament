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
	checkCtx, cancel := context.WithTimeout(ctx, resolved.timeout)
	defer cancel()
	destination, err := js.Stream(checkCtx, resolved.stream)
	if err != nil {
		return fmt.Errorf("nats sink: stream lookup: %w", err)
	}
	info := destination.CachedInfo()
	if info.Config.NoAck {
		return errors.New("nats sink: destination stream must enable publish acknowledgments")
	}
	namespace, _ := json.Marshal([]string{string(run.Tenant), run.PipelineID, run.SinkConnectionID})
	s.namespace = string(namespace)
	s.maxPayload = conn.MaxPayload()
	if info.Config.MaxMsgSize > 0 {
		s.maxPayload = min(s.maxPayload, int64(info.Config.MaxMsgSize))
	}
	s.conn, s.publisher, s.cfg = conn, js, resolved
	s.continuous, s.lifecycle = continuous, lifecycle
	s.opened, ready = true, true
	return nil
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
	messages, encoded, err := s.encode(b)
	if err != nil {
		return empty, s.fail(err)
	}
	if err := s.publishMessages(ctx, messages); err != nil {
		return empty, s.fail(err)
	}
	encodedCRC := encoded.crc
	receipt := filament.WriteReceipt{URI: "nats://" + s.cfg.stream + "/" + b.Resource, Rows: b.NumRows(), Bytes: encoded.bytes, WriteCRC: b.IntegrityCRC(), EncodedCRC: &encodedCRC}
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
	s.close(ctx)
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
