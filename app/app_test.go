package app

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
)

type reconciliationStore struct {
	filament.ContinuousRunStore
	filament.ScheduleStore
	polled chan struct{}
}

func (*reconciliationStore) Name() string { return "reconciliation-test" }

func (s *reconciliationStore) PendingStreamRuns(context.Context, string, int) ([]filament.RunState, error) {
	select {
	case s.polled <- struct{}{}:
	default:
	}
	return nil, nil
}

func TestComposeReconcilesContinuousRuns(t *testing.T) {
	schedules := sqlite.NewMemory()
	t.Cleanup(func() { _ = schedules.Close() })
	store := &reconciliationStore{ScheduleStore: schedules, polled: make(chan struct{}, 1)}
	_, _, cleanup, err := compose(t.Context(), newConfig(WithDataStore(store)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
	select {
	case <-store.polled:
	case <-time.After(5 * time.Second):
		t.Fatal("continuous supervisor did not reconcile the configured store")
	}
}
