package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
	"github.com/galaxy-io/filament/webhooks"
)

func TestWebhookEndpointRevisionTenantAndPipelineFence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant := filament.TenantID("tenant-one")
	if err := store.EnsureTenant(ctx, tenant, "Tenant One"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnection(ctx, filament.Connection{
		ID: "webhook-source", Tenant: string(tenant), Kind: filament.ConnectorKindSource,
		Name: "source", Connector: "webhook",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipeline-one", TenantId: string(tenant), Name: "pipeline"}); err != nil {
		t.Fatal(err)
	}
	oldVersion, err := store.CreatePipelineVersion(ctx, tenant, "pipeline-one", &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatal(err)
	}
	newVersion, err := store.CreatePipelineVersion(ctx, tenant, "pipeline-one", &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: "webhook-source", PipelineID: "pipeline-one", RouteKey: "route/source/sink",
		SecretRefs: []string{"key"}, Resources: []string{"orders"}, ExpectedPipelineVersionID: oldVersion.Id,
	}
	mutation.ApplyDefaults()
	if _, err := store.PutWebhookEndpoint(ctx, mutation); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale pipeline version: %v", err)
	}
	mutation.ExpectedPipelineVersionID = newVersion.Id
	endpoint, err := store.PutWebhookEndpoint(ctx, mutation)
	if err != nil || endpoint.Revision != 1 {
		t.Fatalf("create endpoint: %+v, %v", endpoint, err)
	}
	if _, err := store.LoadWebhookEndpointForTenant(ctx, "other-tenant", mutation.SourceID); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("cross-tenant read: %v", err)
	}
	mutation.ExpectedRevision = 1
	mutation.ExpectedPipelineVersionID = oldVersion.Id
	if _, err := store.PutWebhookEndpoint(ctx, mutation); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale update: %v", err)
	}
	mutation.ExpectedPipelineVersionID = newVersion.Id
	endpoint, err = store.PutWebhookEndpoint(ctx, mutation)
	if err != nil || endpoint.Revision != 2 {
		t.Fatalf("update endpoint: %+v, %v", endpoint, err)
	}
	if _, err := store.PutWebhookEndpoint(ctx, mutation); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	releases, err := store.WebhookReleases(ctx, "WEBHOOK_INBOX", time.Now())
	if err != nil || len(releases) != 1 || !releases[0].Orphaned {
		t.Fatalf("orphaned endpoint release: %+v, %v", releases, err)
	}
}

