package streamkit

import (
	"context"
	"errors"
	"time"
)

// RetryPolicy bounds explicitly retryable protocol operations. MaxAttempts
// includes the first call; zero means one call. Delay is fixed between attempts.
// Retryable must be supplied to retry an error. The caller must establish that
// replaying the operation is safe; do not wrap a partially emitted Read or Apply.
type RetryPolicy struct {
	MaxAttempts int
	Delay       time.Duration
	Retryable   func(error) bool
}

// Retry runs an operation with bounded attempts and cancellation-aware waits.
// No attempt starts after cancellation is observed, and cancellation is never
// converted into success, even when the operation returns nil.
func Retry(ctx context.Context, policy RetryPolicy, operation func(context.Context) error) error {
	if policy.MaxAttempts < 0 || policy.Delay < 0 || operation == nil {
		return errors.New("streamkit: invalid retry policy or operation")
	}
	attempts := max(1, policy.MaxAttempts)
	for attempt := 0; attempt < attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		err := operation(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) ||
			attempt == attempts-1 || policy.Retryable == nil || !policy.Retryable(err) {
			return err
		}
		timer := time.NewTimer(policy.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}
