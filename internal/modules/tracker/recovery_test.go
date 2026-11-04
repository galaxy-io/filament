package tracker

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/sqlite"
)

func TestRecoverInterruptedReleasesCheckpointRoute(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemory()
	defer func() { _ = store.Close() }()
	request := filament.RunRequest{
		Tenant: "local", PipelineID: "pipeline", PipelineVersionID: "version", CheckpointRoute: "route",
	}
	first := filament.RunState{Run: "first", Tenant: "local", Status: filament.RunRunning, Request: request, Records: 16000}
	if err := store.CreateRun(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := RecoverInterrupted(ctx, store); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.LoadRun(ctx, "local", "first")
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != filament.RunFailed || recovered.EndedAt == nil || recovered.Records != first.Records {
		t.Fatalf("recovered run = %#v", recovered)
	}
	if err := store.CreateRun(ctx, filament.RunState{
		Run: "second", Tenant: "local", Status: filament.RunRequested, Request: request,
	}); err != nil {
		t.Fatalf("next run could not use checkpoint route: %v", err)
	}
}
