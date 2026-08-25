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
	// ErrSignalPublish distinguishes a durable transition whose notification
	// could not be placed on the bus. Retrying the command repairs publication.
	ErrSignalPublish = errors.New("publish run signal")
)

type signalCommand struct {
	from           []filament.RunStatus
	to             filament.RunStatus
	resetExecution bool
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
	{filament.SignalPause, filament.RunRequested}: {
		from: []filament.RunStatus{filament.RunRequested}, to: filament.RunPaused,
		transition: true, publish: true,
	},
	{filament.SignalPause, filament.RunPaused}: {},
	{filament.SignalPause, filament.RunRunning}: {
		publish: true, worker: true,
	},
	{filament.SignalResume, filament.RunRequested}: {publish: true}, // repair dispatch
	{filament.SignalResume, filament.RunPaused}: {
		from: []filament.RunStatus{filament.RunPaused, filament.RunPartial}, to: filament.RunRequested,
		resetExecution: true, transition: true, publish: true,
	},
	{filament.SignalResume, filament.RunPartial}: {
		from: []filament.RunStatus{filament.RunPaused, filament.RunPartial}, to: filament.RunRequested,
		resetExecution: true, transition: true, publish: true,
	},
	{filament.SignalCancel, filament.RunRequested}: {
		from: []filament.RunStatus{filament.RunRequested, filament.RunPaused, filament.RunPartial},
		to:   filament.RunCanceled, transition: true, publish: true,
	},
	{filament.SignalCancel, filament.RunPaused}: {
		from: []filament.RunStatus{filament.RunRequested, filament.RunPaused, filament.RunPartial},
		to:   filament.RunCanceled, transition: true, publish: true,
	},
	{filament.SignalCancel, filament.RunPartial}: {
		from: []filament.RunStatus{filament.RunRequested, filament.RunPaused, filament.RunPartial},
		to:   filament.RunCanceled, transition: true, publish: true,
	},
	{filament.SignalCancel, filament.RunCanceled}: {},
	{filament.SignalCancel, filament.RunRunning}: {
		publish: true, worker: true,
	},
}

// Signal plans and executes one lifecycle command. Transition policy belongs
// here rather than in an RPC transport: every caller gets the same idempotency,
// compare-and-swap, reset, and event publication semantics.
func Signal(
	ctx context.Context,
	bus eventbus.Bus,
	store filament.RunTransitionStore,
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
		preserveProgress := command.resetExecution && state.Status == filament.RunPaused &&
			filament.CheckpointCoverageFor(state.Request.Resources, state.Request.IngestionTypes) == filament.CheckpointCoverageAll
		state, err = store.TransitionRun(ctx, state.Run, command.from, command.to, filament.RunTransitionOptions{
			ResetExecution: command.resetExecution, PreserveProgress: preserveProgress,
		})
		if err != nil {
			return SignalResult{}, err
		}
	}
	if !command.publish {
		return SignalResult{WorkerAcknowledgement: command.worker}, nil
	}
	fact := signalFact(state, signal, command.worker)
	if err := events.Publish(ctx, bus, fact); err != nil {
		return SignalResult{}, fmt.Errorf("%w: %v", ErrSignalPublish, err)
	}
	return SignalResult{WorkerAcknowledgement: command.worker}, nil
}

func planSignal(status filament.RunStatus, signal filament.Signal) (signalCommand, error) {
	command, ok := signalCommands[signalKey{signal: signal, status: status}]
	if !ok {
		return signalCommand{}, fmt.Errorf("%w: signal %d from status %d", ErrSignalTransition, signal, status)
	}
	return command, command.err
}

func signalFact(state filament.RunState, signal filament.Signal, worker bool) events.Fact {
	env := events.Envelope{Tenant: state.Tenant, Run: state.Run, At: time.Now()}
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
