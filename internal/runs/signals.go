package runs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
)

var (
	// ErrSignalTransition means the command is not admitted from the run's
	// current lifecycle state.
	ErrSignalTransition = errors.New("run signal is not valid from the current status")
	// ErrSignalPublish means the signal's fact could not be placed on the
	// bus, so nothing changed. Retrying the command is safe.
	ErrSignalPublish = errors.New("publish run signal")
)

// signalCommand is what one signal does from one status. A transition is
// folded by the tracker from a command fact. Otherwise publish puts the
// signal's fact on the bus directly, and worker means a running worker must
// acknowledge it.
type signalCommand struct {
	to             filament.RunStatus
	resetExecution bool
	ended          bool
	publish        bool
	transition     bool
	worker         bool
	err            error
}

// SignalResult describes work that remains after a command is published.
type SignalResult struct {
	WorkerAcknowledgement bool
}

type signalKey struct {
	signal filament.Signal
	status filament.RunStatus
}

var signalCommands = map[signalKey]signalCommand{
	{filament.SignalPause, filament.RunRequested}:  {to: filament.RunPaused, transition: true},
	{filament.SignalPause, filament.RunPaused}:     {},
	{filament.SignalPause, filament.RunRunning}:    {publish: true, worker: true},
	{filament.SignalResume, filament.RunRequested}: {publish: true}, // repair dispatch
	{filament.SignalResume, filament.RunPaused}:    {to: filament.RunRequested, resetExecution: true, transition: true},
	{filament.SignalResume, filament.RunPartial}:   {to: filament.RunRequested, resetExecution: true, transition: true},
	{filament.SignalCancel, filament.RunRequested}: {to: filament.RunCanceled, ended: true, transition: true},
	{filament.SignalCancel, filament.RunPaused}:    {to: filament.RunCanceled, ended: true, transition: true},
	{filament.SignalCancel, filament.RunPartial}:   {to: filament.RunCanceled, ended: true, transition: true},
	{filament.SignalCancel, filament.RunCanceled}:  {},
	{filament.SignalCancel, filament.RunRunning}:   {publish: true, worker: true},
}

// Signal plans and executes one lifecycle command. Transition policy belongs
// here rather than in an RPC transport: every caller gets the same idempotency,
// fold, reset, and event publication semantics.
//
// Signal never writes run state. A transition of a run that is not executing
// is published as a command fact and folded by the tracker, the only writer of
// run state. Signal returns once the fold is visible in store, or with the
// context's error if it ends first. The command stays on the bus then and is
// still folded.
func Signal(
	ctx context.Context,
	bus eventbus.Bus,
	store filament.DataStore,
	state filament.RunState,
	signal filament.Signal,
) (SignalResult, error) {
	command, err := planSignal(state.Status, signal)
	if err != nil {
		return SignalResult{}, err
	}
	if command.worker && signal == filament.SignalPause &&
		filament.CheckpointCoverageFor(state.Request.Resources, state.Request.IngestionTypes) == filament.CheckpointCoverageSome {
		return SignalResult{}, fmt.Errorf("%w: pause requires either all or no resources to be checkpointable", ErrSignalTransition)
	}
	if command.transition {
		if err := events.Publish(ctx, bus, commandFact(state, signal)); err != nil {
			return SignalResult{}, fmt.Errorf("%w: %v", ErrSignalPublish, err)
		}
		return SignalResult{}, awaitFold(ctx, store, state, signal, command.to)
	}
	if !command.publish {
		return SignalResult{WorkerAcknowledgement: command.worker}, nil
	}
	fact := signalFact(state, time.Now(), signal, command.worker)
	if err := events.Publish(ctx, bus, fact); err != nil {
		return SignalResult{}, fmt.Errorf("%w: %v", ErrSignalPublish, err)
	}
	return SignalResult{WorkerAcknowledgement: command.worker}, nil
}

