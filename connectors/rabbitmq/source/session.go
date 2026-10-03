package source

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	rmq "github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	istream "github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

type delivery struct {
	stream string
	offset int64
	msg *amqp.Message
}

type streamState struct {
	domain filament.DomainKey
	next int64
	initial bool
	consumer *rmq.Consumer
	deliveries chan delivery
}

type session struct {
	lifecycle istream.SourceLifecycle
	streams map[string]*streamState
	writers map[string]arrowbatch.RowWriter
	columns map[string]*streamkit.MessageColumns
	projectors map[string]*streamkit.Projector
	codecs filament.CodecResolver
	closeOnce sync.Once
	closeErr error
}

func selectedStreams(resources, defaults []string) ([]string, error) {
	if len(resources) == 0 {
		resources = defaults
	}
	out := slices.Clone(resources)
	slices.Sort(out)
	if len(out) == 0 {
		return nil, errors.New("rabbitmq: select at least one stream")
	}
	for i, name := range out {
		if name == "" {
			return nil, errors.New("rabbitmq: stream name cannot be empty")
		}
		if i > 0 && out[i-1] == name {
			return nil, fmt.Errorf("rabbitmq: duplicate stream %q", name)
		}
	}
	return out, nil
}

func streamDomain(identity, name string) filament.DomainKey {
	domain, _ := json.Marshal(name)
	incarnation, _ := json.Marshal([]string{identity, name})
	return filament.DomainKey{Domain: string(domain), Incarnation: string(incarnation)}
}

// OpenStream creates one offset-addressed consumer per selected RabbitMQ stream.
// Broker-side consumer offsets are deliberately unused: Filament certificates
// are the only durable progress authority.
func (s *Source) OpenStream(ctx context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	if s.env == nil || opts.SourceConnectionID == "" || opts.CheckAuthority == nil {
		return nil, errors.New("rabbitmq: configured source, stable identity and authority required")
	}
	if err := opts.Attempt.Validate(); err != nil {
		return nil, err
	}
	if err := opts.CheckAuthority(ctx); err != nil {
		return nil, err
	}
	names, err := selectedStreams(opts.Resources, s.defaultStreams)
	if err != nil {
		return nil, err
	}
	ss := &session{streams: map[string]*streamState{}, writers: map[string]arrowbatch.RowWriter{}, columns: map[string]*streamkit.MessageColumns{}, projectors: map[string]*streamkit.Projector{}, codecs: s}
	known := map[filament.DomainKey]bool{}
	for _, name := range names {
		domain := streamDomain(opts.SourceConnectionID, name)
		known[domain] = true
		next := int64(0)
		initial := true
		if p, ok := opts.CommittedPositions[domain]; ok {
			next, err = offset(p)
			if err != nil {
				return nil, err
			}
			initial = false
		}
		state := &streamState{domain: domain, next: next, initial: initial, deliveries: make(chan delivery, 16)}
		handler := func(cc rmq.ConsumerContext, msg *amqp.Message) {
			d := delivery{stream: name, offset: cc.Consumer.GetOffset(), msg: msg}
			select {
			case state.deliveries <- d:
			default:
				// InitialCredits bounds normal delivery. A full queue means the
				// session is no longer safe to advance; Close will force replay.
			}
		}
		consumerOpts := rmq.NewConsumerOptions().SetManualCommit().SetInitialCredits(1)
		if initial {
			consumerOpts.SetOffset(rmq.OffsetSpecification{}.First())
		} else {
			consumerOpts.SetOffset(rmq.OffsetSpecification{}.Offset(next))
		}
		state.consumer, err = s.env.NewConsumer(name, handler, consumerOpts)
		if err != nil {
			for _, opened := range ss.streams {
				_ = opened.consumer.Close()
			}
			return nil, fmt.Errorf("rabbitmq: open stream %s: %w", name, err)
		}
		ss.streams[name] = state
	}
	for d := range opts.CommittedPositions {
		if !known[d] {
			for _, opened := range ss.streams {
				_ = opened.consumer.Close()
			}
			return nil, filament.ErrPositionIncomparable
		}
	}
	ss.lifecycle, err = istream.NewSourceLifecycle(opts.Attempt, opts.CheckAuthority)
	if err != nil {
		for _, opened := range ss.streams {
			_ = opened.consumer.Close()
		}
		return nil, err
	}
	return ss, nil
}

