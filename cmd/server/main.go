// Command server runs the Filament API: it serves the ConnectRPC surface and
// the embedded web UI, persists pipeline and run submissions, and publishes
// run.requested facts for the control plane to dispatch.
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

	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
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
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
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

	pool, err := ctlpg.NewPool(ctx, persistenceDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := ctlpg.New(pool)

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

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	mux := http.NewServeMux()
	server.New(registry.DefaultSources, registry.DefaultSinks, store, orch, bus).Mount(mux)
	mux.Handle("/", ui.Handler())
	fmt.Println("server:", "http://localhost"+addr)
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	return srv.ListenAndServe()
}
