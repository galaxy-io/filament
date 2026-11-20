package sink

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/rabbitmq/internal/connection"
	"github.com/galaxy-io/filament/internal/sinkstate"
)

// minProducerQueue is the client's minimum internal queue. Outstanding
// publishes are still bounded by max_in_flight.
const minProducerQueue = 500

type destination struct {
	name         string
	producer     *stream.Producer
	publications atomic.Pointer[publishTracker]
}

// Sink serializes Apply calls and never retains borrowed Arrow batches.
// Acknowledged messages survive aborts. A lost confirmation prevents reporting
// a quiescent streaming session.
type Sink struct {
	mu           sync.Mutex
	env          *stream.Environment
	cfg          config
	destinations map[string]*destination
	tenant       string
	pipeline     string
	connection   string
	uncertain    error
	lifecycle    sinkstate.Lifecycle
}

// New returns a sink ready to open for one run.
func New() *Sink { return &Sink{} }

// Name identifies the registered sink.
func (*Sink) Name() string { return "rabbitmq" }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
)

// TestConnection dials the Stream URI. It does not require a pipeline stream.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	env, err := connection.Dial(ctx, connection.URI(cfg))
	if err != nil {
		return err
	}
	return env.Close()
}

// Open connects a run. A fixed stream must already exist. Per-resource streams
// are checked when their first batch is routed.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lifecycle.Opened() {
		return errors.New("rabbitmq sink: already opened; create a new sink for another run")
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
			return errors.New("rabbitmq sink: only append is supported")
		}
	}
	env, err := connection.Dial(ctx, connection.URI(cfg))
	if err != nil {
		return err
	}
	ready := false
	defer func() {
		if !ready {
			_ = env.Close()
		}
	}()
	s.env, s.cfg = env, resolved
	s.destinations = map[string]*destination{}
	s.tenant, s.pipeline, s.connection = string(run.Tenant), run.PipelineID, run.SinkConnectionID
	if !resolved.stream.perResource() {
		if _, err := s.destination(ctx, "resource"); err != nil {
			return err
		}
	}
	s.lifecycle.Open()
	ready = true
	return nil
}

func (s *Sink) destination(ctx context.Context, resource string) (*destination, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name := s.cfg.stream.expand(resource)
	if d, ok := s.destinations[name]; ok {
		return d, nil
	}
	if err := validStreamName(name); err != nil {
		return nil, fmt.Errorf("%w for resource %q", err, resource)
	}
	exists, err := s.env.StreamExists(name)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq sink: stream %q lookup: %w", name, err)
	}
	if !exists {
		return nil, fmt.Errorf("rabbitmq sink: stream %q does not exist", name)
	}
	queue := s.cfg.maxInFlight
	if queue < minProducerQueue {
		queue = minProducerQueue
	}
	opts := stream.NewProducerOptions().
		SetProducerName(producerName(s.tenant, s.pipeline, s.connection, name)).
		SetQueueSize(queue).
		SetBatchSize(1).
		SetSubEntrySize(1).
		SetConfirmationTimeOut(s.cfg.timeout).
		SetClientProvidedName("filament")
	producer, err := s.env.NewProducer(name, opts)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq sink: producer for stream %q: %w", name, err)
	}
	d := &destination{name: name, producer: producer}
	go d.readConfirms(producer.NotifyPublishConfirmation())
	s.destinations[name] = d
	return d, nil
}

func (s *Sink) fail(err error) error {
	s.lifecycle.Fail(err)
	return err
}

// Apply publishes one JSON message per row and waits for every confirmation.
// A failed batch can leave a subset of messages at the destination.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var empty filament.WriteReceipt
	if err := s.lifecycle.CheckApply(); err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, s.fail(err)
	}
	if opts.Policy.Capability.Mode != filament.WriteAppend {
		return empty, s.fail(errors.New("rabbitmq sink: only append is supported"))
	}
	if b == nil || b.Rows() == nil {
		return empty, s.fail(errors.New("rabbitmq sink: a record batch is required"))
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		return empty, s.fail(err)
	}
	prepared, err := s.encode(ctx, b)
	if err != nil {
		return empty, s.fail(err)
	}
	if len(prepared.messages) > 0 {
		d := s.destinations[prepared.encoded.stream]
		if d == nil {
			return empty, s.fail(errors.New("rabbitmq sink: destination missing"))
		}
		if err := s.publishMessages(ctx, d, prepared.messages, prepared.sizes); err != nil {
			return empty, s.fail(err)
		}
	}
	encodedCRC := prepared.encoded.crc
	return filament.WriteReceipt{URI: "rabbitmq-stream://" + prepared.encoded.stream + "/" + b.Resource, Rows: b.NumRows(), Bytes: prepared.encoded.bytes, WriteCRC: b.IntegrityCRC(), EncodedCRC: &encodedCRC}, nil
}

func (s *Sink) cleanup() {
	for _, d := range s.destinations {
		if d.producer != nil {
			_ = d.producer.Close()
		}
	}
	if s.env != nil {
		_ = s.env.Close()
		s.env = nil
	}
}
