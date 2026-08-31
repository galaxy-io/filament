package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestLifecycleEndpoints(t *testing.T) {
	var checks atomic.Int32
	state := New(time.Second, func(context.Context) error {
		checks.Add(1)
		return nil
	})
	mux := http.NewServeMux()
	state.Mount(mux)

	assertStatus(t, mux, "/livez", http.StatusOK)
	assertStatus(t, mux, "/startupz", http.StatusServiceUnavailable)
	assertStatus(t, mux, "/readyz", http.StatusServiceUnavailable)
	if got := checks.Load(); got != 0 {
		t.Fatalf("checks before startup = %d, want 0", got)
	}

	state.MarkStarted()
	assertStatus(t, mux, "/startupz", http.StatusOK)
	assertStatus(t, mux, "/readyz", http.StatusOK)
	if got := checks.Load(); got != 1 {
		t.Fatalf("checks after startup = %d, want 1", got)
	}

	state.MarkStopping()
	assertStatus(t, mux, "/startupz", http.StatusServiceUnavailable)
	assertStatus(t, mux, "/readyz", http.StatusServiceUnavailable)
}

func TestReadinessFailsWhenDependencyFails(t *testing.T) {
	state := New(time.Second, func(context.Context) error { return errors.New("down") })
	state.MarkStarted()
	mux := http.NewServeMux()
	state.Mount(mux)
	assertStatus(t, mux, "/readyz", http.StatusServiceUnavailable)
}

func TestReadinessCheckHasDeadline(t *testing.T) {
	state := New(time.Second, func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); !ok {
			return errors.New("missing deadline")
		}
		return nil
	})
	state.MarkStarted()
	mux := http.NewServeMux()
	state.Mount(mux)
	assertStatus(t, mux, "/readyz", http.StatusOK)
}

func assertStatus(t *testing.T, handler http.Handler, path string, want int) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != want {
		t.Fatalf("GET %s status = %d, want %d", path, rec.Code, want)
	}
}
