// Command standalone runs all of Filament in one process: an embedded NATS
// JetStream event plane, the ConnectRPC API, engine, tracker, and orchestrator
// over a SQLite store (STORE_PATH, defaulting next to the NATS store). It is
// the CLI's local engine and a single-container evaluation deployment, not a
// production topology.
//
// Secrets are stored encrypted in the store, keyed by ENCRYPTION_KEY or by a
// secret.key file beside it, and references the store does not hold resolve
// from environment variables. One process owns a store at a time, and runs
// left unfinished by a previous process are failed on startup.
//
// `standalone run` executes one untracked run for the CLI: a JSON run spec
// and then control lines on stdin, the run's facts on file descriptor 3.
// `standalone -version` prints the version.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"

	"github.com/galaxy-io/filament/app"
	"github.com/galaxy-io/filament/cmd/internal/connectors"
	"github.com/galaxy-io/filament/cmd/internal/inline"
	"github.com/galaxy-io/filament/cmd/internal/logger"
	"github.com/galaxy-io/filament/cmd/internal/secret"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/datastore/sqlite/metrics"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/ui"
	"github.com/galaxy-io/filament/version"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	var err error
	switch {
	case len(os.Args) > 1 && os.Args[1] == "-version":
		fmt.Println(version.String())
	case len(os.Args) > 1 && os.Args[1] == "run":
		err = runInline(ctx)
	default:
		err = run(ctx)
	}
	stop()
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	lg, err := logger.New()
	if err != nil {
		return err
	}
	sources, err := connectors.SourcesFromEnv()
	if err != nil {
		return err
	}
	sinks, err := connectors.SinksFromEnv()
	if err != nil {
		return err
	}
	dir := os.Getenv("NATS_STORE_DIR")
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "filament-standalone")
	}
	storePath := os.Getenv("STORE_PATH")
	if storePath == "" {
		storePath = filepath.Join(dir, "filament.db")
	}
	unlock, err := lockStore(storePath)
	if err != nil {
		return err
	}
	defer unlock()

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

	store, err := sqlite.Open(storePath)
	if err != nil {
		return err
	}
	if err := tracker.RecoverInterrupted(ctx, store); err != nil {
		_ = store.Close()
		return err
	}
	secrets, err := secret.Local(store.DB(), filepath.Join(filepath.Dir(storePath), "secret.key"))
	if err != nil {
		_ = store.Close()
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
		app.WithNotifier(),
		app.WithBus(bus),
		app.WithSources(sources),
		app.WithSinks(sinks),
		app.WithDataStore(store),
		app.WithMetricsStore(metrics.New(store.DB())),
		app.WithLogger(lg),
		app.WithSecrets(secrets),
		app.WithUI(ui.Handler()),
	)
}

// runInline executes one untracked run, with facts on file descriptor 3.
func runInline(ctx context.Context) error {
	sources, err := connectors.SourcesFromEnv()
	if err != nil {
		return err
	}
	sinks, err := connectors.SinksFromEnv()
	if err != nil {
		return err
	}
	// Descriptor 3 is never closed here: when the caller left it closed, the
	// runtime may hold that number itself, and a write fails instead.
	return inline.Run(ctx, sources, sinks, os.Stdin, os.NewFile(3, "facts"))
}

// lockStore keeps a second process from serving, and recovering the runs of,
// a store another process owns. The lock is held until the process exits.
func lockStore(storePath string) (func(), error) {
	// #nosec G703 -- STORE_PATH is operator configuration, never request input
	if err := os.MkdirAll(filepath.Dir(storePath), 0o700); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	// #nosec G703 -- STORE_PATH is operator configuration, never request input
	file, err := os.OpenFile(storePath+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open store lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, fmt.Errorf("another process is serving %s", storePath)
		}
		return nil, fmt.Errorf("lock store: %w", err)
	}
	return func() {
		_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		_ = file.Close()
	}, nil
}
