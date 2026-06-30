package request

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/obs"
)

// Limiter gates outgoing requests. Implementations:
//   - StaticLimiter — wraps a fixed-rate rate.Limiter
//   - DynamicLimiter — observes response headers and adjusts the inner
//     limiter, optionally sleeping until a reset timestamp.
type Limiter interface {
	Wait(ctx context.Context) error
	Observe(resp *http.Response)
}

// NewStaticLimiter returns a Limiter that allows rps requests/sec with
// a small burst. rps <= 0 means unlimited.
func NewStaticLimiter(rps float64) Limiter {
	if rps <= 0 {
		return &staticLimiter{rl: rate.NewLimiter(rate.Inf, 0)}
	}
	burst := max(int(rps)*2, 1)
	return &staticLimiter{rl: rate.NewLimiter(rate.Limit(rps), burst)}
}

type staticLimiter struct {
	rl *rate.Limiter
}

func (s *staticLimiter) Wait(ctx context.Context) error { return s.rl.Wait(ctx) }
func (s *staticLimiter) Observe(_ *http.Response)       {}

// DynamicConfig parses provider rate-limit headers.
//
//	RemainingHeader — count of requests remaining in the window
//	ResetHeader     — when the window resets
//	ResetFormat     — unix_seconds | seconds_from_now | http_date
//	MinFloorRPS     — floor for the dynamic limiter so we never starve completely
type DynamicConfig struct {
	RemainingHeader string
	ResetHeader     string
	ResetFormat     string
	MinFloorRPS     float64
}

// DynamicOption configures a DynamicLimiter at construction.
type DynamicOption func(*dynamicLimiter)

// WithDynamicLogger attaches a logger that records header-parse failures.
// Without a logger the limiter still defaults to slog.Default(), but tests
// typically inject a capturing handler.
func WithDynamicLogger(l *slog.Logger) DynamicOption {
	return func(d *dynamicLimiter) { d.logger = obs.Logger(l) }
}

// NewDynamicLimiter wraps a static limiter with header observation. When
// `remaining` drops to zero the next Wait blocks until the reset timestamp
// (or returns immediately on context cancel). Header parse failures are
// logged at WARN so misconfigured limits don't hide behind 429 cascades.
func NewDynamicLimiter(rps float64, cfg DynamicConfig, opts ...DynamicOption) Limiter {
	floor := cfg.MinFloorRPS
	if floor <= 0 {
		floor = 0.5
	}
	d := &dynamicLimiter{
		base:   NewStaticLimiter(rps),
		cfg:    cfg,
		floor:  floor,
		logger: obs.Logger(nil),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type dynamicLimiter struct {
	base   Limiter
	cfg    DynamicConfig
	floor  float64
	logger *slog.Logger

	pauseTil atomic.Pointer[time.Time]
}

func (d *dynamicLimiter) Wait(ctx context.Context) error {
	if pause := d.pauseTil.Load(); pause != nil {
		delay := time.Until(*pause)
		if delay > 0 {
			t := time.NewTimer(delay)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return ctx.Err()
			}
		}
	}
	return d.base.Wait(ctx)
}

func (d *dynamicLimiter) Observe(resp *http.Response) {
	if resp == nil {
		return
	}
	if d.cfg.RemainingHeader != "" {
		raw := resp.Header.Get(d.cfg.RemainingHeader)
		if raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil {
				d.logger.Warn("rate-limit remaining header parse failed",
					"header", d.cfg.RemainingHeader,
					"value", raw,
					"error", err)
			} else if n <= 0 {
				rs := resp.Header.Get(d.cfg.ResetHeader)
				reset, perr := d.parseReset(rs)
				if perr != nil {
					d.logger.Warn("rate-limit reset header parse failed",
						"header", d.cfg.ResetHeader,
						"value", rs,
						"format", d.cfg.ResetFormat,
						"error", perr)
				} else if !reset.IsZero() {
					t := reset
					d.pauseTil.Store(&t)
				}
			}
		}
	}
	d.base.Observe(resp)
}

// parseReset parses the reset header per ResetFormat. Returns (zero, error)
// for unparseable values so the caller can surface misconfigured manifests
// via the logger.
func (d *dynamicLimiter) parseReset(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	switch d.cfg.ResetFormat {
	case "unix_seconds", "":
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %q is not unix seconds", errs.ErrRateLimitParse, s)
		}
		return time.Unix(n, 0), nil
	case "seconds_from_now":
		n, err := strconv.Atoi(s)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %q is not a duration", errs.ErrRateLimitParse, s)
		}
		return time.Now().Add(time.Duration(n) * time.Second), nil
	case "http_date":
		t, err := http.ParseTime(s)
		if err != nil {
			return time.Time{}, fmt.Errorf("%w: %q is not an HTTP date", errs.ErrRateLimitParse, s)
		}
		return t, nil
	}
	return time.Time{}, fmt.Errorf("%w: unknown ResetFormat %q", errs.ErrRateLimitParse, d.cfg.ResetFormat)
}
