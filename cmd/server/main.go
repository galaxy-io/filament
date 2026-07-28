// Command server runs the Filament API: it migrates the datastore and ensures
// the default tenant, serves the ConnectRPC surface and the embedded web UI,
// persists pipeline and run submissions, and publishes run.requested facts for
// the control plane to dispatch.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/eventbus"
	"github.com/galaxy-io/filament/cmd/internal/otel"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	"github.com/galaxy-io/filament/cmd/internal/secret"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/module"
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

//nolint:funlen // startup wiring reads better linear
func run(ctx context.Context, migrateOnly bool) error {
	if migrateOnly {
		return persistence.MigrateFromEnv(ctx)
	}

	store, err := persistence.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if c, ok := store.(io.Closer); ok {
			_ = c.Close()
		}
	}()
	secrets, err := secret.FromEnv(ctx, store)
	if err != nil {
		return err
	}
	metrics, tracer, otelShutdown, err := otel.FromEnv(ctx)
	if err != nil {
		return err
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = otelShutdown(flushCtx)
	}()

	// Ensure the default tenant up front so a deployment with no readiness
	// probes (local dev) still gets one; readyz retries until it lands when
	// the database is still starting.
	var tenantEnsured atomic.Bool
	if err := store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
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
		if err := store.Ping(ctx); err != nil {
			http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		if !tenantEnsured.Load() {
			if err := store.EnsureTenant(ctx, filament.TenantID(defaultTenantID()), "Default tenant"); err != nil {
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

	bus, err := eventbus.FromEnv()
	if err != nil {
		return err
	}
	defer func() {
		if c, ok := any(bus).(io.Closer); ok {
			_ = c.Close()
		}
	}()

	orch := orchestrator.New()
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Sources: registry.DefaultSources, Sinks: registry.DefaultSinks, Metrics: metrics, Tracer: tracer},
		orch,
	)
	if err != nil {
		return fmt.Errorf("mount: %w", err)
	}
	h := host.New(bus)
	defer func() {
		_ = h.Close()
		if c, ok := any(bus).(io.Closer); ok {
			_ = c.Close()
		}
	}()
	if err := h.Run(ctx, mods...); err != nil {
		return fmt.Errorf("run host: %w", err)
	}

	server.New(registry.DefaultSources, registry.DefaultSinks, store, orch, bus, server.WithSecrets(secrets)).Mount(mux)
	mux.Handle("/", ui.Handler())
	fmt.Println("server:", "http://localhost"+addr)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return srv.Close()
	}
}

func defaultTenantID() string {
	if id := os.Getenv("DEFAULT_TENANT_ID"); id != "" {
		return id
	}
	return ctlpg.DefaultTenantID
}
