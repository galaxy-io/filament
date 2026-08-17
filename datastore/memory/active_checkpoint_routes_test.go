package memory

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestSaveRunRejectsOverlappingCheckpointRoute(t *testing.T) {
	ctx := context.Background()
	store := New()
	request := filament.RunRequest{PipelineID: "pipe", PipelineVersionID: "version-1", CheckpointRoute: "route/source/sink/upsert"}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-a", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-b", Status: filament.RunRequested, Request: request}); err == nil {
		t.Fatal("expected overlapping run rejection")
	}
	first, err := store.LoadRun(ctx, "run-a")
	if err != nil {
		t.Fatal(err)
	}
	first.Status = filament.RunCompleted
	if err := store.SaveRun(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-b", Status: filament.RunRequested, Request: request}); err != nil {
		t.Fatalf("save after completion: %v", err)
	}
}

func TestPartialRunDoesNotBlockNextScheduledAttempt(t *testing.T) {
	ctx := context.Background()
	store := New()
	request := filament.RunRequest{PipelineID: "pipe", PipelineVersionID: "version-1", CheckpointRoute: "route/source/sink/upsert"}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-a", Status: filament.RunPartial, Request: request}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-b", Status: filament.RunRequested, Request: request}); err != nil {
		t.Fatalf("partial run blocked next attempt: %v", err)
	}
}
