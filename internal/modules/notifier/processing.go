package notifier

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	notification "github.com/galaxy-io/filament/internal/notifier"
)

func (m *Module) deliverAll(ctx context.Context, trigger notification.Notification, rules []notification.Notifier) error {
	var next atomic.Int64
	var wg sync.WaitGroup
	for range min(maxConcurrent, len(rules)) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i := int(next.Add(1) - 1)
				if i >= len(rules) {
					return
				}
				m.deliverWithRetry(ctx, trigger, rules[i])
			}
		}()
	}
	wg.Wait()
	return ctx.Err()
}

func (m *Module) deliverWithRetry(ctx context.Context, trigger notification.Notification, rule notification.Notifier) {
	for i := 0; i < maxAttempts && ctx.Err() == nil; i++ {
		if i > 0 {
			timer := time.NewTimer(time.Second << (i - 1))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return
		}
		a := m.deliver(ctx, trigger, rule)
		if a.skipped {
			return
		}
		m.report(ctx, a)
		if !a.result.Retryable {
			return
		}
	}
}

// keepAlive prevents redelivery while a large batch is still being processed.
func (m *Module) keepAlive(ctx context.Context, cancel context.CancelFunc, msg eventbus.Message) func() {
	progress, ok := msg.(eventbus.ProgressReporter)
	if !ok {
		return func() {}
	}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			if err := progress.InProgress(); err != nil {
				if m.log != nil {
					m.log.Warn("notification acknowledgement extension failed")
				}
				cancel()
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	return func() {
		close(stop)
		<-done
	}
}
