// Command ingestion-control runs the POC control plane without an in-process
// engine. It persists run submissions, publishes run.requested facts, and folds
// worker-emitted facts back into Postgres through tracker.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/sample"
	"github.com/galaxy-io/filament/connectors/stdout"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/k8sdispatch"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"

	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
)

func main() {
	registry.RegisterSource("sample", func() ingestion.Source { return sample.New() })
	registry.RegisterSink("stdout", func() ingestion.Sink { return stdout.New() })

	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	PersistenceDSN := os.Getenv("PERSISTENCE_DSN")
	if PersistenceDSN == "" {
		return errors.New("PERSISTENCE_DSN is required")
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return errors.New("NATS_URL is required")
	}

	if migrateEnabled() {
		db, err := ctlpg.NewSQLDB(PersistenceDSN)
		if err != nil {
			return err
		}
		if err := ctlpg.Migrate(db); err != nil {
			_ = db.Close()
			return err
		}
		if err := db.Close(); err != nil {
			return fmt.Errorf("close migration db: %w", err)
		}
	}

	pool, err := ctlpg.NewPool(ctx, PersistenceDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := ctlpg.New(pool)
	if err := store.EnsureTenant(ctx, ingestion.TenantID(defaultTenantID()), "Default tenant"); err != nil {
		return err
	}

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
		tracker.New(),
		k8sdispatch.NewFromEnv(),
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
	for _, name := range h.Mounted() {
		fmt.Println("mounted:", name)
	}

	addr := os.Getenv("INGESTION_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	server.New(registry.DefaultSources, registry.DefaultSinks, store, orch, bus).Mount(mux)
	fmt.Println("connectrpc:", "http://localhost"+addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}

func migrateEnabled() bool {
	switch os.Getenv("PERSISTENCE_MIGRATE") {
	case "", "1", "true", "TRUE", "yes", "YES":
		return true
	default:
		return false
	}
}

func defaultTenantID() string {
	if id := os.Getenv("DEFAULT_TENANT_ID"); id != "" {
		return id
	}
	return ctlpg.DefaultTenantID
}
