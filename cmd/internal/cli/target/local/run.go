package local

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"
)

type runSession struct {
	ref          model.RunRef
	spec         filament.RunSpec
	bus          *inproc.Bus
	subscription eventbus.Subscription
	done         <-chan error
	cancel       context.CancelFunc
}

// SubmitRun starts an in-process run and returns its target identity before
// waiting for progress. The subscription is established first so no early
// event can be lost between submission and TailRun.
func (t *Target) SubmitRun(ctx context.Context, submission model.RunSubmission) (model.RunGroup, error) {
	spec := submission.Spec
	if spec.Tenant == "" {
		spec.Tenant = "local"
	}
	if spec.Run == "" {
		spec.Run = filament.RunID("cli-" + strconv.FormatInt(time.Now().UnixNano(), 36))
	}
	source, ok := t.catalog.Sources[spec.Source.Connector]
	if !ok {
		return model.RunGroup{}, fmt.Errorf("unknown source connector %q", spec.Source.Connector)
	}
	sourceConfig, err := cliapp.ResolveConfigSecrets(source.Config, spec.Source.Config, os.LookupEnv)
	if err != nil {
		return model.RunGroup{}, fmt.Errorf("source connector %q: %w", spec.Source.Connector, err)
	}
	sink, ok := t.catalog.Sinks[spec.Sink.Connector]
	if !ok {
		return model.RunGroup{}, fmt.Errorf("unknown sink connector %q", spec.Sink.Connector)
	}
	sinkConfig, err := cliapp.ResolveConfigSecrets(sink.Config, spec.Sink.Config, os.LookupEnv)
	if err != nil {
		return model.RunGroup{}, fmt.Errorf("sink connector %q: %w", spec.Sink.Connector, err)
	}
	spec.Source.Config = sourceConfig
	spec.Sink.Config = sinkConfig
	bus := inproc.New(inproc.WithBuffer(1024))
	subscription, err := bus.Subscribe(events.RunPattern(spec.Tenant, spec.Run), eventbus.SubOpts{})
	if err != nil {
		_ = bus.Close()
		return model.RunGroup{}, err
	}

	runCtx, cancel := context.WithCancel(ctx)
	runnerDone := make(chan error, 1)
	go func() {
		runnerDone <- runner.RunOne(runCtx, runner.Deps{
			Bus:       bus,
			DataStore: memory.New(),
			Sources:   registry.DefaultSources,
			Sinks:     registry.DefaultSinks,
		}, spec)
	}()
	route := spec.PipelineID
	if route == "" {
		route = spec.Source.Connector + "->" + spec.Sink.Connector
	}
	ref := model.RunRef{ID: string(spec.Run), Route: route}
	t.runMu.Lock()
	t.runs[ref.ID] = &runSession{
		ref: ref, spec: spec, bus: bus, subscription: subscription, done: runnerDone, cancel: cancel,
	}
	t.runMu.Unlock()
	return model.RunGroup{Runs: []model.RunRef{ref}}, nil
}

