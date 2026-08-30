package runner

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/memory"
)

func TestWaitForDurableStreamCheckpointsRejectsOlderCursor(t *testing.T) {
	store := memory.New()
	spec := filament.RunSpec{
		Run: "run", PipelineID: "pipeline", PipelineVersionID: "version",
		CheckpointRoute: "route", Resources: []string{"users"},
	}
	key, _ := spec.ResourceCheckpointKey("users")
	if err := store.SaveResourceCheckpoint(context.Background(), filament.ResourceCheckpointState{
		Key: key, Run: "older", Checkpoint: checkpoint.NewStreamDelta("users", "0/10", 4),
	}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := waitForDurableStreamCheckpoints(ctx, store, spec, map[string]filament.Checkpoint{
		"users": checkpoint.NewStreamDelta("users", "0/20", 4),
	})
	if err == nil {
		t.Fatal("older durable cursor satisfied final checkpoint")
	}
}

func TestWaitForDurableStreamCheckpointsReturnsPromotedCursor(t *testing.T) {
	store := memory.New()
	spec := filament.RunSpec{
		Run: "run", PipelineID: "pipeline", PipelineVersionID: "version",
		CheckpointRoute: "route", Resources: []string{"users"},
	}
	target := checkpoint.NewStreamDelta("users", "0/20", 4)
	key, _ := spec.ResourceCheckpointKey("users")
	if err := store.SaveResourceCheckpoint(context.Background(), filament.ResourceCheckpointState{
		Key: key, Run: spec.Run, Checkpoint: target,
	}); err != nil {
		t.Fatal(err)
	}
	durable, err := waitForDurableStreamCheckpoints(context.Background(), store, spec, map[string]filament.Checkpoint{"users": target})
	if err != nil {
		t.Fatal(err)
	}
	if !streamCheckpointReached(durable["users"], target) {
		t.Fatalf("durable checkpoint = %#v, want target %#v", durable["users"], target)
	}
}
