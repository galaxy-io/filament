package local

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/engine"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/events"
)

type runSession struct {
	ref model.RunRef
	run *engine.InlineRun
}

// submitLocal starts an untracked run in the local engine and returns its
// identity before waiting for progress.
func (t *Target) submitLocal(ctx context.Context, spec filament.RunSpec) (model.RunGroup, error) {
	catalog, err := t.Catalog(ctx)
	if err != nil {
		return model.RunGroup{}, err
	}
	if spec.Tenant == "" {
		spec.Tenant = "local"
	}
	if spec.Run == "" {
		spec.Run = filament.RunID("cli-" + strconv.FormatInt(time.Now().UnixNano(), 36))
	}
	source, ok := catalog.Sources[spec.Source.Connector]
	if !ok {
		return model.RunGroup{}, fmt.Errorf("unknown source connector %q", spec.Source.Connector)
	}
	sourceConfig, err := cliapp.ResolveConfigSecrets(source.Config, spec.Source.Config, os.LookupEnv)
	if err != nil {
		return model.RunGroup{}, fmt.Errorf("source connector %q: %w", spec.Source.Connector, err)
	}
	sink, ok := catalog.Sinks[spec.Sink.Connector]
	if !ok {
		return model.RunGroup{}, fmt.Errorf("unknown sink connector %q", spec.Sink.Connector)
	}
	sinkConfig, err := cliapp.ResolveConfigSecrets(sink.Config, spec.Sink.Config, os.LookupEnv)
	if err != nil {
		return model.RunGroup{}, fmt.Errorf("sink connector %q: %w", spec.Sink.Connector, err)
	}
	spec.Source.Config = sourceConfig
	spec.Sink.Config = sinkConfig
	if t.engine == nil {
		return model.RunGroup{}, fmt.Errorf("local target has no engine for inline runs")
	}
	run, err := t.engine.Inline(ctx, spec, os.Stdout, os.Stderr)
	if err != nil {
		return model.RunGroup{}, err
	}
	route := spec.PipelineID
	if route == "" {
		route = spec.Source.Connector + "->" + spec.Sink.Connector
	}
	ref := model.RunRef{ID: string(spec.Run), Route: route}
	t.runMu.Lock()
	t.runs[ref.ID] = &runSession{ref: ref, run: run}
	t.runMu.Unlock()
	return model.RunGroup{Runs: []model.RunRef{ref}}, nil
}

// tailLocal waits for one direct run and reports normalized progress.
func (t *Target) tailLocal(ctx context.Context, group model.RunGroup, observe func(model.RunEvent)) (model.RunResult, error) {
	if len(group.Runs) != 1 {
		return model.RunResult{}, fmt.Errorf("local target expected one submitted run, got %d", len(group.Runs))
	}
	ref := group.Runs[0]
	session, err := t.runSession(ref.ID)
	if err != nil {
		return model.RunResult{}, err
	}
	defer t.closeRunSession(ref.ID, session)
	runnerDone := session.run.Done()
	// The exit error is reported only when the facts end without a terminal
	// one, since a failed run exits as its run.failed fact is still queued.
	var exitErr error

	for {
		select {
		case fact, ok := <-session.run.Facts():
			if !ok {
				if exitErr != nil {
					return model.RunResult{}, exitErr
				}
				return model.RunResult{}, fmt.Errorf("run event stream closed before completion")
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
		case exitErr = <-runnerDone:
			runnerDone = nil
		case <-ctx.Done():
			return model.RunResult{}, ctx.Err()
		}
	}
}

// signalLocal sends a lifecycle signal to an active direct run.
func (t *Target) signalLocal(ref model.RunRef, signal filament.Signal) error {
	session, err := t.runSession(ref.ID)
	if err != nil {
		return err
	}
	switch signal {
	case filament.SignalPause:
		return session.run.Request("pause")
	case filament.SignalCancel:
		return session.run.Request("cancel")
	case filament.SignalResume:
		return fmt.Errorf("local target does not support resuming a submitted run")
	default:
		return fmt.Errorf("unknown run signal %d", signal)
	}
}

func (t *Target) hasSession(id string) bool {
	t.runMu.Lock()
	defer t.runMu.Unlock()
	_, ok := t.runs[id]
	return ok
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
	_ = session.run.Close()
}

func runEvent(ref model.RunRef, resource, status string) model.RunEvent {
	return model.RunEvent{Run: ref.ID, Route: ref.Route, Resource: resource, Status: status}
}

func notifyRunObserver(observe func(model.RunEvent), event model.RunEvent) {
	if observe != nil {
		observe(event)
	}
}
