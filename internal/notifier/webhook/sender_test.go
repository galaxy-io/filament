package webhook

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/galaxy-io/filament/internal/notifier"
)

func TestRetryAfterSeconds(t *testing.T) {
	const maxSeconds = math.MaxInt64 / int64(time.Second)
	for _, tc := range []struct {
		value string
		want  time.Duration
	}{
		{"", 0},
		{"0", 0},
		{"-1", 0},
		{"invalid", 0},
		{"1.5", 0},
		{"7", 7 * time.Second},
		{"2147483648", 2147483648 * time.Second},
		{strconv.FormatInt(maxSeconds, 10), time.Duration(maxSeconds) * time.Second},
		{strconv.FormatInt(maxSeconds+1, 10), time.Duration(math.MaxInt64)},
		{strconv.FormatInt(math.MaxInt64, 10), time.Duration(math.MaxInt64)},
	} {
		t.Run(tc.value, func(t *testing.T) {
			if got := retryAfter(tc.value); got != tc.want {
				t.Fatalf("retryAfter(%q) = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestPostLargeRetryAfter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", strconv.FormatInt(math.MaxInt64, 10))
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()

	result := New(server.Client()).Post(context.Background(), server.URL, nil, []byte(`{}`))
	if result.Outcome != notifier.OutcomeFailed || !result.Retryable || result.ErrorCode != notifier.ErrorRateLimited {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.RetryAfter != time.Duration(math.MaxInt64) {
		t.Fatalf("RetryAfter = %v, want saturated duration", result.RetryAfter)
	}
}

func TestRetryAfterDate(t *testing.T) {
	if got := retryAfter(time.Now().Add(-time.Hour).UTC().Format(http.TimeFormat)); got != 0 {
		t.Fatalf("past date = %v, want zero", got)
	}
	value := time.Now().Add(time.Hour).UTC().Format(http.TimeFormat)
	at, err := http.ParseTime(value)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	got := retryAfter(value)
	after := time.Now()
	if got < at.Sub(after) || got > at.Sub(before) {
		t.Fatalf("future date delay %v is outside [%v, %v]", got, at.Sub(after), at.Sub(before))
	}
}
