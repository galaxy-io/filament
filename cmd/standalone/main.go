// Command standalone runs all of Filament in one process: an embedded NATS
// JetStream event plane, the ConnectRPC API, engine, tracker, and orchestrator
// over a SQLite store (STORE_PATH, defaulting next to the NATS store).
// Secrets resolve from environment variables
package main

import (
	"context"
	"errors"
	"log"
	"os"
	"path/filepath"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament/app"
	"github.com/galaxy-io/filament/cmd/internal/logger"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/datastore/sqlite/metrics"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	secretenv "github.com/galaxy-io/filament/secret/env"
	"github.com/galaxy-io/filament/ui"

	// Curated connector set — self-register via init():
	_ "github.com/galaxy-io/filament/connectors/clickhouse"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/mysql"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	lg, err := logger.New()
	if err != nil {
		return err
	}
	dir := os.Getenv("NATS_STORE_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "filament-standalone")
	}
	ns, err := natsserver.NewServer(&natsserver.Options{
		ServerName: "filament-standalone",
		DontListen: true,
		JetStream:  true,
		StoreDir:   dir,
	})
	if err != nil {
		return err
	}
	go ns.Start()
	defer ns.Shutdown()
	if !ns.ReadyForConnections(10 * time.Second) {
		return errors.New("embedded nats: not ready")
	}

	storePath := os.Getenv("STORE_PATH")
	if storePath == "" {
		storePath = filepath.Join(dir, "filament.db")
	}
	store, err := sqlite.Open(storePath)
	if err != nil {
		return err
	}

	nc, err := natsgo.Connect("", natsgo.InProcessServer(ns))
	if err != nil {
		return err
	}
	defer nc.Close()

	bus, err := natsbus.New("", events.Codec,
		natsbus.WithConn(nc),
		natsbus.WithSubjects("ingestion.v1.>"),
	)
	if err != nil {
		return err
	}

	return app.Run(ctx,
		app.WithBus(bus),
		app.WithDataStore(store),
		app.WithMetricsStore(metrics.New(store.DB())),
		app.WithLogger(lg),
		app.WithSecrets(secretenv.New()),
		app.WithUI(ui.Handler()),
	)
}
