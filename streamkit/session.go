package streamkit

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrHeartbeatRunning means Stop timed out before the protocol callback joined.
// Client cleanup or a canceled context alone is not proof of quiescence.
var ErrHeartbeatRunning = errors.New("streamkit: heartbeat still running")

// Heartbeat runs one serialized protocol task independently of row production.
// Its callback must support cancellation and be safe alongside native reads. It
// must not touch builders, certify coverage, or acknowledge tentative progress.
// A callback failure cancels Context, which the owner must use for blocked reads
// and writes. This helper joins only its callback, not the session or destination.
type Heartbeat struct {
	ctx    context.Context
	cancel context.CancelCauseFunc
	done   chan struct{}
	mu     sync.Mutex
	err    error
}

// StartHeartbeat starts periodic work after the first interval. Use Retry
// explicitly inside the callback if the protocol permits retries. Stop and join
// heartbeat before releasing any client resources its callback uses.
func StartHeartbeat(ctx context.Context, interval time.Duration, task func(context.Context) error) (*Heartbeat, error) {
	if interval <= 0 || task == nil {
		return nil, errors.New("streamkit: invalid heartbeat interval or task")
	}
	child, cancel := context.WithCancelCause(ctx)
	h := &Heartbeat{ctx: child, cancel: cancel, done: make(chan struct{})}
	go h.run(interval, task)
	return h, nil
}

// Context is canceled by parent cancellation, Stop, or a protocol task failure.
// context.Cause preserves the terminal protocol failure for the owner.
func (h *Heartbeat) Context() context.Context { return h.ctx }

// Done closes only after the heartbeat goroutine and callback have exited.
func (h *Heartbeat) Done() <-chan struct{} { return h.done }

// Stop cancels heartbeat and waits for its callback. It is safe to call again
// after a timeout to join a callback that later exits. A timeout returns
// ErrHeartbeatRunning and the wait context's error, never a clean outcome.
func (h *Heartbeat) Stop(ctx context.Context) error {
	h.cancel(context.Canceled)
	select {
	case <-h.done:
		return h.Err()
	default:
	}
	select {
	case <-h.done:
		return h.Err()
	case <-ctx.Done():
		return errors.Join(ErrHeartbeatRunning, ctx.Err())
	}
}

// Err returns a terminal callback error, if one has been observed.
func (h *Heartbeat) Err() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *Heartbeat) run(interval time.Duration, task func(context.Context) error) {
	defer close(h.done)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			if h.ctx.Err() != nil {
				return
			}
			if err := task(h.ctx); err != nil {
				if h.ctx.Err() != nil && errors.Is(err, h.ctx.Err()) {
					return
				}
				h.mu.Lock()
				h.err = err
				h.mu.Unlock()
				h.cancel(err)
				return
			}
		}
	}
}
