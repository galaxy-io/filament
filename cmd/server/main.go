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
	"github.com/galaxy-io/filament/cmd/internal/identity"
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

	// A nil provider means auth is disabled: the API stays unauthenticated
	// and the UI renders without a session.
	identityProvider, err := identity.FromEnv(ctx)
	if err != nil {
		return err
	}

	tenantEnsured := ensureDefaultTenant(ctx, deps.Store, serverLog)

	// Health endpoints listen before the NATS connect wait so liveness
	// probes answer while dependencies are still starting.
	addr := serverAddress()
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
	started := false
	shutdown := serverShutdown(srv, healthState, serverLog, &started)
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
	apiOpts := []server.Option{
		server.WithSecrets(deps.Secrets),
		server.WithMetricsStore(metricStore),
		server.WithLogger(deps.Log),
	}
	if identityProvider != nil {
		apiOpts = append(apiOpts, server.WithIdentity(identityProvider))
	}
	api := server.New(registry.DefaultSources, registry.DefaultSinks, deps.Store, orch, eventBus, apiOpts...)
	h, err := boot.Mount(ctx, deps, eventBus, orch)
	if err != nil {
		return err
	}
	defer func() { logHostClose(h.Close(), serverLog) }()

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

func logHostClose(err error, log filament.Logger) {
	if err != nil {
		log.Warn("server host close failed",
			filament.Field{Key: "event.name", Value: "server.host.close_failed"},
			filament.Field{Key: "error", Value: err.Error()})
	}
}

// ensureDefaultTenant makes one eager attempt; readiness retries while the
// datastore is still starting.
func ensureDefaultTenant(ctx context.Context, store filament.DataStore, log filament.Logger) *atomic.Bool {
	ensured := &atomic.Bool{}
	if err := store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
		log.Warn("default tenant not ensured; readiness will retry",
			filament.Field{Key: "event.name", Value: "server.default_tenant.ensure_deferred"},
			filament.Field{Key: "error", Value: err.Error()})
		return ensured
	}
	ensured.Store(true)
	return ensured
}

func serverAddress() string {
	if addr := os.Getenv("SERVER_ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}

func serverShutdown(srv *http.Server, healthState *health.State, log filament.Logger, started *bool) func() error {
	var once sync.Once
	var shutdownErr error
	return func() error {
		once.Do(func() {
			healthState.MarkStopping()
			shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			shutdownErr = srv.Shutdown(shutCtx)
			if shutdownErr != nil {
				log.Error("server shutdown failed", shutdownErr,
					filament.Field{Key: "event.name", Value: "server.shutdown_failed"})
				return
			}
			if *started {
				log.Info("server stopped",
					filament.Field{Key: "event.name", Value: "server.stopped"})
			}
		})
		return shutdownErr
	}
}

func defaultTenantID() string {
	if id := os.Getenv("DEFAULT_TENANT_ID"); id != "" {
		return id
	}
	return ctlpg.DefaultTenantID
}
