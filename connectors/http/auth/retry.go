package auth

import (
	"errors"
	"math"
	"time"

	"github.com/galaxy-io/filament/connectors/http/errs"
)

// RetryPolicy decides whether an auth refresh attempt that errored should be
// retried, and how long to wait first. Implementations are stateless across
// calls — pass `attempt` (1-based) for backoff calculations.
//
// Returns (delay, true) to retry after delay; (0, false) to give up and
// surface the original error to the caller.
type RetryPolicy interface {
	ShouldRetry(err error, attempt int) (time.Duration, bool)
}

// NoRetry is the default policy: never retry. A single refresh failure
// surfaces immediately to the caller.
type NoRetry struct{}

// ShouldRetry always returns false.
func (NoRetry) ShouldRetry(_ error, _ int) (time.Duration, bool) {
	return 0, false
}

// ExponentialBackoff retries up to MaxAttempts times with `Base * 2^(attempt-1)`
// delay, capped at MaxDelay. Only retries errors that wrap
// errs.ErrAuthRefresh (i.e. transient refresh failures); other errors are
// surfaced immediately.
//
// A typical config for a flaky token endpoint:
//
//	auth.ExponentialBackoff{MaxAttempts: 4, Base: 500*time.Millisecond, MaxDelay: 30*time.Second}
type ExponentialBackoff struct {
	MaxAttempts int
	Base        time.Duration
	MaxDelay    time.Duration
}

// ShouldRetry returns the next backoff delay or (0, false) when the cap is
// hit or the error isn't a refresh-class failure.
func (e ExponentialBackoff) ShouldRetry(err error, attempt int) (time.Duration, bool) {
	if !errors.Is(err, errs.ErrAuthRefresh) {
		return 0, false
	}
	if attempt < 1 || attempt >= e.MaxAttempts {
		return 0, false
	}
	mult := math.Pow(2, float64(attempt-1))
	d := time.Duration(float64(e.Base) * mult)
	if e.MaxDelay > 0 && d > e.MaxDelay {
		d = e.MaxDelay
	}
	return d, true
}
