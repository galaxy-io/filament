package runner

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/memory"
)

type incrementalTestSource struct {
	previous map[string]filament.Checkpoint
	cursors  map[string]filament.ResourceCursorConfig
}

func TestScheduledCDCLoadsPipelineCheckpointAcrossRuns(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	spec := filament.RunSpec{
		Run: "run-b", PipelineID: "pipe", PipelineVersionID: 2,
		CheckpointRoute: "route/source/sink/cdc", Resources: []string{"users"},
	}
	key, _ := spec.ResourceCheckpointKey("users")
	want := checkpoint.NewStreamDelta("users", "0/16B6C50", 41)
	if err := store.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: "run-a", Checkpoint: want}); err != nil {
		t.Fatal(err)
	}
	got, err := loadChangeCheckpoints(ctx, store, spec)
	if err != nil {
		t.Fatal(err)
	}
	lsn, seq, ok := checkpoint.ParseStream(got["users"])
	if !ok || lsn != "0/16B6C50" || seq != 41 {
		t.Fatalf("scheduled checkpoint = %#v", got["users"])
	}
}

func (*incrementalTestSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "test"}
}
func (*incrementalTestSource) Validate(filament.Config) error                   { return nil }
func (*incrementalTestSource) Configure(context.Context, filament.Config) error { return nil }
func (*incrementalTestSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}
func (*incrementalTestSource) Teardown(context.Context) error { return nil }
func (*incrementalTestSource) ExtractFrom(context.Context, filament.RecordSink, filament.ExtractOpts, map[string]filament.Checkpoint) error {
	return nil
}

func (s *incrementalTestSource) PlanIncremental(_ context.Context, resources []string, prev map[string]filament.Checkpoint, cursors map[string]filament.ResourceCursorConfig) (map[string]filament.Checkpoint, error) {
	s.previous, s.cursors = prev, cursors
	out := make(map[string]filament.Checkpoint, len(resources))
	for _, resource := range resources {
		if cp := prev[resource]; cp != nil {
			out[resource] = cp
		} else {
			out[resource] = filament.NewCheckpoint(resource).Set("position", "initial")
		}
	}
	return out, nil
}

func TestResolveExtractorCarriesCheckpointAcrossRuns(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	plan := filament.IngestionPlan{
		Type:         filament.IngestionIncrementalUpsert,
		SourcePolicy: filament.SourcePolicy{Mode: filament.ModeIncremental, Checkpointing: filament.CheckpointAfterBatch},
	}
	base := filament.RunSpec{
		Run: "run-a", PipelineID: "pipe", PipelineVersionID: 1, CheckpointRoute: "route/source/sink/upsert",
		Source: filament.Ref{Provider: "test"}, Resources: []string{"users"},
		CursorConfigs: map[string]filament.ResourceCursorConfig{"users": {Field: "updated_at", LookbackSeconds: 300}},
	}
	first := &incrementalTestSource{}
	if _, err := resolveExtractor(ctx, store, first, base, plan); err != nil {
		t.Fatal(err)
	}
	key, _ := base.ResourceCheckpointKey("users")
	seeded, err := store.LoadResourceCheckpoint(ctx, key)
	if err != nil || seeded.Checkpoint.String("position") != "initial" {
		t.Fatalf("seeded checkpoint = %#v, err = %v", seeded, err)
	}

	advanced := filament.NewCheckpoint("users").Set("position", "next-run")
	if err := store.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: "run-a", Checkpoint: advanced}); err != nil {
		t.Fatal(err)
	}
	secondSpec := base
	secondSpec.Run = "run-b"
	second := &incrementalTestSource{}
	if _, err := resolveExtractor(ctx, store, second, secondSpec, plan); err != nil {
		t.Fatal(err)
	}
	if second.previous["users"].String("position") != "next-run" {
		t.Fatalf("previous checkpoint = %#v", second.previous["users"].Raw())
	}
	if second.cursors["users"].Field != "updated_at" {
		t.Fatalf("cursor config = %#v", second.cursors)
	}
}
