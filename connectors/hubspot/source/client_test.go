package hubspot

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestClientRetriesRateLimitsAndServerErrors(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "0")
		switch attempts {
		case 1:
			w.WriteHeader(429)
		case 2:
			w.WriteHeader(503)
		default:
			_, _ = w.Write([]byte(`{"results":[]}`))
		}
	}))
	defer server.Close()
	c := newClient("test")
	defer c.http.CloseIdleConnections()
	c.baseURL, c.limiter = server.URL, rate.NewLimiter(rate.Inf, 1)
	var out objectPage
	if err := c.request(t.Context(), "contacts", http.MethodPost, "/search", searchRequest{}, &out); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d", attempts)
	}
}

func TestClientDoesNotRetryAccessErrorsOrExposeResponseValues(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(403)
		_, _ = w.Write([]byte(`{"category":"MISSING_SCOPES","correlationId":"diagnostic-id","message":"private-value"}`))
	}))
	defer server.Close()
	c := newClient("test")
	defer c.http.CloseIdleConnections()
	c.baseURL = server.URL
	err := c.request(t.Context(), "contacts", http.MethodGet, "/records", nil, &objectPage{})
	var api *apiError
	if !errors.As(err, &api) || api.Status != 403 || attempts != 1 {
		t.Fatalf("attempts=%d error=%v", attempts, err)
	}
	if strings.Contains(err.Error(), "private-value") || !strings.Contains(err.Error(), "diagnostic-id") {
		t.Fatal(err)
	}
}

func TestClientCancellationStopsRetryWait(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
		cancel()
	}))
	defer server.Close()
	c := newClient("test")
	defer c.http.CloseIdleConnections()
	c.baseURL = server.URL
	if err := c.request(ctx, "contacts", http.MethodGet, "/records", nil, &objectPage{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d", attempts)
	}
}

func TestRetryAfter(t *testing.T) {
	now := testTime
	for _, test := range []struct {
		value   string
		attempt int
		want    time.Duration
	}{
		{"2", 0, 2 * time.Second},
		{now.Add(time.Minute).Format(http.TimeFormat), 0, time.Minute},
		{"invalid", 2, 4 * time.Second},
		{"-1", 0, time.Second},
	} {
		if got := retryDelay(test.value, test.attempt, now); got != test.want {
			t.Fatalf("Retry-After %q = %v, want %v", test.value, got, test.want)
		}
	}
}
