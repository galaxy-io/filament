package server

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
)

type loadRunErrorStore struct {
	filament.DataStore
	err error
}

func (s loadRunErrorStore) LoadRun(context.Context, filament.RunID) (filament.RunState, error) {
	return filament.RunState{}, s.err
}

func TestLoadRunSnapshotOnlySuppressesNotFound(t *testing.T) {
	t.Parallel()

	storeErr := errors.New("database unavailable")
	server := &Server{store: loadRunErrorStore{DataStore: memory.New(), err: storeErr}}
	if _, ok, err := server.loadRunSnapshot(context.Background(), "run-1"); ok || !errors.Is(err, storeErr) {
		t.Fatalf("loadRunSnapshot() = (_, %v, %v), want (_, false, store error)", ok, err)
	}

	server.store = loadRunErrorStore{DataStore: memory.New(), err: filament.ErrNotFound}
	if _, ok, err := server.loadRunSnapshot(context.Background(), "run-1"); ok || err != nil {
		t.Fatalf("loadRunSnapshot() = (_, %v, %v), want (_, false, nil)", ok, err)
	}
}
