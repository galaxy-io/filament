package tracker

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
)

// fakeMsg delivers a fact by reference with a broker-style stream sequence.
type fakeMsg struct {
	fact events.Fact
	seq  uint64
}

func (m fakeMsg) Subject() string { return "" }
func (m fakeMsg) Payload() any    { return m.fact }
func (m fakeMsg) Seq() uint64     { return m.seq }
func (m fakeMsg) Ack() error      { return nil }
func (m fakeMsg) Nak() error      { return nil }

func mounted(t *testing.T) (*Module, *memory.Store) {
	t.Helper()
	m := New()
	ds := memory.New()
	if err := m.Mount(context.Background(), module.Deps{DataStore: ds}); err != nil {
		t.Fatalf("mount: %v", err)
	}
	return m, ds
}

func deliver[T any](t *testing.T, m *Module, seq uint64, typ events.EventType[T], env events.Envelope, data T) {
	t.Helper()
	if err := m.onFact(context.Background(), fakeMsg{fact: events.NewFact(typ, env, data), seq: seq}); err != nil {
		t.Fatalf("onFact seq %d: %v", seq, err)
	}
}

func TestTrackerFoldsRunLifecycle(t *testing.T) {
	m, ds := mounted(t)
	ctx := context.Background()
	started := time.Now()
	env := events.Envelope{Tenant: "t1", Run: "r1", At: started}
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users", At: started}

	deliver(t, m, 1, events.RunStarted, env, events.RunStartedEvent{})
	state, err := ds.LoadRun(ctx, "r1")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if state.Status != filament.RunRunning {
		t.Fatalf("status after run.started = %v, want running", state.Status)
	}
	if state.StartedAt.IsZero() {
		t.Fatal("run.started did not stamp StartedAt")
	}

	deliver(t, m, 2, events.BatchWritten, res, events.BatchWrittenEvent{Records: 5, Bytes: 100})
	state, _ = ds.LoadRun(ctx, "r1")
	if state.Records != 5 || state.Bytes != 100 {
		t.Fatalf("run totals = %d/%d, want 5/100", state.Records, state.Bytes)
	}
	if len(state.Resources) != 1 || state.Resources[0].Records != 5 {
		t.Fatalf("resource state = %#v, want users with 5 records", state.Resources)
	}

	deliver(t, m, 3, events.RunCompleted, env, events.RunCompletedEvent{Records: 5, Bytes: 100})
	state, _ = ds.LoadRun(ctx, "r1")
	if state.Status != filament.RunCompleted {
		t.Fatalf("status after run.completed = %v, want completed", state.Status)
	}
	if state.FinishedAt == nil {
		t.Fatal("run.completed did not stamp FinishedAt")
	}
}

func TestTrackerDedupsRedeliveredFacts(t *testing.T) {
	m, ds := mounted(t)
	ctx := context.Background()
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users", At: time.Now()}

	deliver(t, m, 7, events.BatchWritten, res, events.BatchWrittenEvent{Records: 5, Bytes: 100})
	deliver(t, m, 7, events.BatchWritten, res, events.BatchWrittenEvent{Records: 5, Bytes: 100})
	state, err := ds.LoadRun(ctx, "r1")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if state.Records != 5 {
		t.Fatalf("records after redelivery = %d, want 5 (folded once)", state.Records)
	}
}

func TestTrackerTerminalStampIsFirstWriteWins(t *testing.T) {
	m, ds := mounted(t)
	ctx := context.Background()
	first := time.Now().Add(-time.Minute)
	env := events.Envelope{Tenant: "t1", Run: "r1", At: first}

	deliver(t, m, 1, events.RunCompleted, env, events.RunCompletedEvent{})
	late := env
	late.At = first.Add(time.Minute)
	deliver(t, m, 2, events.RunFailed, late, events.RunFailedEvent{Error: "late straggler"})

	state, err := ds.LoadRun(ctx, "r1")
	if err != nil {
		t.Fatalf("load run: %v", err)
	}
	if state.FinishedAt == nil || !state.FinishedAt.Equal(first) {
		t.Fatalf("FinishedAt = %v, want first stamp %v", state.FinishedAt, first)
	}
}

func TestTrackerEvictsAccumulatorsOnTerminal(t *testing.T) {
	m, _ := mounted(t)
	env := events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()}
	res := events.Envelope{Tenant: "t1", Run: "r1", Resource: "users", At: time.Now()}

	deliver(t, m, 1, events.BatchWritten, res, events.BatchWrittenEvent{
		Records: 5, Bytes: 100, Checkpoint: checkpoint.NewCoarseAck("users", 0, 5),
	})
	m.mu.Lock()
	seeded := len(m.cp) > 0 && len(m.bm) > 0
	m.mu.Unlock()
	if !seeded {
		t.Fatal("batch fact did not seed accumulator entries")
	}

	deliver(t, m, 2, events.RunCompleted, env, events.RunCompletedEvent{Records: 5, Bytes: 100})
	m.mu.Lock()
	leaked := len(m.cp) + len(m.since) + len(m.bm) + len(m.every)
	m.mu.Unlock()
	if leaked != 0 {
		t.Fatalf("accumulator entries after terminal = %d, want 0", leaked)
	}
}
