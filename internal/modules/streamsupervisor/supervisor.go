// Package streamsupervisor reconciles durable activations independently of bus delivery.
package streamsupervisor

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/runner"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/version"
)

// Supervisor reconciles durable intent and dispatches admitted worker attempts.
type Supervisor struct {
	deps     runner.Deps
	store    filament.ContinuousRunStore
	dispatch filament.Dispatcher
	cancel   context.CancelFunc
	done     chan struct{}
	workers  sync.WaitGroup
	mu       sync.Mutex
	running  map[string]bool
	retries  map[string]*retryState
	// yielded records that this pass already yielded one stale attempt.
	yielded bool
}

// New creates a supervisor; a nil dispatcher executes workers in process.
func New(deps runner.Deps, dispatch filament.Dispatcher) *Supervisor {
	return &Supervisor{deps: deps, store: deps.StreamStore, dispatch: dispatch, running: map[string]bool{}}
}

// Start begins reconciliation until cancellation or Close. Call it once.
func (s *Supervisor) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		if s.store == nil {
			return
		}
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if err := s.Reconcile(ctx); err != nil && ctx.Err() == nil && s.deps.Log != nil {
				s.deps.Log.Error("continuous reconciliation failed", err)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

// Close stops reconciliation and waits for in-process workers to drain.
func (s *Supervisor) Close() {
	if s.cancel != nil {
		s.cancel()
		<-s.done
		s.workers.Wait()
	}
}

// Reconcile dispatches pending runs using durable ownership and intent.
func (s *Supervisor) Reconcile(ctx context.Context) error {
	if s.store == nil {
		return nil
	}
	s.yielded = false
	after := ""
	for {
		runs, err := s.store.PendingStreamRuns(ctx, after, 100)
		if err != nil {
			return err
		}
		for _, r := range runs {
			if err := s.reconcileRun(ctx, r); err != nil && !errors.Is(err, filament.ErrVersionConflict) && !errors.Is(err, filament.ErrTakeoverBlocked) && !errors.Is(err, filament.ErrFenced) {
				if s.deps.Log != nil {
					s.deps.Log.Error("continuous dispatch failed", err, filament.Field{Key: "run_id", Value: string(r.Run)})
				}
			}
		}
		if len(runs) < 100 {
			return nil
		}
		after = string(runs[len(runs)-1].Run)
	}
}

func (s *Supervisor) reconcileRun(ctx context.Context, r filament.RunState) error {
	req, err := r.StreamStateRequest()
	if err != nil {
		return err
	}
	state, err := s.store.LoadStreamState(ctx, req)
	// Deletion can race the pending-run scan. There is no remaining work.
	if errors.Is(err, filament.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if state.Run != r.Run {
		return nil
	}
	delay, exhausted := s.retryPolicy(state)
	a := state.Attempt
	if a != nil && a.EndedAt == nil {
		if !a.Claimed && state.Desired != filament.StreamEnabled {
			return s.store.RetireUnclaimedAttempt(ctx, a.Lease)
		}
		if !a.ExpiresAt.After(time.Now()) {
			if !a.Claimed {
				return s.store.RetireUnclaimedAttempt(ctx, a.Lease)
			}
			return filament.ErrTakeoverBlocked
		}
		if a.Claimed && state.Desired == filament.StreamEnabled {
			return s.yieldStale(ctx, r.Run, a)
		}
		if a.Claimed || state.Desired != filament.StreamEnabled {
			return nil
		}
	} else {
		if state.Desired != filament.StreamEnabled {
			return nil
		}
		if a != nil {
			// The persisted attempt reason survives supervisor restarts. A manual
			// pause/resume changes the revision and permits a fresh assessment.
			if schemaBlocked(state) {
				return nil
			}
			if a.Termination != filament.AttemptClean {
				return filament.ErrTakeoverBlocked
			}
			if exhausted {
				err := s.store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: req, ExpectedRevision: state.Revision, Desired: filament.StreamPaused})
				if err == nil && s.deps.Log != nil {
					s.deps.Log.Error("continuous pipeline paused after five retries", errors.New(a.Reason), filament.Field{Key: "run_id", Value: string(r.Run)})
				}
				return err
			}
			if time.Since(*a.EndedAt) < delay {
				return nil
			}
		}
		admitted, err := s.store.StartAttempt(ctx, filament.StartAttemptRequest{Tenant: r.Tenant, Stream: req.Stream, Run: r.Run, ExecutionID: uuid.NewString(), PipelineVersionID: state.PipelineVersionID, Route: state.Route, DispatchVersion: version.Version, ExpectedRevision: state.Revision, TTL: runner.DefaultLeaseTTL})
		if err != nil {
			return err
		}
		a = &admitted
	}
	if s.dispatch != nil {
		_, err := s.dispatch.Dispatch(ctx, a.Spec)
		return err
	}
	id := a.Spec.ExecutionID
	s.mu.Lock()
	if s.running[id] {
		s.mu.Unlock()
		return nil
	}
	s.running[id] = true
	s.workers.Add(1)
	s.mu.Unlock()
	go func(spec filament.RunSpec) {
		defer s.workers.Done()
		defer func() { s.mu.Lock(); delete(s.running, id); s.mu.Unlock() }()
		if err := runner.ExecuteContinuousAttempt(ctx, s.deps, spec); err != nil && s.deps.Log != nil {
			s.deps.Log.Error("continuous attempt ended", err, filament.Field{Key: "execution_id", Value: id})
		}
	}(a.Spec)
	return nil
}

// yieldStale yields a live attempt dispatched by an older release, at most one
// per reconcile pass, so an upgrade restarts streams one at a time. The stream
// stays enabled and the next pass starts a fresh attempt on the current image.
func (s *Supervisor) yieldStale(ctx context.Context, run filament.RunID, a *filament.Attempt) error {
	if a.YieldRequested || a.DispatchVersion == version.Version || s.yielded {
		return nil
	}
	if err := s.store.YieldAttempt(ctx, a.Lease); err != nil {
		return err
	}
	s.yielded = true
	if s.deps.Log != nil {
		s.deps.Log.Info("continuous attempt yielded for upgrade",
			filament.Field{Key: "run_id", Value: string(run)},
			filament.Field{Key: "execution_id", Value: a.Lease.Attempt.ExecutionID},
			filament.Field{Key: "attempt_version", Value: a.DispatchVersion},
			filament.Field{Key: "control_plane_version", Value: version.Version})
	}
	return nil
}

func schemaBlocked(state filament.StreamState) bool {
	a := state.Attempt
	return a != nil && a.EndedAt != nil && a.Revision == state.Revision && strings.HasPrefix(a.Reason, schema.ErrBlocked.Error()+":")
}
