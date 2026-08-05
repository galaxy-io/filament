package server

import (
	"testing"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

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
