package tracker

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
	runcommands "github.com/galaxy-io/filament/internal/runs"
)

// applyCommand folds a pause, resume, or cancel accepted for a run that is not
// executing. The transition is planned from the status loaded here, and folds
// run one at a time, so no other fold can save a status it read before the
// command over the command's status.
//
// The outcome fact announces the transition to dispatch and the notifier. Its
// own fold finishes a paused or canceled run, as it does when a worker
// publishes it.
func (m *Module) applyCommand(ctx context.Context, env events.Envelope, data any) error {
	signal, ok := runcommands.CommandSignal(data)
	if !ok {
		return nil
	}
	key := runKey{env.Tenant, env.Run}
	if err := m.announce(ctx, key); err != nil {
		return err
	}
	transitions, ok := m.ds.(filament.RunTransitionStore)
	if !ok {
		// Redelivery cannot fix a missing capability, and a nak would stall
		// every later fact behind this one.
		if m.log != nil {
			m.log.Error("run command ignored", errors.New("datastore does not support run transitions"),
				filament.Field{Key: "event.name", Value: "tracker.command.unsupported"},
				filament.Field{Key: "run_id", Value: string(env.Run)})
		}
		return nil
	}
	state, err := m.ds.LoadRun(ctx, env.Tenant, env.Run)
	if errors.Is(err, filament.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	transition, ok := runcommands.PlanTransition(state, signal)
	if !ok && (signal == filament.SignalPause && state.Status == filament.RunPaused ||
		signal == filament.SignalCancel && state.Status == filament.RunCanceled) {
		// Already applied, by this command before a restart lost its outcome,
		// or by another command. A repeated outcome is dropped by the broker
		// or folds as stale. A resume is not repeated, because a second
		// run.requested would dispatch the run twice.
		m.mu.Lock()
		m.unannounced[key] = runcommands.Outcome(env, signal)
		m.mu.Unlock()
		return m.announce(ctx, key)
	}
	if !ok {
		// Another fact moved the run first, for example run.started before a
		// cancel. The caller's Signal reports the conflict.
		if m.log != nil {
			m.log.Debug("run command not applicable",
				filament.Field{Key: "event.name", Value: "tracker.command.skipped"},
				filament.Field{Key: "run_id", Value: string(env.Run)},
				filament.Field{Key: "signal", Value: int(signal)},
				filament.Field{Key: "status", Value: int(state.Status)})
		}
		return nil
	}
	if _, err := transitions.TransitionRun(ctx, env.Tenant, env.Run,
		[]filament.RunStatus{transition.From}, transition.To, transition.Options); err != nil {
		return err
	}
	m.mu.Lock()
	m.unannounced[key] = runcommands.Outcome(env, signal)
	m.mu.Unlock()
	return m.announce(ctx, key)
}

// announce publishes the outcome of a command the tracker applied and has not
// announced yet. A failed publish naks the command; once applied, its
// redelivery no longer plans a transition, so the outcome waits here instead.
func (m *Module) announce(ctx context.Context, key runKey) error {
	m.mu.Lock()
	outcome, ok := m.unannounced[key]
	m.mu.Unlock()
	if !ok {
		return nil
	}
	if m.bus != nil {
		if err := events.Publish(ctx, m.bus, outcome); err != nil {
			return fmt.Errorf("tracker: publish %s for run %q: %w", outcome.Name, key.run, err)
		}
	}
	m.mu.Lock()
	delete(m.unannounced, key)
	m.mu.Unlock()
	return nil
}

// staleOutcome reports a pause or cancel outcome whose run no longer has the
// status the command stored, because a later command already moved it. Such
// an outcome must not fold, or it would restore the earlier status.
func (m *Module) staleOutcome(ctx context.Context, f events.Fact) (bool, error) {
	var want filament.RunStatus
	switch d := f.Data.(type) {
	case events.RunPausedEvent:
		if !d.Commanded {
			return false, nil
		}
		want = filament.RunPaused
	case events.RunCanceledEvent:
		if !d.Commanded {
			return false, nil
		}
		want = filament.RunCanceled
	default:
		return false, nil
	}
	state, err := m.ds.LoadRun(ctx, f.Tenant, f.Run)
	if errors.Is(err, filament.ErrNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return state.Status != want, nil
}

// applyStarted folds run.started. A worker admitted before a pause or cancel
// command folded still runs, so the tracker asks it to stop the way SignalRun
// asks a running worker, and the worker acknowledges with its own fact. The
// request is published before the fold, so a failed save redelivers both.
func (m *Module) applyStarted(ctx context.Context, env events.Envelope) error {
	state, err := m.ds.LoadRun(ctx, env.Tenant, env.Run)
	if err != nil && !errors.Is(err, filament.ErrNotFound) {
		return err
	}
	if err == nil && m.bus != nil {
		stop := events.Envelope{Tenant: env.Tenant, Run: env.Run, At: env.At}
		var request events.Fact
		switch state.Status {
		case filament.RunPaused:
			request = events.NewFact(events.RunPauseRequested, stop, events.RunPauseRequestedEvent{})
		case filament.RunCanceled:
			request = events.NewFact(events.RunCancelRequested, stop, events.RunCancelRequestedEvent{})
		}
		if request.Name != "" {
			if err := events.Publish(ctx, m.bus, request); err != nil {
				return fmt.Errorf("tracker: publish %s for run %q: %w", request.Name, env.Run, err)
			}
		}
	}
	return m.mutate(ctx, env, func(r *filament.RunState) {
		r.Status = filament.RunRunning
		// The store keeps the first stamp, so a redelivered fact cannot move it.
		r.StartedAt = env.At
	})
}
