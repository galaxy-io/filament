// Command worker executes one already-persisted Filament run.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	ingestion "github.com/galaxy-io/filament"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"
	"github.com/galaxy-io/filament/secret"

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
	// Default logger too, so library logs (e.g. iceberg-go) come out as JSON.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	runID := ingestion.RunID(os.Getenv("RUN_ID"))
	if runID == "" {
		return errors.New("RUN_ID is required")
	}
	if err := runID.Valid(); err != nil {
		return fmt.Errorf("RUN_ID: %w", err)
	}

	PersistenceDSN := os.Getenv("PERSISTENCE_DSN")
	if PersistenceDSN == "" {
		return errors.New("PERSISTENCE_DSN is required")
	}
	natsURL := os.Getenv("NATS_URL")
	if natsURL == "" {
		return errors.New("NATS_URL is required")
	}

	pool, err := ctlpg.NewPool(ctx, PersistenceDSN)
	if err != nil {
		return err
	}
	defer pool.Close()
	store := ctlpg.New(pool)
	secrets, err := secret.FromEnv(pool)
	if err != nil {
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

	state, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	if state.Status != ingestion.RunRequested && state.Status != ingestion.RunPartial {
		return nil
	}

	runner.RunOne(ctx, runner.Deps{
		Bus:       bus,
		DataStore: store,
		Log:       slogLogger{l: logger},
		Secrets:   secrets,
		Sources:   registry.DefaultSources,
		Sinks:     registry.DefaultSinks,
	}, runner.SpecFromState(state))
	return nil
}
