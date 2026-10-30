package source

import (
	"context"
	"errors"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/streamkit"
)

func (s *session) readSchemaBatch(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary, first jetstream.Msg) (coverage filament.Coverage, result error) {
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
	messages, err := s.collectSchemaBatch(ctx, b, first)
	if err != nil {
		return coverage, err
	}
	if len(messages) == 0 {
		return coverage, nil
	}
	payloads := make([][]byte, len(messages))
	for i, msg := range messages {
		payloads[i] = msg.Data()
	}
	columns, writer, err := s.live.Prepare(ctx, out, messageBaseSchema(s.binding.resource), payloads)
	if err != nil {
		return coverage, err
	}
	s.writer, s.columns = writer, columns
	s.projector = streamkit.NewEventMetadataProjector(writer, s.codecs)

	for _, msg := range messages {
		next, err := s.readMessage(ctx, out, msg)
		if err != nil {
			return filament.Coverage{}, err
		}
		if len(next.Positions) > 0 {
			coverage = next
		}
	}
	s.mu.Lock()
	s.buffered = nil
	s.mu.Unlock()
	if err := s.lifecycle.MarkRead(coverage); err != nil {
		return filament.Coverage{}, err
	}
	return coverage, nil
}

// Buffer at most one epoch within the consumer's delivery credit. Heartbeats
// cover buffered messages while probing and waiting for destination DDL locks.
// No tentative input is emitted or acknowledged until probing succeeds.
func (s *session) collectSchemaBatch(ctx context.Context, b filament.Boundary, first jetstream.Msg) ([]jetstream.Msg, error) {
	wait := b.MaxWait
	if b.MaxAge > 0 && b.MaxAge < wait {
		wait = b.MaxAge
	}
	readCtx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()
	stop := context.AfterFunc(s.hb.Context(), cancel)
	defer stop()
	limit := min(b.MaxRecords, max(1, s.maxPending))
	messages := make([]jetstream.Msg, 0, limit)
	previous := s.committed
	var nbytes int64
	for len(messages) < limit {
		msg := first
		first = nil
		if msg == nil {
			var err error
			msg, err = s.consumer.Next(jetstream.FetchContext(readCtx))
			if err != nil {
				if ctx.Err() != nil {
					return nil, context.Cause(ctx)
				}
				if s.hb.Err() != nil {
					return nil, s.hb.Err()
				}
				if idleFetch(err) {
					break
				}
				return nil, err
			}
		}
		meta, err := msg.Metadata()
		if err != nil {
			return nil, err
		}
		seq := meta.Sequence.Stream
		if seq <= s.committed {
			if err := s.lifecycle.CheckAuthority(ctx); err != nil {
				return nil, err
			}
			if err := msg.DoubleAck(ctx); err != nil {
				return nil, err
			}
			continue
		}
		if seq <= previous {
			continue
		} // buffered redelivery
		if !s.binding.managed && seq != previous+1 {
			return nil, errors.New("nats: source sequence gap during schema probe")
		}
		previous = seq
		messages = append(messages, msg)
		s.mu.Lock()
		s.buffered = append(s.buffered, msg)
		s.prefetched = nil
		s.mu.Unlock()
		nbytes += int64(len(msg.Data()))
		if b.MaxBytes > 0 && nbytes >= b.MaxBytes {
			break
		}
	}
	return messages, nil
}
