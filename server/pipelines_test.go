package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/registry"
)

// TestGetPipelineIncludesDeleted covers the soft-delete read path: a deleted
// pipeline stays readable by id — with its versions and deleted_at — while
// running it is refused.
func TestGetPipelineIncludesDeleted(t *testing.T) {
	ctx := context.Background()
	store := memory.New()
	api := New(registry.NewSources(), registry.NewSinks(), store, nil, nil)

	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: "t1", Name: "doomed"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if _, err := store.CreatePipelineVersion(ctx, "pipe-1", &ingestionv1.PipelineVersion{}); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if err := store.DeletePipeline(ctx, "pipe-1"); err != nil {
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
	if len(res.Msg.GetPipeline().GetVersions()) != 0 || res.Msg.GetPipeline().GetCurrentVersion() == nil {
		t.Fatalf("expected version history kept, got %+v", res.Msg)
	}

	_, err = api.RunPipeline(ctx, connect.NewRequest(&ingestionv1.RunPipelineRequest{PipelineId: "pipe-1"}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("expected FailedPrecondition running a deleted pipeline, got %v", err)
	}
}

func TestValidateCursorConfigs(t *testing.T) {
	valid := []*ingestionv1.PipelineEdge{{
		Resource:         "users",
		StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL,
		Cursors:          []*ingestionv1.ResourceCursorConfig{{Resource: "users", Field: "updated_at", LookbackSeconds: 300}},
	}}
	if err := validateCursorConfigs(valid); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		edge *ingestionv1.PipelineEdge
	}{
		{"Replace", &ingestionv1.PipelineEdge{Resource: "users", StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE, Cursors: valid[0].Cursors}},
		{"missing resource", &ingestionv1.PipelineEdge{StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Field: "updated_at"}}}},
		{"wrong resource", &ingestionv1.PipelineEdge{Resource: "users", StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "orders", Field: "updated_at"}}}},
		{"missing field", &ingestionv1.PipelineEdge{Resource: "users", StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "users"}}}},
		{"negative lookback", &ingestionv1.PipelineEdge{Resource: "users", StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "users", Field: "updated_at", LookbackSeconds: -1}}}},
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
	ctx := context.Background()
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
				StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL,
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
				StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL,
			}},
		},
	}))
	if err != nil {
		t.Fatalf("valid Incremental resource: %v", err)
	}
}
