package runner

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/sqlite"
)

type ackTestSource struct {
	incrementalTestSource
	acknowledged map[string]filament.Checkpoint
}

func (s *ackTestSource) AcknowledgeChanges(_ context.Context, cps map[string]filament.Checkpoint) error {
	s.acknowledged = cps
	return nil
}

func ackTestSpec() filament.RunSpec {
	return filament.RunSpec{
		Tenant: "tenant", Run: "run", PipelineID: "pipeline", PipelineVersionID: "version",
		CheckpointRoute: "route", Resources: []string{"users"},
	}
}

func saveRouteCheckpoint(t *testing.T, store *sqlite.Store, spec filament.RunSpec, resource string, run filament.RunID, cp filament.Checkpoint) {
	t.Helper()
	key, _ := spec.ResourceCheckpointKey(resource)
	if err := store.SaveResourceCheckpoint(context.Background(), spec.Tenant, filament.ResourceCheckpointState{Key: key, Run: run, Checkpoint: cp}); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForDurableStreamCheckpointsRejectsOlderCursor(t *testing.T) {
	store := sqlite.NewMemory()
	spec := ackTestSpec()
	saveRouteCheckpoint(t, store, spec, "users", "older", checkpoint.NewStreamDelta("users", "0/10", 4))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := waitForDurableStreamCheckpoints(ctx, store, spec, map[string]filament.Checkpoint{
		"users": checkpoint.NewStreamDelta("users", "0/20", 4),
	})
	if err == nil {
		t.Fatal("older durable cursor satisfied final checkpoint")
	}
}

func TestWaitForDurableStreamCheckpointsReturnsOncePromoted(t *testing.T) {
	store := sqlite.NewMemory()
	spec := ackTestSpec()
	target := checkpoint.NewStreamDelta("users", "0/20", 4)
	saveRouteCheckpoint(t, store, spec, "users", spec.Run, target)
	if err := waitForDurableStreamCheckpoints(context.Background(), store, spec, map[string]filament.Checkpoint{"users": target}); err != nil {
		t.Fatal(err)
	}
}

// A run that covered only users must still hand the source the older orders
// cursor, so the stream is never released past a resource the run left out.
// Cursors that are not stream positions share the route but not the stream.
func TestAcknowledgeDurableChangesUsesRouteFloor(t *testing.T) {
	store := sqlite.NewMemory()
	spec := ackTestSpec()
	target := checkpoint.NewStreamDelta("users", "0/20", 4)
	saveRouteCheckpoint(t, store, spec, "users", spec.Run, target)
	saveRouteCheckpoint(t, store, spec, "orders", "older", checkpoint.NewStreamDelta("orders", "0/10", 2))
	saveRouteCheckpoint(t, store, spec, "events", "older", &filament.CheckpointData{ResourceName: "events", Cursor: map[string]any{"cursor": "2026-01-01"}})

	src := &ackTestSource{}
	acknowledgeDurableChanges(context.Background(), Deps{DataStore: store}, spec, src, map[string]filament.Checkpoint{"users": target})

	if _, ok := src.acknowledged["orders"]; !ok {
		t.Fatalf("acknowledged = %v, want the omitted orders cursor", src.acknowledged)
	}
	if !streamCheckpointReached(src.acknowledged["users"], target) {
		t.Fatalf("users cursor = %#v, want %#v", src.acknowledged["users"], target)
	}
	if _, ok := src.acknowledged["events"]; ok {
		t.Fatal("incremental cursor was offered as a stream position")
	}
}

// At run start nothing is pending, so whatever the store holds is acknowledged
// as is: this repeats an acknowledgement the previous run could not finish.
func TestAcknowledgeDurableChangesAtRunStart(t *testing.T) {
	store := sqlite.NewMemory()
	spec := ackTestSpec()
	saveRouteCheckpoint(t, store, spec, "users", "older", checkpoint.NewStreamDelta("users", "0/20", 4))
	saveRouteCheckpoint(t, store, spec, "orders", "older", checkpoint.NewStreamDelta("orders", "0/10", 2))

	src := &ackTestSource{}
	acknowledgeDurableChanges(context.Background(), Deps{DataStore: store}, spec, src, nil)

	if len(src.acknowledged) != 2 {
		t.Fatalf("acknowledged = %v, want both route cursors", src.acknowledged)
	}
}
