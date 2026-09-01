// Command worker executes one already-persisted Filament run.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx)
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

	deps, closeDeps, err := boot.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer closeDeps()
	workerLog := deps.Log.With(
		filament.Field{Key: "component", Value: "worker"},
		filament.Field{Key: "run_id", Value: string(runID)},
	)

	bus, closeBus, err := boot.Bus()
	if err != nil {
		return err
	}
	defer closeBus()

	state, err := deps.Store.LoadRun(ctx, runID)
	if err != nil {
		return err
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
		Sources:   registry.DefaultSources,
		Sinks:     registry.DefaultSinks,
		Tracer:    deps.Tracer,
	}, runner.SpecFromState(state))
	stopHeartbeat()
	return err
}
