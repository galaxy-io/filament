package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResourceCheckpointSurvivesRunIdentity(t *testing.T) {
	ctx := context.Background()
	store := New()
	key := filament.ResourceCheckpointKey{PipelineID: "pipe", PipelineVersionID: 2, Route: "source/sink/upsert", Resource: "users"}
	cp := filament.NewCheckpoint("users").Set("updated_at", "2026-08-04T00:00:00Z")
	if err := store.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: "run-a", Checkpoint: cp}); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Run != "run-a" || got.Checkpoint.String("updated_at") != "2026-08-04T00:00:00Z" {
		t.Fatalf("checkpoint = %#v", got)
	}

	otherVersion := key
	otherVersion.PipelineVersionID++
	if _, err := store.LoadResourceCheckpoint(ctx, otherVersion); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("other version error = %v", err)
	}
	if err := store.DeleteResourceCheckpoint(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadResourceCheckpoint(ctx, key); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("deleted checkpoint error = %v", err)
	}
}
