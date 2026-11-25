package tracker

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/events"
)

// isolationRun seeds an incremental run over users and orders, each with an
// empty single-shard keyset plan in its route checkpoint.
func isolationRun(t *testing.T) (*Module, *sqlite.Store, filament.RunRequest) {
	t.Helper()
	ctx := context.Background()
	m, ds := mounted(t)
	request := filament.RunRequest{
		PipelineID: "pipe", PipelineVersionID: "v1", CheckpointRoute: "route/source/sink",
		IngestionTypes: map[string]filament.IngestionType{
			"users":  filament.IngestionIncrementalUpsert,
			"orders": filament.IngestionIncrementalUpsert,
		},
		Options: filament.RunOptions{IsolateResourceFailures: true},
	}
	if err := ds.SaveRun(ctx, filament.RunState{Run: "r1", Tenant: "t1", Status: filament.RunRunning, Request: request}); err != nil {
		t.Fatal(err)
	}
	for _, resource := range []string{"users", "orders"} {
		key, ok := request.ResourceCheckpointKey(resource)
		if !ok {
			t.Fatalf("no route key for %s", resource)
		}
		plan := checkpoint.KeysetCheckpoint{Cols: []string{"id"}, Types: []string{"bigint"}, Shards: []checkpoint.KeysetShard{{}}}.ToCheckpoint(resource)
		if err := ds.SaveResourceCheckpoint(ctx, "t1", filament.ResourceCheckpointState{Key: key, Run: "r1", Checkpoint: plan}); err != nil {
			t.Fatal(err)
		}
	}
	return m, ds, request
}

func routeKey(t *testing.T, ds *sqlite.Store, request filament.RunRequest, resource string) []string {
	t.Helper()
	key, _ := request.ResourceCheckpointKey(resource)
	stored, err := ds.LoadResourceCheckpoint(context.Background(), "t1", key)
	if err != nil {
		t.Fatal(err)
	}
	plan, ok := checkpoint.ParseKeyset(stored.Checkpoint)
	if !ok {
		t.Fatalf("%s checkpoint is not a keyset: %#v", resource, stored.Checkpoint.Raw())
	}
	return plan.Shards[0].Key
}

func written(t *testing.T, m *Module, seq uint64, resource, key string, policy filament.CheckpointPolicy) {
	t.Helper()
	deliver(t, m, seq, events.BatchWritten, events.Envelope{Tenant: "t1", Run: "r1", Resource: resource, At: time.Now()},
		events.BatchWrittenEvent{Records: 10, Bytes: 100, Checkpoint: checkpoint.NewShardDelta(resource, 0, []string{key}), CheckpointPolicy: policy})
}

func TestIsolatedCompletionPromotesOnlyHealthyCommitGatedCursors(t *testing.T) {
	m, ds, request := isolationRun(t)
	written(t, m, 1, "users", "42", filament.CheckpointAfterCommit)
	written(t, m, 2, "orders", "7", filament.CheckpointAfterCommit)
	env := func(resource string) events.Envelope {
		return events.Envelope{Tenant: "t1", Run: "r1", Resource: resource, At: time.Now()}
	}
	deliver(t, m, 3, events.ResourceFailed, env("orders"), events.ResourceFailedEvent{Error: "permission denied", Discarded: true})
	deliver(t, m, 4, events.ResourceCompleted, env("users"), events.ResourceCompletedEvent{Records: 10, Bytes: 100})
	deliver(t, m, 5, events.RunCompleted, env(""), events.RunCompletedEvent{Records: 10, Bytes: 100, FailedResources: []string{"orders"}})

	if got := routeKey(t, ds, request, "users"); len(got) != 1 || got[0] != "42" {
		t.Fatalf("users cursor = %v, want [42]", got)
	}
	if got := routeKey(t, ds, request, "orders"); len(got) != 0 {
		t.Fatalf("orders cursor = %v, want the unadvanced plan: its writes were aborted", got)
	}
	state, err := ds.LoadRun(context.Background(), "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Status != filament.RunCompleted || state.Records != 10 {
		t.Fatalf("run = %v records %d, want completed with 10", state.Status, state.Records)
	}
	for _, rs := range state.Resources {
		if rs.Resource == "orders" && (rs.Status != filament.RunFailed || rs.Records != 0) {
			t.Fatalf("orders = %v records %d, want failed with 0", rs.Status, rs.Records)
		}
	}
}

// The run's terminal alone must keep a failed commit-gated cursor back, even
// when resource.failed was never folded first.
func TestIsolatedCompletionSkipsFailedCursorWithoutResourceFact(t *testing.T) {
	m, ds, request := isolationRun(t)
	written(t, m, 1, "orders", "7", filament.CheckpointAfterCommit)
	deliver(t, m, 2, events.RunCompleted, events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()},
		events.RunCompletedEvent{FailedResources: []string{"orders"}})
	if got := routeKey(t, ds, request, "orders"); len(got) != 0 {
		t.Fatalf("orders cursor = %v, want the unadvanced plan", got)
	}
}

// Apply-durable rows stay in the destination when their resource fails later,
// so their cursor is promoted and the next run does not read them again.
func TestIsolatedCompletionPromotesApplyDurableFailedCursor(t *testing.T) {
	m, ds, request := isolationRun(t)
	written(t, m, 1, "orders", "7", filament.CheckpointAfterBatch)
	deliver(t, m, 2, events.ResourceFailed, events.Envelope{Tenant: "t1", Run: "r1", Resource: "orders", At: time.Now()},
		events.ResourceFailedEvent{Error: "permission denied"})
	deliver(t, m, 3, events.RunCompleted, events.Envelope{Tenant: "t1", Run: "r1", At: time.Now()},
		events.RunCompletedEvent{FailedResources: []string{"orders"}})
	if got := routeKey(t, ds, request, "orders"); len(got) != 1 || got[0] != "7" {
		t.Fatalf("orders cursor = %v, want [7]", got)
	}
}

func TestDiscardedResourceTalliesSubtractOnce(t *testing.T) {
	m, ds, _ := isolationRun(t)
	written(t, m, 1, "orders", "7", filament.CheckpointAfterCommit)
	failed := events.ResourceFailedEvent{Error: "boom", Discarded: true}
	env := events.Envelope{Tenant: "t1", Run: "r1", Resource: "orders", At: time.Now()}
	deliver(t, m, 2, events.ResourceFailed, env, failed)
	deliver(t, m, 3, events.ResourceFailed, env, failed) // a republished copy, new stream seq
	state, err := ds.LoadRun(context.Background(), "t1", "r1")
	if err != nil {
		t.Fatal(err)
	}
	if state.Records != 0 {
		t.Fatalf("run records = %d, want 0", state.Records)
	}
}
