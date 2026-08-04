package tracker

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/memory"
)

func TestIncrementalCheckpointUsesDurableResourceState(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: 3, CheckpointRoute: "route/source/sink/upsert",
		IngestionType: filament.IngestionUpsert,
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-a", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.ds = store
	cp := filament.NewCheckpoint("users").Set("watermark", "2026-08-04T00:00:00Z")
	if err := m.saveCheckpoint(ctx, "run-a", cp); err != nil {
		t.Fatal(err)
	}
	key, _ := request.ResourceCheckpointKey("users")
	stored, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Checkpoint.String("watermark") != "2026-08-04T00:00:00Z" {
		t.Fatalf("stored checkpoint = %#v", stored.Checkpoint.Raw())
	}
	if got := m.loadCheckpoint(ctx, "run-a", "users"); got == nil || got.String("watermark") == "" {
		t.Fatalf("loaded checkpoint = %#v", got)
	}
}

func TestCompletedBackfillPromotesInitialWatermark(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: 3, CheckpointRoute: "route/source/sink/upsert",
		IngestionType: filament.IngestionUpsert,
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-a", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	backfill := checkpoint.AsIncrementalBackfill(
		checkpoint.KeysetCheckpoint{Cols: []string{"id"}, Types: []string{"bigint"}, Shards: []checkpoint.KeysetShard{{}}},
		[]string{"updated_at", "id"}, []string{"timestamptz", "bigint"},
		[]string{"2026-08-04T00:00:00Z", "9"}, 300,
	).ToCheckpoint("users")
	key, _ := request.ResourceCheckpointKey("users")
	if err := store.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: "run-a", Checkpoint: backfill}); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.ds = store
	m.flushResource(ctx, "run-a", "users")
	stored, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	plan, ok := checkpoint.ParseKeyset(stored.Checkpoint)
	if !ok || plan.Mode != checkpoint.ModeIncremental {
		t.Fatalf("checkpoint was not promoted: %#v", stored.Checkpoint.Raw())
	}
}
