package sink

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// publishTracker is separate from Sink.mu: SDK callbacks must never wait for
// Apply or call SDK methods. Each Apply owns a new tracker, keyed by the exact
// message pointers reserved before submission, so late/duplicate callbacks
// cannot release capacity belonging to another publication.
type publishTracker struct {
	mu       sync.Mutex
	pending  map[*nats.Msg]int64
	bytes    int64
	failure  error
	changed  chan struct{}
	maxCount int
	maxBytes int64
}

func newPublishTracker(cfg config) *publishTracker {
	return &publishTracker{pending: make(map[*nats.Msg]int64), changed: make(chan struct{}, 1), maxCount: cfg.maxInFlight, maxBytes: cfg.maxInFlightBytes}
}

// reserve reports whether a message fits, or whether all work is done for nil.
// Reservation happens before PublishMsgAsync, which may invoke a callback
// before returning to its caller.
func (p *publishTracker) reserve(msg *nats.Msg) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return false, p.failure
	}
	if msg == nil {
		return len(p.pending) == 0, nil
	}
	size := int64(msg.Size())
	if len(p.pending) >= p.maxCount || size > p.maxBytes-p.bytes {
		return false, nil
	}
	p.pending[msg] = size
	p.bytes += size
	return true, nil
}

func (p *publishTracker) complete(msg *nats.Msg, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	size, ok := p.pending[msg]
	if !ok {
		return
	}
	delete(p.pending, msg)
	p.bytes -= size
	if err != nil && p.failure == nil {
		p.failure = err
	}
	// Coalesce notifications instead of blocking the SDK acknowledgment handler.
	select {
	case p.changed <- struct{}{}:
	default:
	}
}

func (s *Sink) publishAck(_ jetstream.JetStream, msg *nats.Msg, ack *jetstream.PubAck) {
	if pending := s.publications.Load(); pending != nil {
		var err error
		if ack == nil || ack.Stream != s.cfg.stream || ack.Sequence == 0 {
			err = errors.New("nats sink: invalid publish acknowledgment")
		}
		pending.complete(msg, err)
	}
}

func (s *Sink) publishError(_ jetstream.JetStream, msg *nats.Msg, err error) {
	if pending := s.publications.Load(); pending != nil {
		if errors.Is(err, jetstream.ErrAsyncPublishTimeout) {
			err = errors.Join(context.DeadlineExceeded, err)
		}
		if err == nil {
			err = errors.New("nats sink: missing publish error")
		}
		pending.complete(msg, err)
	}
}

// publishMessages submits in row order and waits for callback-driven completion.
// The SDK enforces acknowledgment deadlines; no future channels or per-message
// waiting goroutines are allocated here.
func (s *Sink) publishMessages(ctx context.Context, messages []*nats.Msg) (err error) {
	if len(messages) == 0 {
		return ctx.Err()
	}
	pending := newPublishTracker(s.cfg)
	s.publications.Store(pending)
	attempted := false
	defer func() {
		// Detach before SDK cleanup, which itself can invoke error callbacks.
		s.publications.Store(nil)
		if err != nil {
			s.cleanupPublisher()
			if attempted {
				s.uncertain = fmt.Errorf("nats sink: publish outcome uncertain: %w", err)
				err = s.uncertain
			}
		}
	}()
	for next := 0; ; {
		if err := ctx.Err(); err != nil {
			return err
		}
		var msg *nats.Msg
		if next < len(messages) {
			msg = messages[next]
		}
		ready, err := pending.reserve(msg)
		if err != nil {
			return err
		}
		if ready {
			if msg == nil {
				return nil
			}
			attempted = true
			if _, err := s.publisher.PublishMsgAsync(msg, jetstream.WithRetryAttempts(0), jetstream.WithStallWait(s.cfg.timeout)); err != nil {
				return err
			}
			next++
			continue
		}
		select {
		case <-pending.changed:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
