package server

import (
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/internal/runs"
	"github.com/galaxy-io/filament/registry"
)

func TestCreatePipelinePersistsDefaultWorkerConfiguration(t *testing.T) {
	ctx := testCtx()
	store := memory.New()
	api := New(registry.NewSources(), registry.NewSinks(), store, nil, nil)

	res, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: "defaults"}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	want := defaultWorkerConfiguration()
	if want.NodeSelector == nil || want.Tolerations == nil {
		t.Fatalf("default worker configuration must include empty node selector and tolerations: %+v", want)
	}
	if got := res.Msg.GetPipeline().GetWorkerConfiguration(); !proto.Equal(got, want) {
		t.Fatalf("response worker configuration = %+v, want %+v", got, want)
	}
	stored, err := store.LoadPipeline(ctx, filament.DefaultTenantID, res.Msg.GetPipeline().GetId())
	if err != nil {
		t.Fatalf("LoadPipeline: %v", err)
	}
	if got := stored.GetWorkerConfiguration(); !proto.Equal(got, want) {
		t.Fatalf("stored worker configuration = %+v, want %+v", got, want)
	}
}

func TestCreatePipelineMergesWorkerConfigurationWithDefaults(t *testing.T) {
	ctx := testCtx()
	store := memory.New()
	api := New(registry.NewSources(), registry.NewSinks(), store, nil, nil)

	res, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{
		Name: "partial worker configuration",
		WorkerConfiguration: &ingestionv1.WorkerConfiguration{
			Resources:    &ingestionv1.WorkerResources{Requests: map[string]string{"cpu": "750m"}},
			NodeSelector: map[string]string{"pool": "batch"},
			Tolerations:  []*ingestionv1.WorkerToleration{{Key: "dedicated", Value: "batch"}},
		},
	}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}

	got := res.Msg.GetPipeline().GetWorkerConfiguration()
	if got.GetResources().GetRequests()["cpu"] != "750m" || got.GetResources().GetRequests()["memory"] != "256Mi" {
		t.Fatalf("merged requests = %v, want cpu override and default memory", got.GetResources().GetRequests())
	}
	if !proto.Equal(got, &ingestionv1.WorkerConfiguration{
		Resources: &ingestionv1.WorkerResources{
			Requests: map[string]string{"cpu": "750m", "memory": "256Mi"},
			Limits:   map[string]string{"cpu": "1000m", "memory": "512Mi"},
		},
		NodeSelector: map[string]string{"pool": "batch"},
		Tolerations:  []*ingestionv1.WorkerToleration{{Key: "dedicated", Value: "batch"}},
	}) {
		t.Fatalf("merged worker configuration = %+v", got)
	}
}

