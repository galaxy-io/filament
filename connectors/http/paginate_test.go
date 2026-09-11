package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const throttleTestManifest = `
version: 1
name: test
display_name: Test
description: Generic HTTP runtime fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
  rate_limit:
    dynamic:
      remaining_header: Budget-Remaining
      reset_header: Budget-Reset
      reset_format: unix_seconds
    responses:
      - status: 403
        header: Retry-After
      - status: 403
        header: Budget-Remaining
        header_value: "0"
      - status: 403
        body_path: error.detail
        body_contains: capacity exhausted
        backoff_seconds: 90
resources:
  - name: items
    path: /items
    records: $
    primary_key: [id]
    fields:
      id: int64
    response:
      poll_pending: true
`

func throttleTestSource(t *testing.T, handler http.HandlerFunc) *Source {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	src := NewManifest([]byte(strings.Replace(throttleTestManifest, "base_url: https://example.com", "base_url: "+api.URL, 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func throttleTestSpec(t *testing.T) manifest.RateLimit {
	t.Helper()
	m, err := manifest.Parse([]byte(throttleTestManifest))
	if err != nil {
		t.Fatal(err)
	}
	return m.Connection.RateLimit
}

func TestHTTPForbiddenRateLimitRetries(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var calls atomic.Int64
			src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"rate limited"}`)
					return
				}
				fmt.Fprint(w, `[{"id":1}]`)
			})
			var sink collectSink
			var retries atomic.Int64
			if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{
				Resources: []string{"items"},
				Observe: func(p filament.SourceProgress) {
					if p.Kind == filament.SourceProgressRateLimited {
						retries.Add(1)
					}
				},
			}); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 2 || retries.Load() != 1 || len(sink.records) != 1 {
				t.Fatalf("calls=%d retries=%d rows=%d", calls.Load(), retries.Load(), len(sink.records))
			}
		})
	}
}

func TestHTTPForbiddenPermissionsAreFatal(t *testing.T) {
	var calls atomic.Int64
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":{"detail":"permission denied"}}`)
	})
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if err == nil || !strings.Contains(err.Error(), "403") || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
}

func TestRateLimitDelay(t *testing.T) {
	spec := throttleTestSpec(t)
	limited := []byte(`{"error":{"detail":"Capacity exhausted. Try again later."}}`)
	for _, tc := range []struct {
		name             string
		status           int
		headers          map[string]string
		body             []byte
		attempt          int
		minimum, maximum time.Duration
	}{
		{"server delay exceeds fallback cap", 429, map[string]string{"Retry-After": "120"}, nil, 0, 120 * time.Second, 120 * time.Second},
		{"configured backoff", 403, nil, limited, 0, 90 * time.Second, 90 * time.Second},
		{"configured backoff increases", 403, nil, limited, 2, 6 * time.Minute, 6 * time.Minute},
		{"explicit delay takes precedence", 403, map[string]string{"Retry-After": "2"}, limited, 0, 2 * time.Second, 2 * time.Second},
		{"http date", 429, map[string]string{"Retry-After": time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)}, nil, 0, 119 * time.Second, 120 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: make(http.Header)}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			got, limited := rateLimitDelay(resp, tc.body, tc.attempt, spec)
			if !limited {
				t.Fatal("throttling was not recognized")
			}
			if got < tc.minimum || got > tc.maximum {
				t.Fatalf("delay=%s, want [%s,%s]", got, tc.minimum, tc.maximum)
			}
		})
	}
}

func TestHTTPRateLimitWaitCanBeCancelled(t *testing.T) {
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want cancellation", err)
	}
}

func TestHTTPNoContentIsEmptyButInvalidJSONFails(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
			var sink collectSink
			err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"items"}})
			if (err == nil) != (status == http.StatusNoContent) {
				t.Fatalf("status=%d error=%v", status, err)
			}
			if len(sink.records) != 0 {
				t.Fatal("empty response emitted rows")
			}
		})
	}
}

func TestHTTPPendingResponseCanBeCancelled(t *testing.T) {
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{}`)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want cancellation", err)
	}
	if len(sink.records) != 0 {
		t.Fatal("pending responses must not be emitted")
	}
}

func TestRateLimitMatchingIsManifestScoped(t *testing.T) {
	spec := throttleTestSpec(t)
	for _, tc := range []struct {
		name, body string
		status     int
		headers    map[string]string
		want       bool
	}{
		{"body match", `{"error":{"detail":"CAPACITY EXHAUSTED"}}`, 403, nil, true},
		{"permission denial", `{"error":{"detail":"permission denied"}}`, 403, nil, false},
		{"wrong status", `{"error":{"detail":"capacity exhausted"}}`, 401, nil, false},
		{"wrong body path", `{"message":"capacity exhausted"}`, 403, nil, false},
		{"invalid JSON", `{"error":{"detail":"capacity exhausted"}} trailing`, 403, nil, false},
		{"wrong body type", `{"error":{"detail":{"value":"capacity exhausted"}}}`, 403, nil, false},
		{"header presence", "", 403, map[string]string{"Retry-After": "2"}, true},
		{"exhausted budget", "", 403, map[string]string{"Budget-Remaining": "0"}, true},
		{"available budget", "", 403, map[string]string{"Budget-Remaining": "1"}, false},
		{"unconfigured header", "", 403, map[string]string{"Other-Remaining": "0"}, false},
		{"standard status", "", 429, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: make(http.Header)}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			if _, got := rateLimitDelay(resp, []byte(tc.body), 0, spec); got != tc.want {
				t.Fatalf("configured=%v, want %v", got, tc.want)
			}
			if _, got := rateLimitDelay(resp, []byte(tc.body), 0, manifest.RateLimit{}); got != (tc.status == 429) {
				t.Fatal("unconfigured provider-specific response was retried")
			}
		})
	}
	// Conditions on the same rule are conjunctive.
	rule := manifest.RateLimitResponse{Status: 403, Header: "Budget-Remaining", HeaderValue: "0", BodyPath: "error.detail", BodyContains: "capacity exhausted"}
	resp := &http.Response{StatusCode: 403, Header: http.Header{"Budget-Remaining": []string{"1"}}}
	if matchesRateLimit(resp, []byte(`{"error":{"detail":"capacity exhausted"}}`), rule) {
		t.Fatal("body match bypassed header condition")
	}
}

func TestHTTPBudgetResetUsesConfiguredLimiter(t *testing.T) {
	var calls atomic.Int64
	src := throttleTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Budget-Remaining", "0")
		w.Header().Set("Budget-Reset", strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10))
		w.WriteHeader(http.StatusForbidden)
	})
	// The retry backoff expires after one second; the limiter must hold the
	// next request until the budget resets or the caller cancels.
	ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"items"}})
	if !errors.Is(err, context.DeadlineExceeded) || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
}
