package runner

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/datastore/sqlite"
)

type incrementalTestSource struct {
	previous map[string]filament.Checkpoint
	cursors  map[string]filament.ResourceCursorConfig
}

func TestScheduledCDCLoadsPipelineCheckpointAcrossRuns(t *testing.T) {
	ctx := context.Background()
	store := sqlite.NewMemory()
	spec := filament.RunSpec{
		Tenant: "tenant", Run: "run-b", PipelineID: "pipe", PipelineVersionID: "version-2",
		CheckpointRoute: "route/source/sink/cdc", Resources: []string{"users"},
	}
	key, _ := spec.ResourceCheckpointKey("users")
	want := checkpoint.NewStreamDelta("users", "0/16B6C50", 41)
	if err := store.SaveResourceCheckpoint(ctx, spec.Tenant, filament.ResourceCheckpointState{Key: key, Run: "run-a", Checkpoint: want}); err != nil {
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
	return filament.ConnectorSpec{
		Name: "test",
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionFullUpsert,
			filament.IngestionIncrementalUpsert,
		),
	}
}
func (*incrementalTestSource) Validate(filament.Config) error                   { return nil }
func (*incrementalTestSource) Configure(context.Context, filament.Config) error { return nil }
func (*incrementalTestSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}
func (*incrementalTestSource) Teardown(context.Context) error { return nil }
func (*incrementalTestSource) Schema(context.Context, string) (filament.RecordSchema, error) {
	return filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "updated_at", Logical: filament.LogicalTimestampTZ, Native: "timestamptz"},
		},
		PrimaryKey: []string{"id"},
	}, nil
}

func (*incrementalTestSource) CursorColumns(context.Context, string) ([]filament.CursorColumn, error) {
	return []filament.CursorColumn{{
		SchemaField: filament.SchemaField{Name: "updated_at", Logical: filament.LogicalTimestampTZ, Native: "timestamptz"},
		Eligible:    true, Recommended: true,
	}}, nil
}

func (*incrementalTestSource) ExtractFrom(context.Context, filament.RecordSink, filament.ExtractOpts, map[string]filament.Checkpoint) error {
	return nil
}

type incrementalTestSink struct{}

func (*incrementalTestSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Name: "test-sink", Capabilities: filament.SinkCapabilities{WritePolicies: filament.WriteCapabilities(filament.IngestionFullAppend, filament.IngestionFullReplace, filament.IngestionFullUpsert, filament.IngestionIncrementalUpsert)}}
}
func (*incrementalTestSink) Open(context.Context, filament.RunSpec) error { return nil }
func (*incrementalTestSink) Apply(context.Context, *arrowbatch.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, nil
}
func (*incrementalTestSink) Commit(context.Context) error { return nil }
func (*incrementalTestSink) Abort(context.Context) error  { return nil }
func (*incrementalTestSink) Name() string                 { return "test-sink" }

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
	store := sqlite.NewMemory()
	plan := filament.IngestionPlan{}
	base := filament.RunSpec{
		Tenant: "tenant", Run: "run-a", PipelineID: "pipe", PipelineVersionID: "version-1", CheckpointRoute: "route/source/sink",
		Source: filament.Ref{Connector: "test"}, Resources: []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
		CursorConfigs:  map[string]filament.ResourceCursorConfig{"users": {Field: "updated_at", LookbackSeconds: 300}},
	}
	first := &incrementalTestSource{}
	if _, err := resolveExtractor(ctx, store, first, base, plan, nil); err != nil {
		t.Fatal(err)
	}
	key, _ := base.ResourceCheckpointKey("users")
	seeded, err := store.LoadResourceCheckpoint(ctx, base.Tenant, key)
	if err != nil || seeded.Checkpoint.String("position") != "initial" {
		t.Fatalf("seeded checkpoint = %#v, err = %v", seeded, err)
	}

	advanced := filament.NewCheckpoint("users").Set("position", "next-run")
	if err := store.SaveResourceCheckpoint(ctx, base.Tenant, filament.ResourceCheckpointState{Key: key, Run: "run-a", Checkpoint: advanced}); err != nil {
		t.Fatal(err)
	}
	secondSpec := base
	secondSpec.Run = "run-b"
	second := &incrementalTestSource{}
	if _, err := resolveExtractor(ctx, store, second, secondSpec, plan, nil); err != nil {
		t.Fatal(err)
	}
	if second.previous["users"].String("position") != "next-run" {
		t.Fatalf("previous checkpoint = %#v", second.previous["users"].Raw())
	}
	if second.cursors["users"].Field != "updated_at" {
		t.Fatalf("cursor config = %#v", second.cursors)
	}
}

