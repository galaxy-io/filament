package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

func TestHTTPForbiddenRateLimitRetries(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusTooManyRequests} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			var calls atomic.Int64
			src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "1")
					w.WriteHeader(status)
					fmt.Fprint(w, `{"message":"rate limited"}`)
					return
				}
				_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			})
			var sink collectSink
			var retries atomic.Int64
			if err := src.Extract(context.Background(), &sink, filament.ExtractOpts{
				Resources: []string{"repositories"},
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
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"message":"Resource not accessible by personal access token","documentation_url":"https://docs.github.com/rest"}`)
	})
	var sink collectSink
	err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"repositories"}})
	if err == nil || !strings.Contains(err.Error(), "403") || calls.Load() != 1 {
		t.Fatalf("calls=%d error=%v", calls.Load(), err)
	}
}

func TestRateLimitDelay(t *testing.T) {
	secondary := []byte(`{"message":"You have exceeded a secondary rate limit.","documentation_url":"https://docs.github.com/rest/overview/resources-in-the-rest-api#secondary-rate-limits"}`)
	for _, tc := range []struct {
		name             string
		status           int
		headers          map[string]string
		body             []byte
		attempt          int
		minimum, maximum time.Duration
	}{
		{"server delay exceeds fallback cap", 429, map[string]string{"Retry-After": "120"}, nil, 0, 120 * time.Second, 120 * time.Second},
		{"reset budget", 403, map[string]string{"X-RateLimit-Remaining": "0", "X-RateLimit-Reset": strconv.FormatInt(time.Now().Add(5*time.Minute).Unix(), 10)}, nil, 0, 299 * time.Second, 300 * time.Second},
		{"secondary starts at one minute", 403, nil, secondary, 0, time.Minute, time.Minute},
		{"secondary increases", 403, nil, secondary, 2, 4 * time.Minute, 4 * time.Minute},
		{"secondary honors explicit delay", 403, map[string]string{"Retry-After": "2"}, secondary, 0, 2 * time.Second, 2 * time.Second},
		{"http date", 429, map[string]string{"Retry-After": time.Now().Add(2 * time.Minute).UTC().Format(http.TimeFormat)}, nil, 0, 119 * time.Second, 120 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{StatusCode: tc.status, Header: make(http.Header)}
			for k, v := range tc.headers {
				resp.Header.Set(k, v)
			}
			if !isRateLimited(resp, tc.body) {
				t.Fatal("throttling was not recognized")
			}
			if got := rateLimitDelay(resp, tc.body, tc.attempt); got < tc.minimum || got > tc.maximum {
				t.Fatalf("delay=%s, want [%s,%s]", got, tc.minimum, tc.maximum)
			}
		})
	}
}

func TestHTTPRateLimitWaitCanBeCancelled(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"repositories"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want cancellation", err)
	}
}

func TestHTTPNoContentIsEmptyButInvalidJSONFails(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
			var sink collectSink
			err := src.Extract(context.Background(), &sink, filament.ExtractOpts{Resources: []string{"repositories"}})
			if (err == nil) != (status == http.StatusNoContent) {
				t.Fatalf("status=%d error=%v", status, err)
			}
			if len(sink.records) != 0 {
				t.Fatal("empty response emitted rows")
			}
		})
	}
}

func TestHTTPPendingStatisticsCanBeCancelled(t *testing.T) {
	src := githubTestSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/orgs/test-org/repos" {
			_ = json.NewEncoder(w).Encode([]any{githubRepository(10, "one")})
			return
		}
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, `{}`)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	var sink collectSink
	err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{"commit_activity"}})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error=%v, want cancellation", err)
	}
	if len(sink.records) != 0 {
		t.Fatal("pending statistics must not be emitted")
	}
}
