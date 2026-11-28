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
	signal, ok := commandSignal(data)
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
	if m.unannounced == nil {
		m.unannounced = make(map[runKey]events.Fact)
	}
	m.unannounced[key] = runcommands.Outcome(env, signal)
	m.mu.Unlock()
	return m.announce(ctx, key)
}

// commandSignal returns the signal a command fact carries.
func commandSignal(data any) (filament.Signal, bool) {
	switch data.(type) {
	case events.RunPauseCommandedEvent:
		return filament.SignalPause, true
	case events.RunResumeCommandedEvent:
		return filament.SignalResume, true
	case events.RunCancelCommandedEvent:
		return filament.SignalCancel, true
	default:
		return 0, false
	}
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
