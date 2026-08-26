// Command worker executes one already-persisted Filament run.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
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
		log.Fatal(err)
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
		deps.Log.Info("worker: nothing to do",
			filament.Field{Key: "run", Value: string(runID)},
			filament.Field{Key: "status", Value: int(state.Status)})
		return nil
	}
	deps.Log.Info("worker: executing run",
		filament.Field{Key: "run", Value: string(runID)},
		filament.Field{Key: "pipeline", Value: state.Request.PipelineID},
		filament.Field{Key: "source", Value: state.Request.Source.Provider},
		filament.Field{Key: "sink", Value: state.Request.Sink.Provider},
		filament.Field{Key: "resources", Value: len(state.Request.Resources)})

	hb := &heartbeat{
		bus:      bus,
		mx:       deps.Metrics,
		log:      deps.Log,
		tenant:   state.Tenant,
		run:      state.Run,
		pipeline: state.Request.PipelineID,
	}
	stopHeartbeat := hb.start(ctx, heartbeatInterval())

	runner.RunOne(ctx, runner.Deps{
		Bus:       bus,
		DataStore: deps.Store,
		Log:       deps.Log,
		Secrets:   deps.Secrets,
		Sources:   registry.DefaultSources,
		Sinks:     registry.DefaultSinks,
		Tracer:    deps.Tracer,
	}, runner.SpecFromState(state))
	stopHeartbeat()
	return nil
}
