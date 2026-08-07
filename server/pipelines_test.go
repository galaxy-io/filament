package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestListPipelinesIncludeDeleted(t *testing.T) {
	ctx := context.Background()
	api := newConnectionsTestServer()
	if _, err := api.store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: "t1", Name: "doomed"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if err := api.store.DeletePipeline(ctx, "pipe-1"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	live, err := api.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{TenantId: "t1"}))
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if got := len(live.Msg.GetPipelines()); got != 0 {
		t.Fatalf("ListPipelines returned %d pipelines, want 0", got)
	}

	withDeleted, err := api.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{
		TenantId:       "t1",
		IncludeDeleted: true,
	}))
	if err != nil {
		t.Fatalf("ListPipelines with IncludeDeleted: %v", err)
	}
	pipelines := withDeleted.Msg.GetPipelines()
	if len(pipelines) != 1 {
		t.Fatalf("ListPipelines with IncludeDeleted returned %d pipelines, want 1", len(pipelines))
	}
}

func TestValidateCursorConfigs(t *testing.T) {
	valid := []*ingestionv1.PipelineEdge{{
		Resource:      "users",
		IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_UPSERT,
		Cursors:       []*ingestionv1.ResourceCursorConfig{{Resource: "users", Field: "updated_at", LookbackSeconds: 300}},
	}}
	if err := validateCursorConfigs(valid); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		edge *ingestionv1.PipelineEdge
	}{
		{"full ingestion", &ingestionv1.PipelineEdge{Resource: "users", IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_UPSERT, Cursors: valid[0].Cursors}},
		{"missing resource", &ingestionv1.PipelineEdge{IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_UPSERT, Cursors: []*ingestionv1.ResourceCursorConfig{{Field: "updated_at"}}}},
		{"wrong resource", &ingestionv1.PipelineEdge{Resource: "users", IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_UPSERT, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "orders", Field: "updated_at"}}}},
		{"missing field", &ingestionv1.PipelineEdge{Resource: "users", IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_UPSERT, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "users"}}}},
		{"negative lookback", &ingestionv1.PipelineEdge{Resource: "users", IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_UPSERT, Cursors: []*ingestionv1.ResourceCursorConfig{{Resource: "users", Field: "updated_at", LookbackSeconds: -1}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCursorConfigs([]*ingestionv1.PipelineEdge{tt.edge}); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
