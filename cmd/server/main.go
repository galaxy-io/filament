// Command server runs the Filament API: it migrates the datastore and ensures
// the default tenant, serves the ConnectRPC surface and the embedded web UI,
// persists pipeline and run submissions, and publishes run.requested facts for
// the control plane to dispatch.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/cmd/internal/health"
	"github.com/galaxy-io/filament/cmd/internal/metricsstore"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
	"github.com/galaxy-io/filament/ui"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
)

func main() {
	migrateOnly := flag.Bool("migrate", false, "run datastore migrations and exit")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, *migrateOnly)
	stop()
	if err != nil {
		slog.Error("server exited",
			"event.name", "server.exited",
			"error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, migrateOnly bool) error {
	if migrateOnly {
		return persistence.MigrateFromEnv(ctx)
	}

	deps, closeDeps, err := boot.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer closeDeps()
	serverLog := deps.Log.With(filament.Field{Key: "component", Value: "server"})

	// Ensure the default tenant up front so a deployment with no readiness
	// probes (local dev) still gets one; readyz retries until it lands when
	// the database is still starting.
	var tenantEnsured atomic.Bool
	if err := deps.Store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
		serverLog.Warn("default tenant not ensured; readiness will retry",
			filament.Field{Key: "event.name", Value: "server.default_tenant.ensure_deferred"},
			filament.Field{Key: "error", Value: err.Error()})
	} else {
		tenantEnsured.Store(true)
	}

	// Health endpoints listen before the NATS connect wait so liveness
	// probes answer while dependencies are still starting.
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	var eventBus eventbus.Bus
	healthState := health.New(2*time.Second,
		func(ctx context.Context) error {
			if err := deps.Store.Ping(ctx); err != nil {
				return err
			}
			if !tenantEnsured.Load() {
				if err := deps.Store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
					return err
				}
				tenantEnsured.Store(true)
			}
			return nil
		},
		func(ctx context.Context) error {
			if checker, ok := eventBus.(eventbus.ReadinessChecker); ok {
				return checker.Ready(ctx)
			}
			return nil
		},
	)
	healthState.Mount(mux)
	srv := &http.Server{Addr: addr, Handler: otelhttp.NewHandler(mux, "server"), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	var shutdownOnce sync.Once
	var shutdownErr error
	started := false
	shutdown := func() error {
		shutdownOnce.Do(func() {
			healthState.MarkStopping()
			shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			shutdownErr = srv.Shutdown(shutCtx)
			if shutdownErr != nil {
				serverLog.Error("server shutdown failed", shutdownErr,
					filament.Field{Key: "event.name", Value: "server.shutdown_failed"})
				return
			}
			if started {
				serverLog.Info("server stopped",
					filament.Field{Key: "event.name", Value: "server.stopped"})
			}
		})
		return shutdownErr
	}
	defer func() { _ = shutdown() }()

	eventBus, closeBus, err := boot.Bus()
	if err != nil {
		return err
	}
	defer closeBus()

	metricStore, err := metricsstore.FromEnv(ctx, deps.Store)
	if err != nil {
		return err
	}
	orch := orchestrator.New()
	api := server.New(registry.DefaultSources, registry.DefaultSinks, deps.Store, orch, eventBus,
		server.WithSecrets(deps.Secrets), server.WithMetricsStore(metricStore), server.WithLogger(deps.Log))
	h, err := boot.Mount(ctx, deps, eventBus, orch)
	if err != nil {
		return err
	}
	defer func() {
		if err := h.Close(); err != nil {
			serverLog.Warn("server host close failed",
				filament.Field{Key: "event.name", Value: "server.host.close_failed"},
				filament.Field{Key: "error", Value: err.Error()})
		}
	}()

	api.Mount(mux)
	mux.Handle("/", ui.Handler())
	healthState.MarkStarted()
	started = true
	serverLog.Info("server started",
		filament.Field{Key: "event.name", Value: "server.started"},
		filament.Field{Key: "address", Value: addr},
		filament.Field{Key: "modules", Value: h.Mounted()})

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("server listener: %w", err)
	case <-ctx.Done():
		serverLog.Info("server stopping",
			filament.Field{Key: "event.name", Value: "server.stopping"})
		return shutdown()
	}
}

func defaultTenantID() string {
	if id := os.Getenv("DEFAULT_TENANT_ID"); id != "" {
		return id
	}
	return ctlpg.DefaultTenantID
}