// TailRun waits for one local run and reports normalized resource progress.
func (t *Target) TailRun(ctx context.Context, group model.RunGroup, observe func(model.RunEvent)) (model.RunResult, error) {
	if len(group.Runs) != 1 {
		return model.RunResult{}, fmt.Errorf("local target expected one submitted run, got %d", len(group.Runs))
	}
	ref := group.Runs[0]
	session, err := t.runSession(ref.ID)
	if err != nil {
		return model.RunResult{}, err
	}
	defer t.closeRunSession(ref.ID, session)
	runnerDone := session.done

	for {
		select {
		case message, ok := <-session.subscription.C():
			if !ok {
				return model.RunResult{}, fmt.Errorf("run event stream closed before completion")
			}
			fact, decodeErr := events.Decode(message)
			_ = message.Ack()
			if decodeErr != nil {
				return model.RunResult{}, decodeErr
			}
			switch fact.Name {
			case events.ResourceStarted.Name():
				notifyRunObserver(observe, runEvent(ref, fact.Resource, "running"))
			case events.BatchWritten.Name():
				payload := fact.Data.(events.BatchWrittenEvent)
				event := runEvent(ref, fact.Resource, "running")
				event.Records, event.Bytes = payload.Records, payload.Bytes
				notifyRunObserver(observe, event)
			case events.ResourceCompleted.Name():
				payload := fact.Data.(events.ResourceCompletedEvent)
				event := runEvent(ref, fact.Resource, "complete")
				event.Records, event.Bytes, event.Final = payload.Records, payload.Bytes, true
				notifyRunObserver(observe, event)
			case events.ResourceFailed.Name():
				payload := fact.Data.(events.ResourceFailedEvent)
				event := runEvent(ref, fact.Resource, "failed")
				event.Error = payload.Error
				notifyRunObserver(observe, event)
			case events.RunCompleted.Name():
				payload := fact.Data.(events.RunCompletedEvent)
				return model.RunResult{Runs: group.Runs, Status: "complete", Records: payload.Records, Bytes: payload.Bytes}, nil
			case events.RunFailed.Name():
				return model.RunResult{}, fmt.Errorf("run failed: %s", fact.Data.(events.RunFailedEvent).Error)
			case events.RunPartial.Name():
				return model.RunResult{}, fmt.Errorf("run partial: %s", fact.Data.(events.RunPartialEvent).Error)
			case events.RunPaused.Name():
				notifyRunObserver(observe, model.RunEvent{Run: ref.ID, Route: ref.Route, Status: "paused", Final: true})
				return model.RunResult{Runs: group.Runs, Status: "paused"}, nil
			case events.RunCanceled.Name():
				notifyRunObserver(observe, model.RunEvent{Run: ref.ID, Route: ref.Route, Status: "canceled", Final: true})
				return model.RunResult{Runs: group.Runs, Status: "canceled"}, nil
			}
		case runErr := <-runnerDone:
			if runErr != nil {
				return model.RunResult{}, runErr
			}
			runnerDone = nil
		case <-ctx.Done():
			return model.RunResult{}, ctx.Err()
		}
	}
}

// SignalRun sends a lifecycle signal to an active local run.
func (t *Target) SignalRun(ctx context.Context, ref model.RunRef, signal filament.Signal) error {
	session, err := t.runSession(ref.ID)
	if err != nil {
		return err
	}
	envelope := events.Envelope{Tenant: session.spec.Tenant, Run: session.spec.Run, At: time.Now()}
	switch signal {
	case filament.SignalPause:
		return events.Emit(ctx, session.bus, events.RunPauseRequested, envelope, events.RunPauseRequestedEvent{})
	case filament.SignalCancel:
		return events.Emit(ctx, session.bus, events.RunCancelRequested, envelope, events.RunCancelRequestedEvent{})
	case filament.SignalResume:
		return fmt.Errorf("local target does not support resuming a submitted run")
	default:
		return fmt.Errorf("unknown run signal %d", signal)
	}
}

func (t *Target) runSession(id string) (*runSession, error) {
	t.runMu.Lock()
	defer t.runMu.Unlock()
	session, ok := t.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %q is not active on the local target", id)
	}
	return session, nil
}

func (t *Target) closeRunSession(id string, session *runSession) {
	t.runMu.Lock()
	if t.runs[id] == session {
		delete(t.runs, id)
	}
	t.runMu.Unlock()
	session.cancel()
	_ = session.subscription.Close()
	_ = session.bus.Close()
}

func runEvent(ref model.RunRef, resource, status string) model.RunEvent {
	return model.RunEvent{Run: ref.ID, Route: ref.Route, Resource: resource, Status: status}
}

func notifyRunObserver(observe func(model.RunEvent), event model.RunEvent) {
	if observe != nil {
		observe(event)
	}
}
