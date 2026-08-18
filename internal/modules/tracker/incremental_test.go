package tracker

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/events"
)

func TestWatermarkProgressIsNotPersistedBeforeBatchWrite(t *testing.T) {
	m := New()
	m.ds = memory.New()
	cp := &filament.CheckpointData{ResourceName: "users", Cursor: map[string]any{"updated_at": "2026-08-18T00:00:00Z"}}
	err := m.apply(context.Background(), events.NewFact(
		events.WatermarkAdvanced,
		events.Envelope{Tenant: "tenant", Run: "run", Resource: "users"},
		events.WatermarkAdvancedEvent{Checkpoint: cp},
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.cp) != 0 {
		t.Fatalf("observed watermark entered durable accumulator: %#v", m.cp)
	}
}

func TestIncrementalCheckpointBecomesDurableOnlyAfterCommit(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "version-3", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
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
	m.cp[ckKey{run: "run-a", resource: "users"}] = cp
	m.flushRun(ctx, "run-a", false)
	key, _ := request.ResourceCheckpointKey("users")
	if _, err := store.LoadResourceCheckpoint(ctx, key); err == nil {
		t.Fatal("partial run persisted a tentative checkpoint")
	}
	m.flushRun(ctx, "run-a", true)
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

func TestCDCCheckpointBecomesDurableOnlyAfterCommit(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "version-3", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionCDC},
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-a", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.ds = store
	cp := checkpoint.NewStreamDelta("users", "0/16B6C50", 8)
	m.cp[ckKey{run: "run-a", resource: "users"}] = cp
	m.flushRun(ctx, "run-a", false)
	key, _ := request.ResourceCheckpointKey("users")
	if _, err := store.LoadResourceCheckpoint(ctx, key); err == nil {
		t.Fatal("partial CDC run persisted a tentative checkpoint")
	}
	m.flushRun(ctx, "run-a", true)
	stored, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	lsn, _, ok := checkpoint.ParseStream(stored.Checkpoint)
	if !ok || lsn != "0/16B6C50" {
		t.Fatalf("stored CDC checkpoint = %#v", stored.Checkpoint)
	}
}

func TestCompletedBackfillPromotesInitialWatermark(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "version-3", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
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
