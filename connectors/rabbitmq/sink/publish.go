package sink

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"
)

// confirmFaultForTest, when set, fails the next fully confirmed batch. Tests
// use it to replay messages the broker has already stored.
var confirmFaultForTest atomic.Pointer[func() error]

// SetConfirmFaultForTest installs a one-shot fault applied after the broker
// confirms every message in the next batch. Pass nil to clear it.
func SetConfirmFaultForTest(fn func() error) {
	if fn == nil {
		confirmFaultForTest.Store(nil)
		return
	}
	confirmFaultForTest.Store(&fn)
}

func takeConfirmFault() error {
	fn := confirmFaultForTest.Swap(nil)
	if fn == nil || *fn == nil {
		return nil
	}
	return (*fn)()
}

// publishTracker is separate from Sink.mu. Confirmation callbacks must never
// wait for Apply. Each Apply owns a new tracker, keyed by publishing id.
type publishTracker struct {
	mu       sync.Mutex
	pending  map[int64]int64
	bytes    int64
	failure  error
	changed  chan struct{}
	maxCount int
	maxBytes int64
}

func newPublishTracker(cfg config) *publishTracker {
	return &publishTracker{pending: map[int64]int64{}, changed: make(chan struct{}, 1), maxCount: cfg.maxInFlight, maxBytes: cfg.maxInFlightBytes}
}

func (p *publishTracker) reserve(id, size int64) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return false, p.failure
	}
	if len(p.pending) >= p.maxCount || size > p.maxBytes-p.bytes {
		return false, nil
	}
	p.pending[id] = size
	p.bytes += size
	return true, nil
}

func (p *publishTracker) finished() (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return false, p.failure
	}
	return len(p.pending) == 0, nil
}

func (p *publishTracker) complete(id int64, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	size, ok := p.pending[id]
	if !ok {
		return
	}
	delete(p.pending, id)
	p.bytes -= size
	if err != nil && p.failure == nil {
		p.failure = err
	}
	select {
	case p.changed <- struct{}{}:
	default:
	}
}

func (d *destination) readConfirms(ch stream.ChannelPublishConfirm) {
	for batch := range ch {
		tracker := d.publications.Load()
		if tracker == nil {
			continue
		}
		for _, status := range batch {
			if status == nil {
				continue
			}
			tracker.complete(status.GetPublishingId(), confirmError(status))
			for _, linked := range status.LinkedMessages() {
				if linked != nil {
					tracker.complete(linked.GetPublishingId(), confirmError(linked))
				}
			}
		}
	}
}

func confirmError(status *stream.ConfirmationStatus) error {
	if status.IsConfirmed() {
		return nil
	}
	if err := status.GetError(); err != nil {
		return err
	}
	return errors.New("rabbitmq sink: publish was not confirmed")
}

// publishMessages sends in row order and waits for every confirmation.
func (s *Sink) publishMessages(ctx context.Context, d *destination, messages []*amqp.AMQP10, sizes []int64) (err error) {
	if len(messages) == 0 {
		return ctx.Err()
	}
	pending := newPublishTracker(s.cfg)
	d.publications.Store(pending)
	attempted := false
	defer func() {
		d.publications.Store(nil)
		if err != nil && attempted {
			s.uncertain = fmt.Errorf("rabbitmq sink: publish outcome uncertain: %w", err)
			err = s.uncertain
		}
	}()
	next := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if next < len(messages) {
			id := messages[next].GetPublishingId()
			ready, err := pending.reserve(id, sizes[next])
			if err != nil {
				return err
			}
			if !ready {
				if err := waitPublish(ctx, pending); err != nil {
					return err
				}
				continue
			}
			attempted = true
			if err := d.producer.Send(messages[next]); err != nil {
				return err
			}
			next++
			continue
		}
		done, err := pending.finished()
		if err != nil {
			return err
		}
		if done {
			return takeConfirmFault()
		}
		if err := waitPublish(ctx, pending); err != nil {
			return err
		}
	}
}

func waitPublish(ctx context.Context, pending *publishTracker) error {
	select {
	case <-pending.changed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
