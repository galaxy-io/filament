package tracker

import (
	"context"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// RecoverInterrupted folds a failure for unfinished local runs after the
// previous CLI process has exited. The caller must hold the local run lock,
// which proves no other CLI process is executing these runs.
func RecoverInterrupted(ctx context.Context, store filament.DataStore) error {
	runs, _, err := store.ListRuns(ctx, filament.RunFilter{
		Status: []filament.RunStatus{filament.RunRequested, filament.RunRunning},
	})
	if err != nil {
		return fmt.Errorf("list interrupted runs: %w", err)
	}
	m := New()
	m.ds = store
	for _, run := range runs {
		if run.Request.Options.Execution.Normalize() == filament.ExecutionContinuous {
			continue
		}
		fact := events.Fact{
			Envelope: events.Envelope{Tenant: run.Tenant, Run: run.Run, At: time.Now()},
			Data:     events.RunFailedEvent{Error: "local run interrupted before completion"},
		}
		if err := m.apply(ctx, fact); err != nil {
			return fmt.Errorf("recover run %q: %w", run.Run, err)
		}
	}
	return nil
}