// awaitFold waits for the tracker to fold the command published for state.
// The fold moves the run out of the status Signal planned from, either to the
// command's target or, when another fact moved the run first, to a status the
// command does not apply from.
func awaitFold(
	ctx context.Context,
	store filament.DataStore,
	state filament.RunState,
	signal filament.Signal,
	to filament.RunStatus,
) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := store.LoadRun(ctx, state.Tenant, state.Run)
		if err != nil {
			return err
		}
		if current.Status != state.Status {
			if current.Status == to || (signal == filament.SignalResume && resumedPast(current.Status)) {
				return nil
			}
			return fmt.Errorf("run moved to status %d before signal %d applied: %w", current.Status, signal, filament.ErrVersionConflict)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// resumedPast reports a status a resumed run can reach once dispatched, before
// Signal next reads it.
func resumedPast(status filament.RunStatus) bool {
	switch status {
	case filament.RunRunning, filament.RunCompleted, filament.RunFailed, filament.RunPartial:
		return true
	default:
		return false
	}
}

// Transition is the change a folded command makes to the stored run.
type Transition struct {
	From    filament.RunStatus
	To      filament.RunStatus
	Options filament.RunTransitionOptions
}

// PlanTransition decides the transition signal makes from the run's status
// when the tracker folds its command. It reports false when the signal does
// not change the status from there, such as a cancel folded after the run
// started.
func PlanTransition(state filament.RunState, signal filament.Signal) (Transition, bool) {
	command, err := planSignal(state.Status, signal)
	if err != nil || !command.transition {
		return Transition{}, false
	}
	preserveProgress := command.resetExecution && state.Status == filament.RunPaused &&
		filament.CheckpointCoverageFor(state.Request.Resources, state.Request.IngestionTypes) == filament.CheckpointCoverageAll
	return Transition{From: state.Status, To: command.to, Options: filament.RunTransitionOptions{
		ResetExecution: command.resetExecution, PreserveProgress: preserveProgress, Ended: command.ended,
	}}, true
}

// Outcome is the fact the tracker publishes after it folds a command: the
// same fact the run would publish for the transition, which dispatch and the
// notifier act on. It reuses the command's envelope so a republished outcome
// keeps its message ID and the broker drops the duplicate.
func Outcome(env events.Envelope, signal filament.Signal) events.Fact {
	return signalFact(filament.RunState{Tenant: env.Tenant, Run: env.Run}, env.At, signal, false)
}

func commandFact(state filament.RunState, signal filament.Signal) events.Fact {
	env := events.Envelope{Tenant: state.Tenant, Run: state.Run, At: time.Now()}
	switch signal {
	case filament.SignalPause:
		return events.NewFact(events.RunPauseCommanded, env, events.RunPauseCommandedEvent{})
	case filament.SignalResume:
		return events.NewFact(events.RunResumeCommanded, env, events.RunResumeCommandedEvent{})
	case filament.SignalCancel:
		return events.NewFact(events.RunCancelCommanded, env, events.RunCancelCommandedEvent{})
	default:
		panic("runs: commandFact called with invalid signal")
	}
}

func planSignal(status filament.RunStatus, signal filament.Signal) (signalCommand, error) {
	command, ok := signalCommands[signalKey{signal: signal, status: status}]
	if !ok {
		return signalCommand{}, fmt.Errorf("%w: signal %d from status %d", ErrSignalTransition, signal, status)
	}
	return command, command.err
}

func signalFact(state filament.RunState, at time.Time, signal filament.Signal, worker bool) events.Fact {
	env := events.Envelope{Tenant: state.Tenant, Run: state.Run, At: at}
	switch signal {
	case filament.SignalPause:
		if worker {
			return events.NewFact(events.RunPauseRequested, env, events.RunPauseRequestedEvent{})
		}
		return events.NewFact(events.RunPaused, env, events.RunPausedEvent{})
	case filament.SignalResume:
		return events.NewFact(events.RunRequested, env, events.RunRequestedEvent{})
	case filament.SignalCancel:
		if worker {
			return events.NewFact(events.RunCancelRequested, env, events.RunCancelRequestedEvent{})
		}
		return events.NewFact(events.RunCanceled, env, events.RunCanceledEvent{})
	default:
		panic("runs: signalFact called with invalid signal")
	}
}