func TestResolveIngestionPlanCarriesCursorVersionToWritePolicy(t *testing.T) {
	plan, err := filament.ResolveIngestionPlan(context.Background(), &incrementalTestSource{}, &incrementalTestSink{}, filament.RunSpec{
		Resources:      []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
		CursorConfigs:  map[string]filament.ResourceCursorConfig{"users": {Field: "updated_at"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	policy := plan.WritePolicies["users"]
	if policy.Version.Strategy != filament.VersionCursor || policy.Version.Field != "updated_at" {
		t.Fatalf("version policy = %#v", policy.Version)
	}
}

func TestResolveIngestionPlanUsesInsertOrderForSnapshotUpsert(t *testing.T) {
	plan, err := filament.ResolveIngestionPlan(context.Background(), &incrementalTestSource{}, &incrementalTestSink{}, filament.RunSpec{
		Resources:      []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullUpsert},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.WritePolicies["users"].Version.Strategy; got != filament.VersionInsertOrder {
		t.Fatalf("version strategy = %q, want %q", got, filament.VersionInsertOrder)
	}
}

func TestIncrementalAppendFailureIsNotResumable(t *testing.T) {
	spec := filament.RunSpec{
		Resources:      []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalAppend},
	}
	plan := filament.IngestionPlan{WritePolicies: map[string]filament.WritePolicy{
		"users": filament.WritePolicyForIngestion(filament.IngestionIncrementalAppend),
	}}
	if isResumableRun(spec, plan) {
		t.Fatal("incremental append must abort instead of resuming into preserved append data")
	}

	spec.IngestionTypes["users"] = filament.IngestionIncrementalUpsert
	plan.WritePolicies["users"] = filament.WritePolicyForIngestion(filament.IngestionIncrementalUpsert)
	if !isResumableRun(spec, plan) {
		t.Fatal("incremental upsert should remain resumable")
	}

	policy := plan.WritePolicies["users"]
	policy.Checkpoint = filament.CheckpointAfterCommit
	plan.WritePolicies["users"] = policy
	if isResumableRun(spec, plan) {
		t.Fatal("commit-gated progress must fail instead of resuming before commit")
	}
}

type replayTestSource struct {
	incrementalTestSource
	attempts map[string]filament.Checkpoint
}

func (s *replayTestSource) PlanIncrementalResume(_ context.Context, planned, attempts map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	s.attempts = attempts
	for resource, cp := range attempts {
		planned[resource] = cp
	}
	return planned, nil
}

func TestIncrementalAttemptRestoreDoesNotPromoteJobs(t *testing.T) {
	ctx := t.Context()
	store := sqlite.NewMemory()
	for _, run := range []filament.RunID{"run-a", "run-b"} {
		if err := store.SaveRun(ctx, filament.RunState{Tenant: "tenant", Run: run}); err != nil {
			t.Fatal(err)
		}
	}
	spec := filament.RunSpec{
		Tenant: "tenant", Run: "run-a", PipelineID: "pipe", PipelineVersionID: "v1", CheckpointRoute: "route",
		Source: filament.Ref{Connector: "test"}, Resources: []string{"users"},
		IngestionTypes: map[string]filament.IngestionType{"users": filament.IngestionIncrementalUpsert},
	}
	cp := func(value string) filament.Checkpoint {
		return checkpoint.KeysetCheckpoint{Mode: checkpoint.ModeIncremental, Cols: []string{"export_state"}, Meta: map[string]string{checkpoint.ReplayOnResume: "true"}, Shards: []checkpoint.KeysetShard{{Key: []string{value}}}}.ToCheckpoint("users")
	}
	key, _ := spec.ResourceCheckpointKey("users")
	if err := store.SaveResourceCheckpoint(ctx, spec.Tenant, filament.ResourceCheckpointState{Key: key, Run: "old", Checkpoint: cp("committed")}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCheckpoint(ctx, spec.Tenant, spec.Run, cp("active")); err != nil {
		t.Fatal(err)
	}
	src := &replayTestSource{}
	if _, err := resolveExtractor(ctx, store, src, spec, filament.IngestionPlan{}, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := checkpoint.ParseKeyset(src.attempts["users"])
	if got.Shards[0].Key[0] != "active" {
		t.Fatal("attempt not restored")
	}
	durable, err := store.LoadResourceCheckpoint(ctx, spec.Tenant, key)
	if err != nil {
		t.Fatal(err)
	}
	got, _ = checkpoint.ParseKeyset(durable.Checkpoint)
	if got.Shards[0].Key[0] != "committed" {
		t.Fatal("attempt leaked into durable state")
	}
	spec.Run = "run-b"
	if _, err := resolveExtractor(ctx, store, src, spec, filament.IngestionPlan{}, nil); err != nil {
		t.Fatal(err)
	}
	if len(src.attempts) != 0 {
		t.Fatal("new run inherited failed run's jobs")
	}
	if _, err := resolveExtractor(ctx, store, &incrementalTestSource{}, spec, filament.IngestionPlan{}, nil); err == nil {
		t.Fatal("accepted replay state without resume contract")
	}
}
