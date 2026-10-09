package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/cmd/internal/connectors"
	"github.com/galaxy-io/filament/cmd/internal/health"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/streamsupervisor"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"
	"github.com/galaxy-io/filament/worker"
)

const shutdownTimeout = 10 * time.Second

// serve runs the worker long-lived as the one process that carries every
// driver. With -serve it answers connector calls for the control services,
// which needs no datastore or bus. With -execute it consumes requested runs
// from the event bus and executes them, the way a dispatched Job would, for
// deployments without Kubernetes.
func serve(ctx context.Context, serveConnectors, execute bool) error {
	sources, err := connectors.SourcesFromEnv()
	if err != nil {
		return err
	}
	var log filament.Logger
	var checks []health.Check
	if execute {
		deps, executorChecks, closeExecutor, err := startExecutor(ctx, sources)
		if err != nil {
			return err
		}
		defer closeExecutor()
		log, checks = deps.Log, executorChecks
	} else {
		lg, _, _, closeTelemetry, err := boot.Telemetry(ctx)
		if err != nil {
			return err
		}
		defer closeTelemetry()
		log = lg
	}
	log = log.With(filament.Field{Key: "component", Value: "worker"})

	var connectorWorker filament.Worker
	if serveConnectors {
		connectorWorker = worker.Local(sources, registry.DefaultSinks)
	}
	handler, healthState := workerHandler(connectorWorker, checks...)
	addr := workerAddress()
	srv := &http.Server{Addr: addr, Handler: otelhttp.NewHandler(handler, "worker"), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	healthState.MarkStarted()
	log.Info("worker started",
		filament.Field{Key: "event.name", Value: "worker.started"},
		filament.Field{Key: "address", Value: addr},
		filament.Field{Key: "serve", Value: serveConnectors},
		filament.Field{Key: "execute", Value: execute},
		filament.Field{Key: "sources", Value: len(sources.Specs())},
		filament.Field{Key: "sinks", Value: len(registry.DefaultSinks.Specs())})

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("worker listener: %w", err)
	case <-ctx.Done():
		log.Info("worker stopping", filament.Field{Key: "event.name", Value: "worker.stopping"})
		healthState.MarkStopping()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

// workerHandler keeps connector-serving readiness independent of executor
// dependencies. Execution-only workers retain their dependency checks.
func workerHandler(connectorWorker filament.Worker, checks ...health.Check) (http.Handler, *health.State) {
	if connectorWorker != nil {
		checks = nil
	}
	mux := http.NewServeMux()
	state := health.New(2*time.Second, checks...)
	state.Mount(mux)
	if connectorWorker != nil {
		mux.Handle(worker.Handler(connectorWorker))
	}
	return mux, state
}

// startExecutor mounts the engine over the datastore and event bus and
// supervises continuous attempts in this process. It returns the booted
// dependencies, the readiness checks the executor adds, and a close that
// cancels in-flight work, then releases everything in order.
func startExecutor(parent context.Context, sources filament.SourceRegistry) (boot.Deps, []health.Check, func(), error) {
	ctx, cancel := context.WithCancel(parent)
	deps, closeDeps, err := boot.FromEnv(ctx)
	if err != nil {
		cancel()
		return boot.Deps{}, nil, nil, err
	}
	deps.Sources = sources
	bus, closeBus, err := boot.Bus()
	if err != nil {
		cancel()
		closeDeps()
		return boot.Deps{}, nil, nil, err
	}
	h, err := boot.Mount(ctx, deps, bus, engine.New())
	if err != nil {
		cancel()
		closeBus()
		closeDeps()
		return boot.Deps{}, nil, nil, err
	}
	streams := streamsupervisor.New(runner.Deps{
		Bus: bus, DataStore: deps.Store, StreamStore: deps.StreamStore, Secrets: deps.Secrets,
		Sources: sources, Sinks: registry.DefaultSinks, Log: deps.Log, Tracer: deps.Tracer,
	}, nil)
	streams.Start(ctx)
	checks := []health.Check{deps.Store.Ping, func(ctx context.Context) error {
		if checker, ok := bus.(eventbus.ReadinessChecker); ok {
			return checker.Ready(ctx)
		}
		return nil
	}}
	return deps, checks, func() {
		cancel()
		streams.Close()
		_ = h.Close()
		closeBus()
		closeDeps()
	}, nil
}

func workerAddress() string {
	if addr := os.Getenv("WORKER_ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}
