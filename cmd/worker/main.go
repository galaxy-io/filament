// Command worker executes one already-persisted Filament run, or runs
// long-lived: -serve answers connector calls for the control services and
// -execute consumes requested runs from the event bus.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"

	"github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	serveMode := flag.Bool("serve", false, "run long-lived, answering connector calls for the control services")
	execute := flag.Bool("execute", false, "run long-lived, executing requested runs from the event bus")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	var err error
	switch {
	case (*serveMode || *execute) && os.Getenv("RUN_ID") != "":
		err = errors.New("-serve and -execute are exclusive with RUN_ID: a dispatched run executes in its own process")
	case *serveMode || *execute:
		err = serve(ctx, *serveMode, *execute)
	default:
		err = run(ctx)
	}
	stop()
	if err != nil {
		slog.Error("worker exited",
			"event.name", "worker.exited",
			"run_id", os.Getenv("RUN_ID"),
			"error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	runID := filament.RunID(os.Getenv("RUN_ID"))
	if runID == "" {
		return errors.New("RUN_ID is required")
	}
	if err := runID.Valid(); err != nil {
		return fmt.Errorf("RUN_ID: %w", err)
	}
	tenant := filament.TenantID(os.Getenv("TENANT_ID"))
	if err := tenant.Valid(); err != nil {
		return fmt.Errorf("TENANT_ID: %w", err)
	}

	deps, closeDeps, err := boot.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer closeDeps()
	if deps.Sources, err = connectors.SourcesFromEnv(); err != nil {
		return err
	}
	workerLog := deps.Log.With(
		filament.Field{Key: "component", Value: "worker"},
		filament.Field{Key: "run_id", Value: string(runID)},
	)

	bus, closeBus, err := boot.Bus()
	if err != nil {
		return err
	}
	defer closeBus()

	state, err := deps.Store.LoadRun(ctx, tenant, runID)
	if err != nil {
		return err
	}
	if state.Request.Options.Execution.Normalize() == filament.ExecutionContinuous {
		runtime := deps.StreamStore
		if runtime == nil {
			return filament.ErrContinuousDisabled
		}
		spec, err := runner.LoadContinuousAttempt(ctx, runtime, state, os.Getenv("EXECUTION_ID"))
		if err != nil {
			return err
		}
		return runner.ExecuteContinuousAttempt(ctx, runner.Deps{Bus: bus, DataStore: deps.Store, StreamStore: deps.StreamStore, Secrets: deps.Secrets, Sources: deps.Sources, Sinks: registry.DefaultSinks, Log: deps.Log}, spec)
	}
	if !runner.ShouldRun(state) {
		workerLog.Info("worker run skipped",
			filament.Field{Key: "event.name", Value: "worker.run.skipped"},
			filament.Field{Key: "status", Value: int(state.Status)},
			filament.Field{Key: "reason", Value: "not_runnable"})
		return nil
	}
	workerLog.Debug("worker initialized",
		filament.Field{Key: "event.name", Value: "worker.initialized"},
		filament.Field{Key: "tenant_id", Value: string(state.Tenant)},
		filament.Field{Key: "pipeline_id", Value: state.Request.PipelineID},
		filament.Field{Key: "source_connector", Value: state.Request.Source.Connector},
		filament.Field{Key: "sink_connector", Value: state.Request.Sink.Connector},
		filament.Field{Key: "resource_count", Value: len(state.Request.Resources)})

	hb := &heartbeat{
		bus: bus,
		mx:  deps.Metrics,
		log: deps.Log.With(
			filament.Field{Key: "component", Value: "heartbeat"},
			filament.Field{Key: "run_id", Value: string(runID)},
		),
		tenant:   state.Tenant,
		run:      state.Run,
		pipeline: state.Request.PipelineID,
	}
	stopHeartbeat := hb.start(ctx, heartbeatInterval())

	err = runner.RunOne(ctx, runner.Deps{
		Bus:       bus,
		DataStore: deps.Store,
		Log:       deps.Log,
		Secrets:   deps.Secrets,
		Sources:   deps.Sources,
		Sinks:     registry.DefaultSinks,
		Tracer:    deps.Tracer,
	}, runner.SpecFromState(state))
	stopHeartbeat()
	return err
}
