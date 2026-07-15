// Command control-plane runs Filament's orchestration loop: it migrates the
// datastore, executes or dispatches requested runs per DISPATCH_MODE, and
// folds worker-emitted facts back into Postgres through tracker.
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	ingestion "github.com/galaxy-io/filament"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/k8sdispatch"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	secretpostgres "github.com/galaxy-io/filament/secret/postgres"
	"github.com/galaxy-io/filament/server"

	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
	_ "github.com/galaxy-io/filament/connectors/sample"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// dispatchModule selects the execution environment for requested runs:
// kubernetes launches one worker Job per run; inproc executes runs inside
// this process through the engine module.
func dispatchModule() (module.Module, error) {
	switch mode := os.Getenv("DISPATCH_MODE"); mode {
	case "", "kubernetes":
		return k8sdispatch.NewFromEnv(), nil
	case "inproc":
		return engine.New(), nil
	default:
		return nil, fmt.Errorf("unknown DISPATCH_MODE %q (kubernetes|inproc)", mode)
	}
}

func run(ctx context.Context) error {
	persistenceDSN := os.Getenv("PERSISTENCE_DSN")
	if persistenceDSN == "" {
		return errors.New("PERSISTENCE_DSN is required")
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return errors.New("NATS_URL is required")
	}

	if migrateEnabled() {
		db, err := ctlpg.NewSQLDB(persistenceDSN)
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

	pool, err := ctlpg.NewPool(ctx, persistenceDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := ctlpg.New(pool)
	encoded := os.Getenv("FILAMENT_SECRETS_KEY")
	if encoded == "" {
		return errors.New("FILAMENT_SECRETS_KEY is required")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return fmt.Errorf("FILAMENT_SECRETS_KEY must be base64: %w", err)
	}
	keyID := os.Getenv("FILAMENT_SECRETS_KEY_ID")
	if keyID == "" {
		keyID = "default"
	}
	secrets, err := secretpostgres.New(pool, keyID, key)
	if err != nil {
		return err
	}
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

	dispatch, err := dispatchModule()
	if err != nil {
		return err
	}
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Sources: registry.DefaultSources, Sinks: registry.DefaultSinks},
		tracker.New(),
		dispatch,
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
	server.New(registry.DefaultSources, registry.DefaultSinks, store, orch, bus,
		server.WithSecrets(secrets)).Mount(mux)
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