// Receiving follows the stream's existence, not its state. A new stream starts
// stopped, so it already covers a stopped pipeline.
func TestWebhookEndpointAcceptsOnceAStreamReadsTheSource(t *testing.T) {
	ctx := context.Background()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant := filament.TenantID("tenant-one")
	if err := store.EnsureTenant(ctx, tenant, "Tenant One"); err != nil {
		t.Fatal(err)
	}
	for id, kind := range map[string]filament.ConnectorKind{"webhook-source": filament.ConnectorKindSource, "sink": filament.ConnectorKindSink} {
		if _, err := store.CreateConnection(ctx, filament.Connection{ID: id, Tenant: string(tenant), Kind: kind, Name: id, Connector: "webhook"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipeline-one", TenantId: string(tenant), Name: "pipeline"}); err != nil {
		t.Fatal(err)
	}
	version, err := store.CreatePipelineVersion(ctx, tenant, "pipeline-one", &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: "webhook-source", PipelineID: "pipeline-one", RouteKey: "route/source/sink",
		SecretRefs: []string{"key"}, Resources: []string{"orders"}, Enabled: true, ExpectedPipelineVersionID: version.Id,
	}
	mutation.ApplyDefaults()
	endpoint, err := store.PutWebhookEndpoint(ctx, mutation)
	if err != nil || endpoint.Accepting {
		t.Fatalf("endpoint with no stream: accepting=%v, %v", endpoint.Accepting, err)
	}
	if _, err := store.q.CreateReplicationStream(ctx, sqlcgen.CreateReplicationStreamParams{
		ReplicationStreamID: "stream", TenantID: string(tenant), PipelineID: "pipeline-one", RouteKey: mutation.RouteKey, Generation: 1,
		SourceConnectionID: "webhook-source", SinkConnectionID: "sink", ConsumerName: "consumer", ConsumerConfig: "{}",
		ContinuityFingerprint: "fingerprint", CreatedFromPipelineVersionID: version.Id, CreatedAt: 10, UpdatedAt: 10,
	}); err != nil {
		t.Fatal(err)
	}
	var desired string
	if err := store.db.QueryRowContext(ctx, `SELECT desired_state FROM replication_streams WHERE id = 'stream'`).Scan(&desired); err != nil || desired != string(filament.StreamStopped) {
		t.Fatalf("new stream desired state %q, %v", desired, err)
	}
	endpoint, err = store.LoadWebhookEndpoint(ctx, mutation.SourceID)
	if err != nil || !endpoint.Accepting {
		t.Fatalf("endpoint with a stopped stream: accepting=%v, %v", endpoint.Accepting, err)
	}
}

func TestWebhookEndpointVerifierParamsRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant := filament.TenantID("tenant-one")
	if err := store.EnsureTenant(ctx, tenant, "Tenant One"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnection(ctx, filament.Connection{
		ID: "posthog-source", Tenant: string(tenant), Kind: filament.ConnectorKindSource,
		Name: "source", Connector: "posthog",
	}); err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: "posthog-source", SecretRefs: []string{"key"},
		Verifier:       webhooks.SharedSecretHeader,
		VerifierParams: []byte(`{"delivery_id":{"body":"event.uuid"}}`),
	}
	mutation.ApplyDefaults()
	if _, err := store.PutWebhookEndpoint(ctx, mutation); err != nil {
		t.Fatal(err)
	}
	endpoint, err := store.LoadWebhookEndpointForTenant(ctx, tenant, mutation.SourceID)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Verifier != webhooks.SharedSecretHeader || string(endpoint.VerifierParams) != `{"delivery_id":{"body":"event.uuid"}}` {
		t.Fatalf("verifier %q params %s", endpoint.Verifier, endpoint.VerifierParams)
	}
	if _, err := webhooks.BuildVerifier(endpoint.Verifier, endpoint.VerifierParams); err != nil {
		t.Fatal(err)
	}
}

func TestWebhookEndpointIngressRoundTrip(t *testing.T) {
	ctx := context.Background()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant := filament.TenantID("tenant-one")
	if err := store.EnsureTenant(ctx, tenant, "Tenant One"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnection(ctx, filament.Connection{
		ID: "github-source", Tenant: string(tenant), Kind: filament.ConnectorKindSource,
		Name: "source", Connector: "github",
	}); err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: "github-source", SecretRefs: []string{"key"}, Verifier: webhooks.StandardWebhooks,
		Ingress: webhooks.Ingress{Route: &webhooks.Route{
			Event: webhooks.ValueSource{Header: "X-Github-Event"}, Events: map[string]string{"push": "pushes"},
		}},
	}
	mutation.ApplyDefaults()
	endpoint, err := store.PutWebhookEndpoint(ctx, mutation)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Ingress.Route == nil || endpoint.Ingress.Route.Events["push"] != "pushes" {
		t.Fatalf("ingress = %+v", endpoint.Ingress)
	}

	loaded, err := store.LoadWebhookEndpointForTenant(ctx, tenant, "github-source")
	if err != nil || loaded.Ingress.Route == nil || loaded.Ingress.Route.Event.Header != "X-Github-Event" {
		t.Fatalf("loaded ingress = %+v, %v", loaded.Ingress, err)
	}
}

func TestWebhookEndpointVerificationUpdate(t *testing.T) {
	ctx := context.Background()
	store, err := Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tenant := filament.TenantID("tenant-one")
	if err := store.EnsureTenant(ctx, tenant, "Tenant One"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateConnection(ctx, filament.Connection{
		ID: "github-source", Tenant: string(tenant), Kind: filament.ConnectorKindSource,
		Name: "source", Connector: "github",
	}); err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: "github-source", SecretRefs: []string{"key"}, Verifier: webhooks.StandardWebhooks,
		Ingress: webhooks.Ingress{Route: &webhooks.Route{
			Event: webhooks.ValueSource{Header: "X-Github-Event"}, Events: map[string]string{"push": "pushes"},
		}},
	}
	mutation.ApplyDefaults()
	endpoint, err := store.PutWebhookEndpoint(ctx, mutation)
	if err != nil {
		t.Fatal(err)
	}

	listed, err := store.ListWebhookEndpointVerifications(ctx)
	if err != nil || len(listed) != 1 || listed[0].Connector != "github" || listed[0].Revision != endpoint.Revision {
		t.Fatalf("listed = %+v, %v", listed, err)
	}
	v := listed[0]
	v.Verifier, v.Ingress = webhooks.GenericHMAC, webhooks.Ingress{}
	if err := store.UpdateWebhookEndpointVerification(ctx, v); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateWebhookEndpointVerification(ctx, v); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale update: %v", err)
	}
	got, err := store.LoadWebhookEndpointForTenant(ctx, tenant, "github-source")
	if err != nil || got.Verifier != webhooks.GenericHMAC || !got.Ingress.IsZero() || got.Revision != endpoint.Revision+1 {
		t.Fatalf("updated = %+v, %v", got, err)
	}
}
