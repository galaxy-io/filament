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
// run state. Signal returns once the tracker has published the command's
// outcome fact, so a later signal from the same caller is ordered after it.
// It returns the context's error if that ends first. The command stays on the
// bus then and is still folded.
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
		return SignalResult{}, publishCommand(ctx, bus, store, state, signal)
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

// publishCommand publishes the command fact for state and waits for the
// tracker's outcome. The subscription opens first so the outcome cannot be
// missed.
func publishCommand(
	ctx context.Context,
	bus eventbus.Bus,
	store filament.DataStore,
	state filament.RunState,
	signal filament.Signal,
) error {
	sub, err := bus.Subscribe(events.RunPattern(state.Tenant, state.Run), eventbus.SubOpts{})
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSignalPublish, err)
	}
	defer func() { _ = sub.Close() }()
	command := commandFact(state, signal)
	if err := events.Publish(ctx, bus, command); err != nil {
		return fmt.Errorf("%w: %v", ErrSignalPublish, err)
	}
	return awaitOutcome(ctx, store, sub, state, signal, Outcome(command.Envelope, signal))
}

// maxFoldPoll caps the interval between reads while Signal waits for a fold.
const maxFoldPoll = 250 * time.Millisecond

// awaitOutcome waits for the outcome fact of this command, matched by name and
// the command's emit time. Between deliveries it reads the run, because a
// command the tracker skips publishes nothing. It reports a conflict once the
// run is in a status the command cannot apply from, since the tracker plans
// from the status it loads.
func awaitOutcome(
	ctx context.Context,
	store filament.DataStore,
	sub eventbus.Subscription,
	state filament.RunState,
	signal filament.Signal,
	outcome events.Fact,
) error {
	// The fold usually lands within milliseconds, but a tracker working
	// through a backlog can take seconds, so the reads back off.
	wait := 10 * time.Millisecond
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-sub.C():
			if !ok {
				return fmt.Errorf("%w: run event stream closed", ErrSignalPublish)
			}
			fact, err := events.Decode(msg)
			_ = msg.Ack()
			if err == nil && fact.Name == outcome.Name && fact.At.Equal(outcome.At) {
				return nil
			}
			continue
		case <-timer.C:
		}
		current, err := store.LoadRun(ctx, state.Tenant, state.Run)
		if err != nil {
			return err
		}
		if skipped(current, signal) {
			return fmt.Errorf("run moved to status %d before signal %d applied: %w", current.Status, signal, filament.ErrVersionConflict)
		}
		timer.Reset(wait)
		wait = min(2*wait, maxFoldPoll)
	}
}

// skipped reports that the tracker can no longer apply signal's command to a
// run in current's status: the command does not apply from there and the run
// has not reached the command's target, or for a resume moved past it.
func skipped(current filament.RunState, signal filament.Signal) bool {
	if _, ok := PlanTransition(current, signal); ok {
		return false
	}
	switch signal {
	case filament.SignalPause:
		return current.Status != filament.RunPaused
	case filament.SignalCancel:
		return current.Status != filament.RunCanceled
	default:
		return current.Status != filament.RunRequested && !resumedPast(current.Status)
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
// notifier act on. A pause or cancel outcome is marked Commanded, because the
// command fold already stored its status. It reuses the command's envelope so
// a republished outcome keeps its message ID and the broker drops the
// duplicate, and so Signal can tell its own outcome apart.
func Outcome(env events.Envelope, signal filament.Signal) events.Fact {
	switch signal {
	case filament.SignalPause:
		return events.NewFact(events.RunPaused, env, events.RunPausedEvent{Commanded: true})
	case filament.SignalCancel:
		return events.NewFact(events.RunCanceled, env, events.RunCanceledEvent{Commanded: true})
	default:
		return signalFact(filament.RunState{Tenant: env.Tenant, Run: env.Run}, env.At, signal, false)
	}
}

// CommandSignal returns the signal a command fact carries, the inverse of the
// command Signal publishes.
func CommandSignal(data any) (filament.Signal, bool) {
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
