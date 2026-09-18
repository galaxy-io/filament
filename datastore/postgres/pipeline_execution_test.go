package postgres_test

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestPipelineExecutionPersistence(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	p, err := store.CreatePipelineWithSchedule(ctx, &ingestionv1.Pipeline{Id: pipelineOne, TenantId: tenantA, Name: "events", ExecutionMode: ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if p.ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
		t.Fatal("create lost mode")
	}
	p, err = store.UpdatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineOne, TenantId: tenantA, Name: "renamed"})
	if err != nil || p.ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
		t.Fatalf("update/load: %v %v", p, err)
	}
	list, _, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: tenantA})
	if err != nil || len(list) != 1 || list[0].ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
		t.Fatalf("list: %v %v", list, err)
	}
	p, err = store.UpdatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineOne, TenantId: tenantA, Name: "bounded", ExecutionMode: ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED})
	if err != nil || p.ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED {
		t.Fatalf("explicit mode: %v %v", p, err)
	}
	p, err = store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineDeleted, TenantId: tenantA, Name: "legacy"})
	if err != nil || p.ExecutionMode != ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED {
		t.Fatalf("default: %v %v", p, err)
	}
}