// TestGetPipelineIncludesDeleted covers the soft-delete read path: a deleted
// pipeline stays readable by id — with its versions and deleted_at — while
// running it is refused.
func TestGetPipelineIncludesDeleted(t *testing.T) {
	ctx := testCtx()
	store := memory.New()
	api := New(registry.NewSources(), registry.NewSinks(), store, nil, nil)

	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: string(filament.DefaultTenantID), Name: "doomed"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if _, err := store.CreatePipelineVersion(ctx, filament.DefaultTenantID, "pipe-1", &ingestionv1.PipelineVersion{}); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if err := store.DeletePipeline(ctx, filament.DefaultTenantID, "pipe-1"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	res, err := api.GetPipeline(ctx, connect.NewRequest(&ingestionv1.GetPipelineRequest{Id: "pipe-1", IncludeVersions: true}))
	if err != nil {
		t.Fatalf("GetPipeline after delete: %v", err)
	}
	if res.Msg.GetPipeline().GetDeletedAt() == 0 {
		t.Fatalf("expected deleted_at set, got %+v", res.Msg.GetPipeline())
	}
	stamp, ok := strings.CutPrefix(res.Msg.GetPipeline().GetName(), "doomed__deleted__")
	if !ok {
		t.Fatalf("expected delete stamp on the name, got %q", res.Msg.GetPipeline().GetName())
	}
	stampedAt, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatalf("delete stamp %q is not RFC3339: %v", stamp, err)
	}
	if stampedAt.UnixMilli() != res.Msg.GetPipeline().GetDeletedAt() {
		t.Fatalf("delete stamp %d disagrees with deleted_at %d", stampedAt.UnixMilli(), res.Msg.GetPipeline().GetDeletedAt())
	}
	if len(res.Msg.GetPipeline().GetVersions()) != 1 || res.Msg.GetPipeline().GetCurrentVersion() == nil {
		t.Fatalf("expected version history kept, got %+v", res.Msg)
	}

	_, err = api.RunPipeline(ctx, connect.NewRequest(&ingestionv1.RunPipelineRequest{PipelineId: "pipe-1"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected FailedPrecondition running a deleted pipeline, got %v", err)
	}
}

// TestDeletePipelineDropsScheduledRuns covers the reap ordering: the store
// delete cascades the schedule row away, so the schedule id must be captured
// before the delete or the pending pre-created runs are orphaned.
func TestDeletePipelineDropsScheduledRuns(t *testing.T) {
	ctx := testCtx()
	store := memory.New()
	api := New(registry.NewSources(), registry.NewSinks(), store, nil, nil)

	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: string(filament.DefaultTenantID), Name: "scheduled"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	fire := time.Now().Add(time.Hour)
	if err := store.SaveSchedule(ctx, filament.ScheduleState{
		ID:       "sched-1",
		Spec:     filament.ScheduleSpec{Tenant: filament.DefaultTenantID, Name: "scheduled", PipelineID: "pipe-1", Cron: "0 * * * *", Timezone: "UTC", Enabled: true},
		Enabled:  true,
		NextFire: &fire,
	}); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	if _, err := runs.Schedule(ctx, store, filament.RunRequest{Tenant: filament.DefaultTenantID, PipelineID: "pipe-1", IdempotencyKey: "occ-1", ScheduleID: "sched-1", ScheduledFor: fire}); err != nil {
		t.Fatalf("Schedule: %v", err)
	}

	if _, err := api.DeletePipeline(ctx, connect.NewRequest(&ingestionv1.DeletePipelineRequest{Id: "pipe-1"})); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	pending, _, err := store.ListRuns(ctx, filament.RunFilter{Schedule: "sched-1", Status: []filament.RunStatus{filament.RunScheduled}})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected pending scheduled runs reaped on pipeline delete, got %d", len(pending))
	}
}

func TestUpdatePipelineUsesExplicitMutableFields(t *testing.T) {
	ctx := testCtx()
	store := memory.New()
	api := New(registry.NewSources(), registry.NewSinks(), store, nil, nil)
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{
		Id: "pipe-1", TenantId: string(filament.DefaultTenantID), Name: "old", Description: "old description",
		WorkerConfiguration: &ingestionv1.WorkerConfiguration{Resources: &ingestionv1.WorkerResources{Requests: map[string]string{"cpu": "250m"}}},
	}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	version, err := store.CreatePipelineVersion(ctx, filament.DefaultTenantID, "pipe-1", &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}

	res, err := api.UpdatePipeline(ctx, connect.NewRequest(&ingestionv1.UpdatePipelineRequest{
		PipelineId: "pipe-1", Name: "new", Description: "new description",
		WorkerConfiguration: &ingestionv1.WorkerConfiguration{Resources: &ingestionv1.WorkerResources{Requests: map[string]string{"cpu": "500m"}}},
	}))
	if err != nil {
		t.Fatalf("UpdatePipeline: %v", err)
	}
	if got := res.Msg.GetPipeline(); got.GetName() != "new" || got.GetDescription() != "new description" {
		t.Fatalf("updated pipeline = %+v", got)
	} else if got.GetWorkerConfiguration().GetResources().GetRequests()["cpu"] != "500m" {
		t.Fatalf("worker configuration = %+v, want cpu request 500m", got.GetWorkerConfiguration())
	} else if got.GetCurrentVersion().GetId() != version.GetId() {
		t.Fatalf("current version = %q, want %q", got.GetCurrentVersion().GetId(), version.GetId())
	}

	_, err = api.UpdatePipeline(ctx, connect.NewRequest(&ingestionv1.UpdatePipelineRequest{}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("missing pipeline_id error = %v, want invalid argument", err)
	}
}

func TestValidateCursorConfigs(t *testing.T) {
	valid := []*ingestionv1.PipelineEdge{{
		Resource: "users",
		ReadMode: ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
		Cursors:  []*ingestionv1.ResourceCursorConfig{{Resource: "users", Field: "updated_at", LookbackSeconds: 300}},
	}}
	if err := validateCursorConfigs(valid); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		edge *ingestionv1.PipelineEdge
	}{
		{"Full", &ingestionv1.PipelineEdge{Resource: "users", ReadMode: ingestionv1.ReadMode_READ_MODE_FULL, Cursors: valid[0].Cursors}},
		{"missing resource", &ingestionv1.PipelineEdge{ReadMode: ingestionv1.ReadMode_READ_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Field: "updated_at"}}}},
		{"wrong resource", &ingestionv1.PipelineEdge{Resource: "users", ReadMode: ingestionv1.ReadMode_READ_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "orders", Field: "updated_at"}}}},
		{"missing field", &ingestionv1.PipelineEdge{Resource: "users", ReadMode: ingestionv1.ReadMode_READ_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "users"}}}},
		{"negative lookback", &ingestionv1.PipelineEdge{Resource: "users", ReadMode: ingestionv1.ReadMode_READ_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "users", Field: "updated_at", LookbackSeconds: -1}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCursorConfigs([]*ingestionv1.PipelineEdge{tt.edge}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestCreatePipelineVersionRejectsResourceRequirements(t *testing.T) {
	ctx := testCtx()
	api, ids := leverAPI(t)
	pipelineResp, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: "validated"}))
	if err != nil {
		t.Fatal(err)
	}
	pipelineID := pipelineResp.Msg.GetPipeline().GetId()
	nodes := []*ingestionv1.PipelineNode{
		{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: ids["standard"]},
		{Id: "snk", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: ids["sink"]},
	}

	_, err = api.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: pipelineID,
		Graph: &ingestionv1.PipelineGraph{
			Nodes: nodes,
			Edges: []*ingestionv1.PipelineEdge{{
				FromNode: "src", ToNode: "snk", Resource: "audit",
				ReadMode:  ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
				WriteMode: ingestionv1.WriteMode_WRITE_MODE_UPSERT,
			}},
		},
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("invalid Incremental resource: got %v, want invalid argument", err)
	}

	_, err = api.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: pipelineID,
		Graph: &ingestionv1.PipelineGraph{
			Nodes: nodes,
			Edges: []*ingestionv1.PipelineEdge{{
				FromNode: "src", ToNode: "snk", Resource: "orders",
				ReadMode:  ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
				WriteMode: ingestionv1.WriteMode_WRITE_MODE_UPSERT,
			}},
		},
	}))
	if err != nil {
		t.Fatalf("valid Incremental resource: %v", err)
	}
}
