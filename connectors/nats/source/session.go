package source

import (
	"context"
	"errors"
	"fmt"
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
	certifiedAck             uint64
	initialFloor             uint64
	codecs                   *streamkit.Registry
	writer                   arrowbatch.RowWriter
	projector                *streamkit.Projector
	columns                  *streamkit.MessageColumns
	hb                       *streamkit.Heartbeat
	mu                       sync.Mutex
	pending                  []jetstream.Msg
	prefetched               jetstream.Msg
	maxPending               int
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
	if err := validateConsumerProgress(ci, max(s.committed, s.certifiedAck), s.initialFloor); err != nil {
		return err
	}
	if !si.Created.Equal(s.created) || !ci.Created.Equal(s.consumerCreated) {
		return filament.ErrPositionIncomparable
	}
	return nil
}

func (s *session) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (filament.Coverage, error) {
	return s.readBatch(ctx, out, b, nil)
}

// readBatch owns one coverage containing up to the consumer's pending credit.
// The optional first message was prefetched by multiSession on this same owner.
func (s *session) readBatch(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary, first jetstream.Msg) (coverage filament.Coverage, result error) {
	if err := s.lifecycle.CheckRead(ctx); err != nil {
		return coverage, err
	}
	if b.MaxWait <= 0 || b.MaxRecords <= 0 {
		return coverage, errors.New("nats: positive MaxWait and MaxRecords required")
	}
	defer func() {
		if result != nil {
			s.failed = result
			s.lifecycle.Fail(result)
		}
	}()
	wait := b.MaxWait
	if b.MaxAge > 0 && b.MaxAge < wait {
		wait = b.MaxAge
	}
	readCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	stop := context.AfterFunc(s.hb.Context(), cancel)
	defer stop()
	limit := min(b.MaxRecords, max(1, s.maxPending))
	var nbytes int64
	for count := 0; count < limit; {
		msg := first
		first = nil
		if msg == nil {
			var err error
			msg, err = s.consumer.Next(jetstream.FetchContext(readCtx))
			if err != nil {
				if ctx.Err() != nil {
					return filament.Coverage{}, context.Cause(ctx)
				}
				if s.hb.Err() != nil {
					return filament.Coverage{}, s.hb.Err()
				}
				if idleFetch(err) {
					break
				}
				return filament.Coverage{}, err
			}
		}
		next, err := s.readMessage(ctx, out, msg)
		if err != nil {
			return filament.Coverage{}, err
		}
		if len(next.Positions) == 0 {
			continue
		}
		coverage = next
		count++
		nbytes += int64(len(msg.Data()))
		if b.MaxBytes > 0 && nbytes >= b.MaxBytes {
			break
		}
	}
	if err := s.lifecycle.MarkRead(coverage); err != nil {
		return filament.Coverage{}, err
	}
	return coverage, nil
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
	// A batch may receive a redelivery before certification. Do not emit it twice.
	if seq <= s.pendingSeq && len(s.pending) > 0 {
		return filament.Coverage{}, nil
	}
	previous := s.committed
	if len(s.pending) > 0 {
		previous = s.pendingSeq
	}
	// Explicit unfiltered consumers cover every stream sequence. Filtered
	// consumers cover retained matching messages; unrelated sequences and loss
	// to retention cannot be distinguished from stream-wide metadata.
	if !s.binding.managed && seq != previous+1 {
		return filament.Coverage{}, errors.New("nats: source sequence gap; retention or another consumer owner changed progress")
	}
	if s.writer == nil {
		columns, schema, err := streamkit.NewMessageColumns(messageBaseSchema(s.binding.resource), msg.Data())
		if err != nil {
			return filament.Coverage{}, err
		}
		s.writer, err = out.Builder(s.binding.resource, 0, schema)
		if err != nil {
			return filament.Coverage{}, err
		}
		s.projector = streamkit.NewEventMetadataProjector(s.writer, s.codecs)
		s.columns = columns
	}
	appendPayload, err := s.columns.Prepare(msg.Data())
	if err != nil {
		return filament.Coverage{}, fmt.Errorf("nats: subject %s sequence %d: %w", msg.Subject(), seq, err)
	}
	s.mu.Lock()
	s.pending = append(s.pending, msg)
	s.prefetched = nil
	s.pendingSeq = seq
	s.mu.Unlock()
	headers := messageHeaders(msg.Headers())
	position := filament.Position{Codec: PositionCodec, Version: 0, Value: []byte(strconv.FormatUint(seq, 10))}
	s.writer.String(msg.Subject())
	appendPayload(s.writer)
	if err := s.projector.EndEvent(streamkit.Envelope{Identity: filament.EventIdentity{Domain: s.domain, Position: position}, Timestamp: &meta.Timestamp, Headers: headers, KeyNull: true, Payload: msg.Data()}, rowmodel.Meta{}); err != nil {
		s.lifecycle.Fail(err)
		return filament.Coverage{}, err
	}
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: s.domain, Position: position}); err != nil {
		s.lifecycle.Fail(err)
		return filament.Coverage{}, err
	}
	coverage = filament.Coverage{Positions: filament.DomainPositions{s.domain: position}}
	return coverage, nil
}

func (s *session) Acknowledge(ctx context.Context, coverage filament.Coverage) error {
	if err := s.lifecycle.CheckAcknowledge(ctx, coverage); err != nil {
		return err
	}
	if len(s.pending) == 0 {
		return filament.ErrIncompleteCoverage
	}
	// A filtered consumer's ack floor can advance over unrelated sequences
	// between pending messages. After certification that advance is safe up to
	// this batch's exact coverage, including when acknowledgements must retry.
	s.certifiedAck = s.pendingSeq
	// Do not hold the heartbeat lock across network acknowledgement calls.
	for len(s.pending) > 0 {
		msg := s.pending[0]
		meta, err := msg.Metadata()
		if err != nil {
			return err
		}
		if err := msg.DoubleAck(ctx); err != nil {
			return err
		}
		// Advance confirmed acknowledgement progress even if a later ack fails.
		s.committed = meta.Sequence.Stream
		s.mu.Lock()
		s.pending[0] = nil
		s.pending = s.pending[1:]
		s.mu.Unlock()
	}
	s.mu.Lock()
	s.pending = nil
	s.mu.Unlock()
	s.certifiedAck = 0
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
