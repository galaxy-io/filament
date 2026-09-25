package source

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

type session struct {
	lifecycle                stream.SourceLifecycle
	stream                   jetstream.Stream
	consumer                 jetstream.Consumer
	binding                  consumerBinding
	domain                   filament.DomainKey
	created, consumerCreated time.Time
	committed                uint64
	initialFloor             uint64
	scanFloor                uint64
	codecs                   *streamkit.Registry
	writer                   arrowbatch.RowWriter
	projector                *streamkit.Projector
	hb                       *streamkit.Heartbeat
	mu                       sync.Mutex
	pending                  jetstream.Msg
	pendingSeq               uint64
	failed                   error
	closed                   bool
	closeErr                 error
}

func (s *session) authority(ctx context.Context) error {
	if s.closed {
		return errors.New("nats: session closed")
	}
	if s.failed != nil {
		return s.failed
	}
	if err := context.Cause(s.hb.Context()); err != nil {
		return err
	}
	si, err := s.stream.Info(ctx)
	if err != nil {
		return err
	}
	ci, err := s.consumer.Info(ctx)
	if err != nil {
		return err
	}
	if err := s.binding.validateConsumer(ci.Config); err != nil {
		return err
	}
	if err := validateConsumerProgress(ci, s.committed, s.initialFloor); err != nil {
		return err
	}
	if !si.Created.Equal(s.created) || !ci.Created.Equal(s.consumerCreated) {
		return filament.ErrPositionIncomparable
	}
	return nil
}

func (s *session) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (filament.Coverage, error) {
	if err := s.lifecycle.CheckRead(ctx); err != nil {
		return filament.Coverage{}, err
	}
	s.mu.Lock()
	pending := s.pending != nil
	s.mu.Unlock()
	if pending {
		return filament.Coverage{}, errors.New("nats: acknowledge certified epoch before reading again")
	}
	wait := b.MaxWait
	if wait <= 0 {
		return filament.Coverage{}, errors.New("nats: positive MaxWait required")
	}
	readCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	stop := context.AfterFunc(s.hb.Context(), cancel)
	defer stop()
	msg, err := s.consumer.Next(jetstream.FetchContext(readCtx))
	if err != nil {
		if ctx.Err() != nil {
			return filament.Coverage{}, context.Cause(ctx)
		}
		if s.hb.Err() != nil {
			return filament.Coverage{}, s.hb.Err()
		}
		if idleFetch(err) {
			return filament.Coverage{}, nil
		}
		return filament.Coverage{}, err
	}
	return s.readMessage(ctx, out, msg)
}

// idleFetch reports a pull that expired without a message.
func idleFetch(err error) bool {
	return errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
}

// readMessage is called only on the owner goroutine; fetchers never touch builders.
func (s *session) readMessage(ctx context.Context, out filament.StreamRecordSink, msg jetstream.Msg) (coverage filament.Coverage, result error) {
	defer func() {
		if result != nil {
			s.failed = result
		}
	}()
	if err := s.authority(ctx); err != nil {
		return filament.Coverage{}, err
	}
	if s.writer == nil {
		schema, err := Schema(s.binding.resource)
		if err != nil {
			return filament.Coverage{}, err
		}
		s.writer, err = out.Builder(s.binding.resource, 0, schema)
		if err != nil {
			return filament.Coverage{}, err
		}
		s.projector = streamkit.NewProjector(s.writer, s.codecs)
	}
	meta, err := msg.Metadata()
	if err != nil {
		return filament.Coverage{}, err
	}
	seq := meta.Sequence.Stream
	if seq <= s.committed {
		if err := s.lifecycle.CheckAuthority(ctx); err != nil {
			return filament.Coverage{}, err
		}
		return filament.Coverage{}, msg.DoubleAck(ctx)
	}
	// Whole-stream, single-credit delivery cannot safely skip missing input.
	if !s.binding.managed && seq != s.committed+1 {
		return filament.Coverage{}, errors.New("nats: source sequence gap; retention or another consumer owner changed progress")
	}
	if s.binding.managed && seq > s.scanFloor && seq-s.scanFloor > 1 {
		info, err := s.stream.Info(ctx, jetstream.WithDeletedDetails(true))
		if err != nil {
			return filament.Coverage{}, err
		}
		if info.State.FirstSeq > s.scanFloor+1 {
			return filament.Coverage{}, errors.New("nats: retained input no longer covers subject progress")
		}
		for _, deleted := range info.State.Deleted {
			if deleted > s.scanFloor && deleted < seq {
				return filament.Coverage{}, errors.New("nats: deleted input makes subject progress unprovable")
			}
		}
	}
	s.mu.Lock()
	s.pending = msg
	s.pendingSeq = seq
	s.mu.Unlock()
	headers := messageHeaders(msg.Headers())
	position := filament.Position{Codec: PositionCodec, Version: 0, Value: []byte(strconv.FormatUint(seq, 10))}
	s.writer.String(msg.Subject())
	if err := s.projector.EndEvent(streamkit.Envelope{Identity: filament.EventIdentity{Domain: s.domain, Position: position}, Timestamp: &meta.Timestamp, Headers: headers, KeyNull: true, Payload: msg.Data()}, rowmodel.Meta{}); err != nil {
		s.lifecycle.Fail(err)
		return filament.Coverage{}, err
	}
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: s.domain, Position: position}); err != nil {
		s.lifecycle.Fail(err)
		return filament.Coverage{}, err
	}
	coverage = filament.Coverage{Positions: filament.DomainPositions{s.domain: position}}
	if err := s.lifecycle.MarkRead(coverage); err != nil {
		s.lifecycle.Fail(err)
		return filament.Coverage{}, err
	}
	return coverage, nil
}

func (s *session) Acknowledge(ctx context.Context, coverage filament.Coverage) error {
	if err := s.lifecycle.CheckAcknowledge(ctx, coverage); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		return filament.ErrIncompleteCoverage
	}
	if err := s.pending.DoubleAck(ctx); err != nil {
		return err
	}
	s.committed = s.pendingSeq
	s.scanFloor = s.pendingSeq
	s.pending = nil
	return s.lifecycle.MarkAcknowledged(coverage)
}

func (s *session) Close(ctx context.Context) error {
	if s.closed {
		return s.closeErr
	}
	err := s.hb.Stop(ctx)
	select {
	case <-s.hb.Done():
	default:
		s.failed = err
		return err
	}
	// Consumer handles hold no client resources; the pull expires server-side.
	s.closed, s.closeErr = true, err
	return err
}
