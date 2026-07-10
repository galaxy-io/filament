// Command ingestion-worker executes one already-persisted Filament run.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/sample"
	"github.com/galaxy-io/filament/connectors/stdout"
	ctlpg "github.com/galaxy-io/filament/datastore/postgres"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"
	secretenv "github.com/galaxy-io/filament/secret/env"

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
		// Temporary bootstrap path: ConfigRef values are resolved from process
		// env vars as JSON connector configs. Replace this with the fetched
		// control-plane secret/config store before production k8s dispatch.
		Secrets: secretenv.NewIngestionSecrets(),
		Sources: registry.DefaultSources,
		Sinks:   registry.DefaultSinks,
	}, runner.SpecFromState(state))
	return nil
}
