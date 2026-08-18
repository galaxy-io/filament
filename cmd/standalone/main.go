// Command standalone runs all of Filament in one process: an embedded NATS
// JetStream event plane, the ConnectRPC API, engine, tracker, and orchestrator
// over the in-memory store. Secrets resolve from environment variables
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
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	secretenv "github.com/galaxy-io/filament/secret/env"
	"github.com/galaxy-io/filament/ui"

	// Curated connector set — self-register via init():
	_ "github.com/galaxy-io/filament/connectors/clickhouse"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/mysql"
	_ "github.com/galaxy-io/filament/connectors/postgres"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	lg := logger.New()
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
		app.WithLogger(lg),
		app.WithSecrets(secretenv.New()),
		app.WithUI(ui.Handler()),
	)
}