func (s *session) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (coverage filament.Coverage, result error) {
	if err := s.lifecycle.CheckRead(ctx); err != nil {
		return coverage, err
	}
	if b.MaxWait <= 0 {
		return coverage, errors.New("rabbitmq: positive MaxWait required")
	}
	defer func() {
		if result != nil {
			s.lifecycle.Fail(result)
		}
	}()
	names := make([]string, 0, len(s.streams))
	for name := range s.streams { names = append(names, name) }
	slices.Sort(names)
	for _, name := range names {
		st := s.streams[name]
		if st.initial {
			p := position(st.next)
			if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: st.domain, Position: p}); err != nil {
				return coverage, err
			}
			coverage.Positions = filament.DomainPositions{st.domain: p}
			return coverage, s.lifecycle.MarkRead(coverage)
		}
	}
	timer := time.NewTimer(b.MaxWait)
	defer timer.Stop()
	for {
		for _, name := range names {
			select {
			case d := <-s.streams[name].deliveries:
				return s.readDelivery(ctx, out, d)
			default:
			}
		}
		select {
		case <-ctx.Done():
			return coverage, ctx.Err()
		case <-timer.C:
			return coverage, nil
		case <-time.After(time.Millisecond):
		}
	}
}

func (s *session) readDelivery(ctx context.Context, out filament.StreamRecordSink, d delivery) (coverage filament.Coverage, result error) {
	st := s.streams[d.stream]
	if st == nil || d.offset < st.next {
		return coverage, errors.New("rabbitmq: invalid delivery position")
	}
	payload := d.msg.GetData()
	writer := s.writers[d.stream]
	if writer == nil {
		columns, schema, err := streamkit.NewMessageColumns(messageBaseSchema(d.stream), payload)
		if err != nil { return coverage, err }
		writer, err = out.Builder(d.stream, 0, schema)
		if err != nil { return coverage, err }
		s.writers[d.stream] = writer
		s.columns[d.stream] = columns
		s.projectors[d.stream] = streamkit.NewEventMetadataProjector(writer, s.codecs)
	}
	appendPayload, err := s.columns[d.stream].Prepare(payload)
	if err != nil { return coverage, err }
	writer.String(d.stream)
	writer.Int64(d.offset)
	appendPayload(writer)
	next := position(d.offset + 1)
	envelope := streamkit.Envelope{Identity: filament.EventIdentity{Domain: st.domain, Position: next}, Payload: payload, PayloadNull: payload == nil}
	if err := s.projectors[d.stream].EndEvent(envelope, rowmodel.Meta{}); err != nil { return coverage, err }
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: st.domain, Position: next}); err != nil { return coverage, err }
	coverage.Positions = filament.DomainPositions{st.domain: next}
	return coverage, s.lifecycle.MarkRead(coverage)
}

func (s *session) Acknowledge(ctx context.Context, c filament.Coverage) error {
	if err := s.lifecycle.CheckAcknowledge(ctx, c); err != nil {
		return err
	}
	for _, st := range s.streams {
		if p, ok := c.Positions[st.domain]; ok {
			n, err := offset(p)
			if err != nil { return err }
			st.next, st.initial = n, false
		}
	}
	return s.lifecycle.MarkAcknowledged(c)
}

func (s *session) Close(ctx context.Context) error {
	s.closeOnce.Do(func() {
		for _, st := range s.streams {
			if err := st.consumer.Close(); err != nil && s.closeErr == nil {
				s.closeErr = err
			}
		}
		s.lifecycle.MarkClosed(s.closeErr)
	})
	if err := ctx.Err(); err != nil { return err }
	return s.closeErr
}
