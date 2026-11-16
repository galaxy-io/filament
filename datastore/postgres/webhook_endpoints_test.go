package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/webhooks"
)

func TestWebhookEndpointRejectsStalePipelineVersion(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if _, err := store.CreateConnection(ctx, filament.Connection{
		ID: connectionOne, Tenant: tenantA, Kind: filament.ConnectorKindSource,
		Name: "webhook-source", Connector: "webhook",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineOne, TenantId: tenantA, Name: "webhook-pipeline"}); err != nil {
		t.Fatal(err)
	}
	oldVersion, err := store.CreatePipelineVersion(ctx, tenantA, pipelineOne, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatal(err)
	}
	newVersion, err := store.CreatePipelineVersion(ctx, tenantA, pipelineOne, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenantA, SourceID: connectionOne, PipelineID: pipelineOne,
		RouteKey: "route/source/sink", SecretRefs: []string{"test-key"},
		ExpectedPipelineVersionID: oldVersion.Id,
	}
	mutation.ApplyDefaults()
	if _, err := store.PutWebhookEndpoint(ctx, mutation); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale pipeline version: got %v", err)
	}
	mutation.ExpectedPipelineVersionID = newVersion.Id
	if _, err := store.PutWebhookEndpoint(ctx, mutation); err != nil {
		t.Fatalf("current pipeline version: %v", err)
	}
	mutation.ExpectedRevision = 1
	mutation.ExpectedPipelineVersionID = oldVersion.Id
	if _, err := store.PutWebhookEndpoint(ctx, mutation); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale update: got %v", err)
	}
}
