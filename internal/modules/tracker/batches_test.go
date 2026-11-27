package tracker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
)

// countingStore counts run loads and saves and can fail saves on demand.
type countingStore struct {
	*sqlite.Store
	loads, saves, dedups int
	failSave, failDedup  bool
}

func (s *countingStore) LoadRun(ctx context.Context, tenant filament.TenantID, run filament.RunID) (filament.RunState, error) {
	s.loads++
	return s.Store.LoadRun(ctx, tenant, run)
}

func (s *countingStore) SaveRun(ctx context.Context, r filament.RunState) error {
	if s.failSave {
		return errors.New("injected save failure")
	}
	s.saves++
	return s.Store.SaveRun(ctx, r)
}

func (s *countingStore) DedupHighWater(ctx context.Context, tenant string, run filament.RunID) (uint64, bool, error) {
	s.dedups++
	return s.Store.DedupHighWater(ctx, tenant, run)
}

func (s *countingStore) DedupSeen(ctx context.Context, tenant string, run filament.RunID, seq uint64) (bool, error) {
	if s.failDedup {
		return false, errors.New("injected dedup failure")
	}
	s.dedups++
	return s.Store.DedupSeen(ctx, tenant, run, seq)
}

// clocked mounts a tracker on a counting store with a clock the test moves.
func clocked(t *testing.T) (*Module, *countingStore, *time.Time) {
	t.Helper()
	m := New()
	ds := &countingStore{Store: sqlite.NewMemory()}
	if err := m.Mount(context.Background(), module.Deps{DataStore: ds}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	return m, ds, &now
}

func records(t *testing.T, ds filament.DataStore) int64 {
	t.Helper()
	state, err := ds.LoadRun(context.Background(), "t1", "r1")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	return state.Records
}

func TestTrackerIgnoresUnfoldedFactsWithoutStoreCalls(t *testing.T) {
	m, ds, _ := clocked(t)
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users"}
	for seq := uint64(1); seq <= 100; seq++ {
		deliver(t, m, seq, events.BatchBuffered, res, events.BatchBufferedEvent{Records: 256})
	}
	if ds.loads+ds.saves+ds.dedups != 0 {
		t.Fatalf("store calls for ignored facts: %d loads, %d saves, %d dedup", ds.loads, ds.saves, ds.dedups)
	}
}

func TestTrackerBatchesTotalsBetweenFlushes(t *testing.T) {
	m, ds, now := clocked(t)
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users"}
	batch := events.BatchWrittenEvent{Records: 10, Bytes: 100}

	deliver(t, m, 1, events.BatchWritten, res, batch)
	if got := records(t, ds.Store); got != 10 {
		t.Fatalf("first batch saved %d records, want 10 at once", got)
	}
	saves := ds.saves
	for seq := uint64(2); seq <= 50; seq++ {
		deliver(t, m, seq, events.BatchWritten, res, batch)
	}
	if ds.saves != saves {
		t.Fatalf("%d saves inside one flush interval, want none", ds.saves-saves)
	}
	if got := records(t, ds.Store); got != 10 {
		t.Fatalf("records before the interval = %d, want 10", got)
	}

	*now = now.Add(totalsFlushInterval)
	deliver(t, m, 51, events.BatchWritten, res, batch)
	if got := records(t, ds.Store); got != 510 {
		t.Fatalf("records after the interval = %d, want 510", got)
	}
	if hw, _, _ := ds.Store.DedupHighWater(context.Background(), "t1", "r1"); hw != 51 {
		t.Fatalf("dedup mark = %d, want 51", hw)
	}
	// A redelivered batch already folded into the totals is not added again.
	deliver(t, m, 30, events.BatchWritten, res, batch)
	*now = now.Add(totalsFlushInterval)
	deliver(t, m, 52, events.Heartbeat, events.Envelope{Tenant: "t1", Run: "r1"}, events.HeartbeatEvent{})
	if got := records(t, ds.Store); got != 510 {
		t.Fatalf("records after redelivery = %d, want 510", got)
	}
}

func TestTrackerSavesPendingTotalsBeforeOtherFacts(t *testing.T) {
	m, ds, _ := clocked(t)
	env := events.Envelope{Tenant: "t1", Run: "r1"}
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users"}

	deliver(t, m, 1, events.RunStarted, env, events.RunStartedEvent{})
	for seq := uint64(2); seq <= 5; seq++ {
		deliver(t, m, seq, events.BatchWritten, res, events.BatchWrittenEvent{Records: 10, Bytes: 100})
	}
	// run.failed carries no totals, so the pending ones must land first.
	deliver(t, m, 6, events.RunFailed, env, events.RunFailedEvent{Error: "boom"})
	state, err := ds.Store.LoadRun(context.Background(), "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != filament.RunFailed || state.Records != 40 || state.Bytes != 400 {
		t.Fatalf("run after failure = %v with %d/%d, want failed with 40/400", state.Status, state.Records, state.Bytes)
	}
	if len(state.Resources) != 1 || state.Resources[0].Records != 40 {
		t.Fatalf("resources = %#v, want users with 40 records", state.Resources)
	}
	if len(m.folds) != 0 {
		t.Fatalf("run fold kept after terminal: %d entries", len(m.folds))
	}
}

func TestTrackerKeepsPendingTotalsWhenFlushFails(t *testing.T) {
	m, ds, _ := clocked(t)
	ctx := context.Background()
	env := events.Envelope{Tenant: "t1", Run: "r1"}
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users"}

	deliver(t, m, 1, events.BatchWritten, res, events.BatchWrittenEvent{Records: 10})
	deliver(t, m, 2, events.BatchWritten, res, events.BatchWrittenEvent{Records: 10})

	ds.failSave = true
	heartbeat := fakeMsg{fact: events.NewFact(events.Heartbeat, env, events.HeartbeatEvent{CPUSeconds: 1}), seq: 3}
	if err := m.onFact(ctx, heartbeat); err == nil {
		t.Fatal("heartbeat folded despite a failed totals save")
	}
	ds.failSave = false
	if err := m.onFact(ctx, heartbeat); err != nil {
		t.Fatalf("redelivered heartbeat: %v", err)
	}
	state, err := ds.Store.LoadRun(ctx, "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Records != 20 || state.CPUSeconds != 1 {
		t.Fatalf("run = %d records, %v cpu, want 20 and 1", state.Records, state.CPUSeconds)
	}
}

func TestTrackerFoldsCursorWhenDedupWriteFails(t *testing.T) {
	m, ds, _ := clocked(t)
	ctx := context.Background()
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users"}
	batch := fakeMsg{fact: events.NewFact(events.BatchWritten, res, events.BatchWrittenEvent{
		Records: 5, Checkpoint: checkpoint.NewCoarseAck("users", 0, 5),
	}), seq: 1}

	ds.failDedup = true
	if err := m.onFact(ctx, batch); err == nil {
		t.Fatal("batch acknowledged despite a failed dedup write")
	}
	ds.failDedup = false
	m.mu.Lock()
	folded := len(m.bm) > 0
	m.mu.Unlock()
	if !folded {
		t.Fatal("cursor not folded after the totals were saved")
	}
	// The redelivery is skipped as applied, without adding the totals twice.
	if err := m.onFact(ctx, batch); err != nil {
		t.Fatalf("redelivered batch: %v", err)
	}
	if got := records(t, ds.Store); got != 5 {
		t.Fatalf("records = %d, want 5", got)
	}
}

func TestTrackerTerminalFactRestoresTotalsLostOnRestart(t *testing.T) {
	m, ds, _ := clocked(t)
	env := events.Envelope{Tenant: "t1", Run: "r1"}
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users"}
	for seq := uint64(1); seq <= 3; seq++ {
		deliver(t, m, seq, events.BatchWritten, res, events.BatchWrittenEvent{Records: 10, Bytes: 100})
	}
	if got := records(t, ds.Store); got != 10 {
		t.Fatalf("records before restart = %d, want 10 with two batches pending", got)
	}

	restarted := New()
	if err := restarted.Mount(context.Background(), module.Deps{DataStore: ds}); err != nil {
		t.Fatal(err)
	}
	deliver(t, restarted, 4, events.RunFailed, env, events.RunFailedEvent{Error: "boom", Records: 30, Bytes: 300})
	state, err := ds.Store.LoadRun(context.Background(), "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Records != 30 || state.Bytes != 300 {
		t.Fatalf("run after failure = %d/%d, want the engine's 30/300", state.Records, state.Bytes)
	}
}
