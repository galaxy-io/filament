package sink

import (
	"context"
	"errors"

	"github.com/twmb/franz-go/pkg/kgo"
)

type completion struct {
	size int64
	err  error
}

// publish owns a bounded callback channel. Even after cancellation every
// outstanding callback can finish without touching the Sink or a later batch.
func (s *Sink) publish(ctx context.Context, records []*kgo.Record) error {
	done := make(chan completion, s.cfg.maxInFlight)
	pending := 0
	var bytes int64
	consume := func(c completion) error { pending--; bytes -= c.size; return c.err }
	fail := func(err error) error {
		// Conservatively classify submitted errors as uncertain, including errors
		// that may be local. A later successful callback cannot erase uncertainty.
		s.uncertain = errors.Join(errors.New("kafka sink: destination quiescence is unproven"), err)
		_ = s.cleanupPublisher(ctx)
		return err
	}
	for _, r := range records {
		size := recordSize(r)
		// Observe failures before submitting more, even when credits remain.
		select {
		case c := <-done:
			if err := consume(c); err != nil {
				return fail(err)
			}
		default:
		}
		for pending >= s.cfg.maxInFlight || bytes+size > s.cfg.maxInFlightBytes {
			select {
			case c := <-done:
				if err := consume(c); err != nil {
					return fail(err)
				}
			case <-ctx.Done():
				return fail(ctx.Err())
			}
		}
		if err := ctx.Err(); err != nil {
			if pending > 0 {
				return fail(err)
			}
			return err
		}
		pending++
		bytes += size
		// The client frees buffer slots after callbacks return. Our credit can
		// arrive first, so wait for client capacity with the publication context.
		s.producer.Produce(ctx, r, func(_ *kgo.Record, err error) { done <- completion{size, err} })
	}
	for pending > 0 {
		select {
		case c := <-done:
			if err := consume(c); err != nil {
				return fail(err)
			}
		case <-ctx.Done():
			return fail(ctx.Err())
		}
	}
	return nil
}

// Conservative uncompressed record accounting, including Kafka batch overhead.
func recordSize(r *kgo.Record) int64 {
	n := int64(128 + len(r.Topic) + len(r.Key) + len(r.Value))
	for _, h := range r.Headers {
		n += int64(16 + len(h.Key) + len(h.Value))
	}
	return n
}
