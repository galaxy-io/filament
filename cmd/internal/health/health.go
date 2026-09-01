// Package health exposes the process lifecycle endpoints used by Kubernetes.
package health

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"
)

// Check verifies one dependency required for the process to accept work.
type Check func(context.Context) error

// State tracks process startup and runs dependency checks for readiness.
type State struct {
	timeout time.Duration
	checks  []Check
	started atomic.Bool
}

// New returns health state with a shared readiness timeout and ordered checks.
func New(timeout time.Duration, checks ...Check) *State {
	return &State{timeout: timeout, checks: append([]Check(nil), checks...)}
}

// Mount registers the Kubernetes lifecycle endpoints on mux.
func (s *State) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/livez", s.livez)
	mux.HandleFunc("/startupz", s.startupz)
	mux.HandleFunc("/readyz", s.readyz)
}

// MarkStarted allows startup and readiness checks to succeed. Call it only
// after every listener, dependency, and module needed to serve work is ready.
func (s *State) MarkStarted() { s.started.Store(true) }

// MarkStopping removes the process from service before shutdown begins.
func (s *State) MarkStopping() { s.started.Store(false) }

func (s *State) livez(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *State) startupz(w http.ResponseWriter, _ *http.Request) {
	if !s.started.Load() {
		http.Error(w, "not started", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *State) readyz(w http.ResponseWriter, r *http.Request) {
	if !s.started.Load() {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()
	if s.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.timeout)
		defer cancel()
	}
	for _, check := range s.checks {
		if err := check(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
}
