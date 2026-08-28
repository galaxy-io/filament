// Command server runs the Filament API: it migrates the datastore and ensures
// the default tenant, serves the ConnectRPC surface and the embedded web UI,
// persists pipeline and run submissions, and publishes run.requested facts for
// the control plane to dispatch.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/boot"
	"github.com/galaxy-io/filament/cmd/internal/metricsstore"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
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
		log.Fatal(err)
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

	// Ensure the default tenant up front so a deployment with no readiness
	// probes (local dev) still gets one; readyz retries until it lands when
	// the database is still starting.
	var tenantEnsured atomic.Bool
	if err := deps.Store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
		log.Printf("default tenant not ensured yet: %v", err)
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
	mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := deps.Store.Ping(ctx); err != nil {
			http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		if !tenantEnsured.Load() {
			if err := deps.Store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
				http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
				return
			}
			tenantEnsured.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: addr, Handler: otelhttp.NewHandler(mux, "server"), ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	bus, closeBus, err := boot.Bus()
	if err != nil {
		return err
	}
	defer closeBus()

	metricStore, err := metricsstore.FromEnv(ctx, deps.Store)
	if err != nil {
		return err
	}
	orch := orchestrator.New()
	api := server.New(registry.DefaultSources, registry.DefaultSinks, deps.Store, orch, bus,
		server.WithSecrets(deps.Secrets), server.WithMetricsStore(metricStore), server.WithLogger(deps.Log))
	h, err := boot.Mount(ctx, deps, bus, orch)
	if err != nil {
		return err
	}
	defer func() {
		_ = h.Close()
	}()

	api.Mount(mux)
	mux.Handle("/", ui.Handler())
	fmt.Println("server:", "http://localhost"+addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

func defaultTenantID() string {
	if id := os.Getenv("DEFAULT_TENANT_ID"); id != "" {
		return id
	}
	return ctlpg.DefaultTenantID
}
