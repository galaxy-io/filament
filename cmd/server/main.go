// Command server runs the Filament API: it migrates the datastore and ensures
// the default tenant, serves the ConnectRPC surface and the embedded web UI,
// persists pipeline and run submissions, and publishes run.requested facts for
// the control plane to dispatch.
package main

import (
	"context"
	"database/sql"
	"errors"
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

	ingestion "github.com/galaxy-io/filament"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/secret"
	"github.com/galaxy-io/filament/server"
	"github.com/galaxy-io/filament/ui"

	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
	_ "github.com/galaxy-io/filament/connectors/sample"
	_ "github.com/galaxy-io/filament/connectors/stdout"
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
	persistenceDSN := os.Getenv("PERSISTENCE_DSN")
	if persistenceDSN == "" {
		return errors.New("PERSISTENCE_DSN is required")
	}

	if migrateOnly {
		db, err := ctlpg.NewSQLDB(persistenceDSN)
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		if err := waitForDB(ctx, db); err != nil {
			return err
		}
		return ctlpg.Migrate(db)
	}

	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return errors.New("NATS_URL is required")
	}

	pool, err := ctlpg.NewPool(ctx, persistenceDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := ctlpg.New(pool)
	secrets, err := secret.FromEnv(pool)
	if err != nil {
		return err
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
	var tenantEnsured atomic.Bool
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
			return
		}
		if !tenantEnsured.Load() {
			if err := store.EnsureTenant(ctx, ingestion.TenantID(defaultTenantID()), "Default tenant"); err != nil {
				http.Error(w, "not ready: "+err.Error(), http.StatusServiceUnavailable)
				return
			}
			tenantEnsured.Store(true)
		}
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()

	busOpts := []natsbus.Option{}
	if stream := os.Getenv("NATS_STREAM"); stream != "" {
		busOpts = append(busOpts, natsbus.WithStream(stream))
	}
	if subjects := os.Getenv("NATS_SUBJECTS"); subjects != "" {
		busOpts = append(busOpts, natsbus.WithSubjects(subjects))
	}
	bus, err := natsbus.New(natsURL, events.Codec, busOpts...)
	if err != nil {
		return err
	}
	defer func() { _ = bus.Close() }()

	orch := orchestrator.New()
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Sources: registry.DefaultSources, Sinks: registry.DefaultSinks},
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

// waitForDB pings until the database accepts connections, so the migrate init
// container rides out postgres still starting on a fresh install.
func waitForDB(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(5 * time.Minute)
	for {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database not reachable: %w", err)
		}
		log.Printf("waiting for database: %v", err)
		time.Sleep(2 * time.Second)
	}
}

func defaultTenantID() string {
	if id := os.Getenv("DEFAULT_TENANT_ID"); id != "" {
		return id
	}
	return ctlpg.DefaultTenantID
}
