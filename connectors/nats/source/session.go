package source

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

type session struct {
	checkAuthority           func(context.Context) error
	source                   *Source
	sub                      *nats.Subscription
	domain                   filament.DomainKey
	created, consumerCreated time.Time
	committed                uint64
	codecs                   *streamkit.Registry
	writer                   arrowbatch.RowWriter
	projector                *streamkit.Projector
	hb                       *streamkit.Heartbeat
	mu                       sync.Mutex
	pending                  *nats.Msg
	pendingSeq               uint64
}

func (s *session) authority(ctx context.Context) error {
	if err := context.Cause(s.hb.Context()); err != nil {
		return err
	}
	si, err := s.source.js.StreamInfo(s.source.stream, nats.Context(ctx))
	if err != nil {
		return err
	}
	ci, err := s.source.js.ConsumerInfo(s.source.stream, s.source.consumer, nats.Context(ctx))
	if err != nil {
		return err
	}
	if err := validateConsumer(ci.Config, s.source.consumer); err != nil {
		return err
	}
	if !si.Created.Equal(s.created) || !ci.Created.Equal(s.consumerCreated) {
		return filament.ErrPositionIncomparable
	}
	return s.checkAuthority(ctx)
}

func (s *session) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (filament.Coverage, error) {
	if err := s.authority(ctx); err != nil {
		return filament.Coverage{}, err
	}
	s.mu.Lock()
	pending := s.pending != nil
	s.mu.Unlock()
	if pending {
		return filament.Coverage{}, errors.New("nats: acknowledge certified epoch before reading again")
	}
	if s.writer == nil {
		schema, err := Schema(s.source.stream)
		if err != nil {
			return filament.Coverage{}, err
		}
		s.writer, err = out.Builder(s.source.stream, 0, schema)
		if err != nil {
			return filament.Coverage{}, err
		}
		s.projector = streamkit.NewProjector(s.writer, s.codecs)
	}
	wait := b.MaxWait
	if wait <= 0 {
		return filament.Coverage{}, errors.New("nats: positive MaxWait required")
	}
	readCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	stop := context.AfterFunc(s.hb.Context(), cancel)
	defer stop()
	messages, err := s.sub.Fetch(1, nats.Context(readCtx))
	if err != nil {
		if ctx.Err() != nil {
			return filament.Coverage{}, context.Cause(ctx)
		}
		if s.hb.Err() != nil {
			return filament.Coverage{}, s.hb.Err()
		}
		if errors.Is(err, nats.ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
			return filament.Coverage{}, nil
		}
		return filament.Coverage{}, err
	}
	msg := messages[0]
	meta, err := msg.Metadata()
	if err != nil {
		return filament.Coverage{}, err
	}
	seq := meta.Sequence.Stream
	if seq <= s.committed {
		if err := s.authority(ctx); err != nil {
			return filament.Coverage{}, err
		}
		return filament.Coverage{}, msg.AckSync(nats.Context(ctx))
	}
	// Whole-stream, single-credit delivery cannot safely skip missing input.
	if seq != s.committed+1 {
		return filament.Coverage{}, errors.New("nats: source sequence gap; retention or another consumer owner changed progress")
	}
	s.mu.Lock()
	s.pending = msg
	s.pendingSeq = seq
	s.mu.Unlock()
	headers := messageHeaders(msg)
	position := filament.Position{Codec: PositionCodec, Version: 0, Value: []byte(strconv.FormatUint(seq, 10))}
	s.writer.String(msg.Subject)
	if err := s.projector.EndEvent(streamkit.Envelope{Identity: filament.EventIdentity{Domain: s.domain, Position: position}, Timestamp: &meta.Timestamp, Headers: headers, KeyNull: true, Payload: msg.Data}, rowmodel.Meta{}); err != nil {
		return filament.Coverage{}, err
	}
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: s.domain, Position: position}); err != nil {
		return filament.Coverage{}, err
	}
	return filament.Coverage{Positions: filament.DomainPositions{s.domain: position}}, nil
}

func (s *session) Acknowledge(ctx context.Context, coverage filament.Coverage) error {
	if err := s.authority(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(coverage.Claims) > 0 || len(coverage.Positions) != 1 || s.pending == nil {
		return filament.ErrIncompleteCoverage
	}
	p, ok := coverage.Positions[s.domain]
	if !ok || p.Codec != PositionCodec || p.Version != 0 || string(p.Value) != strconv.FormatUint(s.pendingSeq, 10) {
		return filament.ErrIncompleteCoverage
	}
	if err := s.pending.AckSync(nats.Context(ctx)); err != nil {
		return err
	}
	s.committed = s.pendingSeq
	s.pending = nil
	return nil
}

func (s *session) Close(ctx context.Context) error {
	err := s.hb.Stop(ctx)
	if err != nil {
		return err
	}
	return s.sub.Unsubscribe()
}
