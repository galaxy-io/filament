// Package streamsupervisor reconciles durable activations independently of bus delivery.
package streamsupervisor

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/runner"
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
}

// New creates a supervisor; a nil dispatcher executes workers in process.
func New(deps runner.Deps, dispatch filament.Dispatcher) *Supervisor {
	store, _ := deps.DataStore.(filament.ContinuousRunStore)
	return &Supervisor{deps: deps, store: store, dispatch: dispatch, running: map[string]bool{}}
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
	if err != nil {
		return err
	}
	if state.Run != r.Run {
		return nil
	}
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
		if a.Claimed || state.Desired != filament.StreamEnabled {
			return nil
		}
	} else {
		if state.Desired != filament.StreamEnabled {
			return nil
		}
		if a != nil {
			if a.Termination != filament.AttemptClean {
				return filament.ErrTakeoverBlocked
			}
			if time.Since(*a.EndedAt) < 5*time.Second {
				return nil
			}
		}
		admitted, err := s.store.StartAttempt(ctx, filament.StartAttemptRequest{Tenant: r.Tenant, Stream: req.Stream, Run: r.Run, ExecutionID: uuid.NewString(), PipelineVersionID: state.PipelineVersionID, Route: state.Route, ExpectedRevision: state.Revision, TTL: runner.DefaultLeaseTTL})
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
