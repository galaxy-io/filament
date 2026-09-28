package streamkit

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestHeartbeatFailureCancelsOwner(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		failure := errors.New("protocol lost")
		m, err := StartHeartbeat(context.Background(), time.Second, func(context.Context) error { return failure })
		if err != nil {
			t.Fatal(err)
		}
		<-m.Context().Done()
		if !errors.Is(context.Cause(m.Context()), failure) {
			t.Fatal("lost protocol failure")
		}
		if !errors.Is(m.Stop(context.Background()), failure) {
			t.Fatal("Stop lost callback failure")
		}
	})
}

func TestHeartbeatStopJoinsAndIsRepeatable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered := make(chan struct{})
		var calls atomic.Int64
		m, err := StartHeartbeat(context.Background(), time.Second, func(ctx context.Context) error {
			calls.Add(1)
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
		<-entered
		for range 2 {
			if err := m.Stop(context.Background()); err != nil {
				t.Fatal(err)
			}
		}
		if calls.Load() != 1 {
			t.Fatal("heartbeat continued after stop")
		}
		select {
		case <-m.Done():
		default:
			t.Fatal("Stop did not join")
		}
	})
}

func TestHeartbeatUnprovenStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		m, err := StartHeartbeat(context.Background(), time.Second, func(context.Context) error {
			close(entered)
			<-release // Simulate a client that ignores cancellation.
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		<-entered
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err = m.Stop(ctx)
		if !errors.Is(err, ErrHeartbeatRunning) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("unproven stop: %v", err)
		}
		select {
		case <-m.Done():
			t.Fatal("claimed callback joined")
		default:
		}
		close(release)
		if err := m.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHeartbeatParentCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		m, err := StartHeartbeat(ctx, time.Second, func(context.Context) error { t.Error("callback after cancellation"); return nil })
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		if err := m.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	})
}

func TestRetryBoundsAndClassification(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		failure := errors.New("transient")
		calls := 0
		policy := RetryPolicy{MaxAttempts: 3, Delay: time.Second, Retryable: func(err error) bool { return errors.Is(err, failure) }}
		start := time.Now()
		err := Retry(context.Background(), policy, func(context.Context) error { calls++; return failure })
		if !errors.Is(err, failure) || calls != 3 || time.Since(start) != 2*time.Second {
			t.Fatalf("attempts=%d err=%v", calls, err)
		}
		calls = 0
		policy.Retryable = nil
		_ = Retry(context.Background(), policy, func(context.Context) error { calls++; return failure })
		if calls != 1 {
			t.Fatal("retried without explicit classification")
		}
	})
}

func TestRetryCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		policy := RetryPolicy{MaxAttempts: 3, Delay: time.Hour, Retryable: func(error) bool { return true }}
		err := Retry(ctx, policy, func(context.Context) error { calls++; cancel(); return nil })
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("cancellation became success: %v", err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		calls = 0
		err = Retry(ctx, policy, func(context.Context) error { calls++; return errors.New("retry") })
		if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
			t.Fatalf("backoff cancellation: %v, calls %d", err, calls)
		}
	})
}

func TestRetrySuccessAndInvalidConfiguration(t *testing.T) {
	calls := 0
	err := Retry(context.Background(), RetryPolicy{MaxAttempts: 3, Retryable: func(error) bool { return true }}, func(context.Context) error {
		calls++
		if calls == 1 {
			return errors.New("temporary")
		}
		return nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("success: calls=%d err=%v", calls, err)
	}
	for _, policy := range []RetryPolicy{{MaxAttempts: -1}, {Delay: -1}} {
		if err := Retry(context.Background(), policy, func(context.Context) error { t.Error("invalid retry invoked operation"); return nil }); err == nil {
			t.Fatal("invalid policy accepted")
		}
	}
	if _, err := StartHeartbeat(context.Background(), 0, func(context.Context) error { return nil }); err == nil {
		t.Fatal("invalid heartbeat accepted")
	}
}
