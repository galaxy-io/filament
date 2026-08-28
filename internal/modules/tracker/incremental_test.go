package tracker

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/events"
)

type failOnceCheckpointStore struct {
	filament.DataStore
	failed bool
}

func (s *failOnceCheckpointStore) SaveResourceCheckpoint(ctx context.Context, state filament.ResourceCheckpointState) error {
	if !s.failed {
		s.failed = true
		return errors.New("checkpoint unavailable")
	}
	return s.DataStore.SaveResourceCheckpoint(ctx, state)
}

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
	if err := m.flushRunFor(ctx, "run-a", false, checkpointReason("flush")); err != nil {
		t.Fatal(err)
	}
	key, _ := request.ResourceCheckpointKey("users")
	if _, err := store.LoadResourceCheckpoint(ctx, key); err == nil {
		t.Fatal("partial run persisted a tentative checkpoint")
	}
	if err := m.flushRunFor(ctx, "run-a", true, checkpointReason("flush")); err != nil {
		t.Fatal(err)
	}
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

func TestPausedAcknowledgementCommitsCheckpoint(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "version-3", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
	}
	state := filament.RunState{Run: "run-a", Tenant: "tenant", Status: filament.RunRunning, Request: request}
	if err := store.SaveRun(ctx, state); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.ds = store
	cp := filament.NewCheckpoint("users").Set("watermark", "2026-08-04T00:00:00Z")
	m.cp[ckKey{run: "run-a", resource: "users"}] = cp
	m.boundary[ckKey{run: "run-a", resource: "users"}] = filament.CheckpointAfterCommit
	if err := m.apply(ctx, events.NewFact(
		events.RunPaused,
		events.Envelope{Tenant: "tenant", Run: "run-a"},
		events.RunPausedEvent{Committed: true},
	)); err != nil {
		t.Fatal(err)
	}
	key, _ := request.ResourceCheckpointKey("users")
	stored, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.Checkpoint.String("watermark"); got != "2026-08-04T00:00:00Z" {
		t.Fatalf("paused checkpoint = %q", got)
	}
}

func TestPausedAcknowledgementWaitsForCheckpointPromotion(t *testing.T) {
	ctx := context.Background()
	base := memory.New()
	store := &failOnceCheckpointStore{DataStore: base}
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "version-3", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
	}
	if err := base.SaveRun(ctx, filament.RunState{Run: "run-a", Tenant: "tenant", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.ds = store
	cp := filament.NewCheckpoint("users").Set("watermark", "2026-08-04T00:00:00Z")
	m.cp[ckKey{run: "run-a", resource: "users"}] = cp
	m.boundary[ckKey{run: "run-a", resource: "users"}] = filament.CheckpointAfterCommit
	msg := fakeMsg{seq: 7, fact: events.NewFact(
		events.RunPaused,
		events.Envelope{Tenant: "tenant", Run: "run-a"},
		events.RunPausedEvent{Committed: true},
	)}
	if err := m.onFact(ctx, msg); err == nil {
		t.Fatal("pause succeeded despite failed checkpoint promotion")
	}
	state, err := base.LoadRun(ctx, "run-a")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != filament.RunRunning {
		t.Fatalf("status after failed promotion = %v, want running", state.Status)
	}
	if err := m.onFact(ctx, msg); err != nil {
		t.Fatalf("retry pause: %v", err)
	}
	state, err = base.LoadRun(ctx, "run-a")
	if err != nil || state.Status != filament.RunPaused {
		t.Fatalf("status after retry = %v, %v", state.Status, err)
	}
}

func TestCDCCheckpointBecomesDurableOnlyAfterCommit(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "version-3", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionCDCMerge},
	}
	if err := store.SaveRun(ctx, filament.RunState{Run: "run-a", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	m := New()
	m.ds = store
	cp := checkpoint.NewStreamDelta("users", "0/16B6C50", 8)
	m.cp[ckKey{run: "run-a", resource: "users"}] = cp
	if err := m.flushRunFor(ctx, "run-a", false, checkpointReason("flush")); err != nil {
		t.Fatal(err)
	}
	key, _ := request.ResourceCheckpointKey("users")
	if _, err := store.LoadResourceCheckpoint(ctx, key); err == nil {
		t.Fatal("partial CDC run persisted a tentative checkpoint")
	}
	if err := m.flushRunFor(ctx, "run-a", true, checkpointReason("flush")); err != nil {
		t.Fatal(err)
	}
	stored, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	lsn, _, ok := checkpoint.ParseStream(stored.Checkpoint)
	if !ok || lsn != "0/16B6C50" {
		t.Fatalf("stored CDC checkpoint = %#v", stored.Checkpoint)
	}
}

func TestCompletedBackfillPromotesInitialWatermarkAfterRunCommit(t *testing.T) {
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
	m.boundary[ckKey{run: "run-a", resource: "users"}] = filament.CheckpointAfterCommit
	m.flushResource(ctx, "run-a", "users")
	beforeCommit, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	beforePlan, ok := checkpoint.ParseKeyset(beforeCommit.Checkpoint)
	if !ok || beforePlan.Mode != checkpoint.ModeIncrementalBackfill {
		t.Fatalf("resource completion promoted checkpoint before commit: %#v", beforeCommit.Checkpoint.Raw())
	}
	if err := m.flushRunFor(ctx, "run-a", true, checkpointReason("flush")); err != nil {
		t.Fatal(err)
	}
	stored, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	plan, ok := checkpoint.ParseKeyset(stored.Checkpoint)
	if !ok || plan.Mode != checkpoint.ModeIncremental {
		t.Fatalf("checkpoint was not promoted: %#v", stored.Checkpoint.Raw())
	}
}
