package server

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus/inproc"
)

type loadRunErrorStore struct {
	filament.DataStore
	err error
}

func TestSignalRunPauseResumeAndCancelStateMachine(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	api := &Server{store: store, bus: inproc.New()}
	ended := time.Now()
	state := filament.RunState{
		Run: "run-1", Tenant: "tenant-1", Status: filament.RunRequested,
		StartedAt: time.Now().Add(-time.Minute), EndedAt: &ended, Records: 9, Bytes: 90,
		Resources: []filament.ResourceState{{Resource: "users", Status: filament.RunCompleted, Records: 9, Bytes: 90, Error: "old"}},
	}
	if err := store.SaveRun(ctx, state); err != nil {
		t.Fatal(err)
	}

	signal := func(value ingestionv1.RunSignal) error {
		_, err := api.SignalRun(ctx, connect.NewRequest(&ingestionv1.SignalRunRequest{
			TenantId: "tenant-1", RunId: "run-1", Signal: value,
		}))
		return err
	}
	if err := signal(ingestionv1.RunSignal_RUN_SIGNAL_PAUSE); err != nil {
		t.Fatalf("pause requested run: %v", err)
	}
	paused, _ := store.LoadRun(ctx, "run-1")
	if paused.Status != filament.RunPaused {
		t.Fatalf("status after pause = %v", paused.Status)
	}
	if err := signal(ingestionv1.RunSignal_RUN_SIGNAL_RESUME); err != nil {
		t.Fatalf("resume paused run: %v", err)
	}
	resumed, _ := store.LoadRun(ctx, "run-1")
	if resumed.Status != filament.RunRequested || !resumed.StartedAt.IsZero() || resumed.EndedAt != nil || resumed.Records != 0 || resumed.Error != "" {
		t.Fatalf("resumed state was not reset: %#v", resumed)
	}
	if len(resumed.Resources) != 1 || resumed.Resources[0].Status != filament.RunRequested || resumed.Resources[0].Records != 0 || resumed.Resources[0].Error != "" {
		t.Fatalf("resumed resources were not reset: %#v", resumed.Resources)
	}
	if err := signal(ingestionv1.RunSignal_RUN_SIGNAL_CANCEL); err != nil {
		t.Fatalf("cancel requested run: %v", err)
	}
	canceled, _ := store.LoadRun(ctx, "run-1")
	if canceled.Status != filament.RunCanceled {
		t.Fatalf("status after cancel = %v", canceled.Status)
	}
}

func TestSignalRunRejectsUnsafeRunningWorkerCommands(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-1", Tenant: "tenant-1", Status: filament.RunRunning}); err != nil {
		t.Fatal(err)
	}
	api := &Server{store: store, bus: inproc.New()}
	for _, signal := range []ingestionv1.RunSignal{
		ingestionv1.RunSignal_RUN_SIGNAL_PAUSE,
		ingestionv1.RunSignal_RUN_SIGNAL_CANCEL,
	} {
		_, err := api.SignalRun(ctx, connect.NewRequest(&ingestionv1.SignalRunRequest{RunId: "run-1", Signal: signal}))
		if connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("signal %v error = %v, want failed precondition", signal, err)
		}
	}
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
