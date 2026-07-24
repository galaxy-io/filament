// Command worker executes one already-persisted Filament run.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/eventbus"
	"github.com/galaxy-io/filament/cmd/internal/persistence"
	"github.com/galaxy-io/filament/cmd/internal/secret"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"

	_ "github.com/galaxy-io/filament/cmd/internal/connectors"
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

	runID := filament.RunID(os.Getenv("RUN_ID"))
	if runID == "" {
		return errors.New("RUN_ID is required")
	}
	if err := runID.Valid(); err != nil {
		return fmt.Errorf("RUN_ID: %w", err)
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

	bus, err := eventbus.FromEnv()
	if err != nil {
		return err
	}
	defer func() {
		if c, ok := any(bus).(io.Closer); ok {
			_ = c.Close()
		}
	}()

	state, err := store.LoadRun(ctx, runID)
	if err != nil {
		return err
	}
	if state.Status != filament.RunRequested && state.Status != filament.RunPartial {
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
