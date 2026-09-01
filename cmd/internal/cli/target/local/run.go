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

// Run executes a run in-process and reports normalized resource progress.
func (t *Target) Run(ctx context.Context, spec filament.RunSpec, observe func(model.RunEvent)) (model.RunResult, error) {
	if spec.Tenant == "" {
		spec.Tenant = "local"
	}
	if spec.Run == "" {
		spec.Run = filament.RunID("cli-" + strconv.FormatInt(time.Now().UnixNano(), 36))
	}
	source, ok := t.catalog.Sources[spec.Source.Connector]
	if !ok {
		return model.RunResult{}, fmt.Errorf("unknown source connector %q", spec.Source.Connector)
	}
	sourceConfig, err := cliapp.ResolveConfigSecrets(source.Config, spec.Source.Config, os.LookupEnv)
	if err != nil {
		return model.RunResult{}, fmt.Errorf("source connector %q: %w", spec.Source.Connector, err)
	}
	sink, ok := t.catalog.Sinks[spec.Sink.Connector]
	if !ok {
		return model.RunResult{}, fmt.Errorf("unknown sink connector %q", spec.Sink.Connector)
	}
	sinkConfig, err := cliapp.ResolveConfigSecrets(sink.Config, spec.Sink.Config, os.LookupEnv)
	if err != nil {
		return model.RunResult{}, fmt.Errorf("sink connector %q: %w", spec.Sink.Connector, err)
	}
	spec.Source.Config = sourceConfig
	spec.Sink.Config = sinkConfig
	bus := inproc.New(inproc.WithBuffer(1024))
	defer func() { _ = bus.Close() }()
	subscription, err := bus.Subscribe(events.RunPattern(spec.Tenant, spec.Run), eventbus.SubOpts{})
	if err != nil {
		return model.RunResult{}, err
	}
	defer func() { _ = subscription.Close() }()

	runnerDone := make(chan error, 1)
	go func() {
		runnerDone <- runner.RunOne(ctx, runner.Deps{
			Bus:       bus,
			DataStore: memory.New(),
			Sources:   registry.DefaultSources,
			Sinks:     registry.DefaultSinks,
		}, spec)
	}()

	for {
		select {
		case message, ok := <-subscription.C():
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
				notifyRunObserver(observe, model.RunEvent{Resource: fact.Resource, Status: "running"})
			case events.BatchWritten.Name():
				payload := fact.Data.(events.BatchWrittenEvent)
				notifyRunObserver(observe, model.RunEvent{
					Resource: fact.Resource, Status: "running", Records: payload.Records, Bytes: payload.Bytes,
				})
			case events.ResourceCompleted.Name():
				payload := fact.Data.(events.ResourceCompletedEvent)
				notifyRunObserver(observe, model.RunEvent{
					Resource: fact.Resource, Status: "complete", Records: payload.Records, Bytes: payload.Bytes, Final: true,
				})
			case events.ResourceFailed.Name():
				payload := fact.Data.(events.ResourceFailedEvent)
				notifyRunObserver(observe, model.RunEvent{Resource: fact.Resource, Status: "failed", Error: payload.Error})
			case events.RunCompleted.Name():
				payload := fact.Data.(events.RunCompletedEvent)
				return model.RunResult{Run: string(spec.Run), Records: payload.Records, Bytes: payload.Bytes}, nil
			case events.RunFailed.Name():
				return model.RunResult{}, fmt.Errorf("run failed: %s", fact.Data.(events.RunFailedEvent).Error)
			case events.RunPartial.Name():
				return model.RunResult{}, fmt.Errorf("run partial: %s", fact.Data.(events.RunPartialEvent).Error)
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

func notifyRunObserver(observe func(model.RunEvent), event model.RunEvent) {
	if observe != nil {
		observe(event)
	}
}
