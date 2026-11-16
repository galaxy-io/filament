package server

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/registry"
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
	"github.com/galaxy-io/filament/webhooks"
	"github.com/galaxy-io/filament/worker"
)

// toggledWebhookSource receives Standard Webhooks only when a connection's
// ingestion field includes webhooks.
type toggledWebhookSource struct{ recordingSource }

func (*toggledWebhookSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name: "toggled-webhooks",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "ingestion", Type: filament.FieldEnum, Default: filament.IngestionAPI, Scope: filament.ScopeConnection, Enum: []filament.EnumOption{
				{Value: filament.IngestionAPI}, {Value: filament.IngestionWebhooks}, {Value: filament.IngestionBoth},
			}},
		}},
		Webhook: &filament.WebhookSpec{
			Verifier: webhooks.StandardWebhooks, IngestionField: "ingestion", Resources: []string{"events_webhook"},
		},
	}
}

// providerSecretSource receives webhooks signed with a secret the provider
// creates, so Filament must never mint one.
type providerSecretSource struct{ toggledWebhookSource }

func (*providerSecretSource) Spec() filament.ConnectorSpec {
	spec := (&toggledWebhookSource{}).Spec()
	spec.Name = "provider-secret"
	spec.Webhook.Verifier = webhooks.GenericHMAC
	spec.Webhook.SecretIssuer = filament.WebhookSecretProvider
	return spec
}

func (*toggledWebhookSource) Configure(context.Context, filament.Config) error      { return nil }
func (*toggledWebhookSource) TestConnection(context.Context, filament.Config) error { return nil }

func newToggledWebhookServer(t *testing.T) *Server {
	t.Helper()
	store := sqlite.NewMemory()
	t.Cleanup(func() { _ = store.Close() })
	secrets, err := secretsqlite.New(store.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	sources := registry.NewSources()
	sources.Register("toggled-webhooks", func() filament.Source { return &toggledWebhookSource{} })
	sources.Register("provider-secret", func() filament.Source { return &providerSecretSource{} })
	return New(worker.Local(sources, registry.NewSinks()), store, nil, nil, WithSecrets(secrets))
}

func toggledIngestion(enabled bool) string {
	if enabled {
		return filament.IngestionBoth
	}
	return filament.IngestionAPI
}

func createToggledConnection(t *testing.T, api *Server, enabled bool) string {
	t.Helper()
	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "posthog", Connector: "toggled-webhooks",
		Config: configStruct(t, map[string]any{"ingestion": toggledIngestion(enabled)}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	return created.Msg.GetConnection().GetId()
}

func TestPutWebhookEndpointHonorsConnectionToggle(t *testing.T) {
	api := newToggledWebhookServer(t)
	off := createToggledConnection(t, api, false)
	_, err := api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint:   &ingestionv1.WebhookEndpointConfig{SourceConnectionId: off, Enabled: true},
		MintSecret: true,
	}))
	if connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Fatalf("enabling with webhooks off: %v", err)
	}
	// Disabled endpoints may still be saved, so a user can turn one off.
	if _, err := api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint:   &ingestionv1.WebhookEndpointConfig{SourceConnectionId: off},
		MintSecret: true,
	})); err != nil {
		t.Fatalf("saving a disabled endpoint: %v", err)
	}
}

func TestPutWebhookEndpointNeverMintsAProviderSecret(t *testing.T) {
	api := newToggledWebhookServer(t)
	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "metronome", Connector: "provider-secret",
		Config: configStruct(t, map[string]any{"ingestion": filament.IngestionWebhooks}),
	}))
	if err != nil {
		t.Fatal(err)
	}
	sourceID := created.Msg.GetConnection().GetId()
	_, err = api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint: &ingestionv1.WebhookEndpointConfig{SourceConnectionId: sourceID}, MintSecret: true,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("minting a provider-issued secret: %v", err)
	}
	put, err := api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint: &ingestionv1.WebhookEndpointConfig{SourceConnectionId: sourceID}, ImportedSecret: "metronome-webhook-secret",
	}))
	if err != nil || put.Msg.GetMintedSecret() != "" || !put.Msg.GetEndpoint().GetHasSigningSecret() {
		t.Fatalf("importing the provider's secret = %+v, %v", put.Msg, err)
	}
	spec, err := api.GetConnector(testCtx(), connect.NewRequest(&ingestionv1.GetConnectorRequest{
		Connector: "provider-secret", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
	}))
	if err != nil || spec.Msg.GetConnector().GetWebhook().GetSecretIssuer() != ingestionv1.WebhookSecretIssuer_WEBHOOK_SECRET_ISSUER_PROVIDER {
		t.Fatalf("connector spec secret issuer = %v, %v", spec.Msg.GetConnector().GetWebhook().GetSecretIssuer(), err)
	}
}

func TestWebhookDeliveryUrlPrecedesTheEndpoint(t *testing.T) {
	api := newToggledWebhookServer(t)
	api.receiver.PublicBaseURL = "https://hooks.example.com"
	sourceID := createToggledConnection(t, api, true)
	got, err := api.GetWebhookDeliveryURL(testCtx(), connect.NewRequest(&ingestionv1.GetWebhookDeliveryURLRequest{SourceConnectionId: sourceID}))
	if err != nil || got.Msg.GetDeliveryUrl() != "https://hooks.example.com/hooks/"+sourceID {
		t.Fatalf("delivery URL before the endpoint exists = %q, %v", got.Msg.GetDeliveryUrl(), err)
	}
	_, err = api.GetWebhookDeliveryURL(testCtx(), connect.NewRequest(&ingestionv1.GetWebhookDeliveryURLRequest{SourceConnectionId: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("delivery URL of a missing source: %v", err)
	}
}

func TestPutWebhookEndpointUsesVerifierKeyFormat(t *testing.T) {
	api := newToggledWebhookServer(t)
	on := createToggledConnection(t, api, true)
	_, err := api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint:       &ingestionv1.WebhookEndpointConfig{SourceConnectionId: on},
		ImportedSecret: "not-a-standard-webhooks-secret",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("malformed Standard Webhooks key: %v", err)
	}
	_, err = api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint:   &ingestionv1.WebhookEndpointConfig{SourceConnectionId: on, Resources: []string{"persons"}},
		MintSecret: true,
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("API resource on the endpoint: %v", err)
	}
	put, err := api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint:   &ingestionv1.WebhookEndpointConfig{SourceConnectionId: on},
		MintSecret: true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(put.Msg.GetMintedSecret(), "whsec_") || put.Msg.GetEndpoint().GetVerifier() != webhooks.StandardWebhooks {
		t.Fatalf("minted %q with verifier %q", put.Msg.GetMintedSecret(), put.Msg.GetEndpoint().GetVerifier())
	}
}

// Turning webhook ingestion off disables the endpoint, so deliveries stop
// with the setting rather than only at the next save.
func TestTurningWebhooksOffDisablesTheEndpoint(t *testing.T) {
	api := newToggledWebhookServer(t)
	store := api.store.(*sqlite.Store)
	ctx := context.Background()
	sourceID := createToggledConnection(t, api, true)
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipeline-one", TenantId: string(filament.DefaultTenantID), Name: "pipeline"}); err != nil {
		t.Fatal(err)
	}
	version, err := store.CreatePipelineVersion(ctx, filament.DefaultTenantID, "pipeline-one", &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatal(err)
	}
	mutation := webhooks.EndpointMutation{
		Tenant: filament.DefaultTenantID, SourceID: sourceID, Verifier: webhooks.StandardWebhooks,
		PipelineID: "pipeline-one", RouteKey: "route/src/snk/continuous", SecretRefs: []string{"key"},
		Resources: []string{"events_webhook"}, Enabled: true, ExpectedPipelineVersionID: version.Id,
	}
	mutation.ApplyDefaults()
	if _, err := store.PutWebhookEndpoint(ctx, mutation); err != nil {
		t.Fatal(err)
	}

	got, err := api.GetConnection(testCtx(), connect.NewRequest(&ingestionv1.GetConnectionRequest{Id: sourceID}))
	if err != nil {
		t.Fatal(err)
	}
	connection := got.Msg.GetConnection()
	connection.Config = configStruct(t, map[string]any{"ingestion": filament.IngestionAPI})
	if _, err := api.UpdateConnection(testCtx(), connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: connection})); err != nil {
		t.Fatal(err)
	}
	endpoint, err := store.LoadWebhookEndpointForTenant(ctx, filament.DefaultTenantID, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	if endpoint.Enabled || endpoint.PipelineID != "pipeline-one" || len(endpoint.Resources) != 1 {
		t.Fatalf("endpoint after turning webhooks off = %+v, want disabled with its route kept", endpoint)
	}
}

// mutableWebhookSource receives webhooks with whatever spec the test sets,
// standing in for a connector whose update changes its verifier or routing.
type mutableWebhookSource struct {
	recordingSource
	spec *filament.WebhookSpec
}

func (s *mutableWebhookSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "mutable-webhooks", Webhook: s.spec}
}

func (*mutableWebhookSource) Configure(context.Context, filament.Config) error      { return nil }
func (*mutableWebhookSource) TestConnection(context.Context, filament.Config) error { return nil }

func TestWebhookEndpointsReconcileWithConnectorSpec(t *testing.T) {
	store := sqlite.NewMemory()
	t.Cleanup(func() { _ = store.Close() })
	secrets, err := secretsqlite.New(store.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	spec := &filament.WebhookSpec{Verifier: webhooks.StandardWebhooks, Resources: []string{"pushes"}}
	sources := registry.NewSources()
	sources.Register("mutable-webhooks", func() filament.Source { return &mutableWebhookSource{spec: spec} })
	api := New(worker.Local(sources, registry.NewSinks()), store, nil, nil, WithSecrets(secrets))

	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "hooks", Connector: "mutable-webhooks",
	}))
	if err != nil {
		t.Fatal(err)
	}
	sourceID := created.Msg.GetConnection().GetId()
	if _, err := api.PutWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.PutWebhookEndpointRequest{
		Endpoint: &ingestionv1.WebhookEndpointConfig{SourceConnectionId: sourceID}, MintSecret: true,
	})); err != nil {
		t.Fatal(err)
	}
	load := func() webhooks.Endpoint {
		t.Helper()
		endpoint, err := store.LoadWebhookEndpointForTenant(context.Background(), filament.DefaultTenantID, sourceID)
		if err != nil {
			t.Fatal(err)
		}
		return endpoint
	}
	saved := load()

	// The connector moves to the HMAC profile and routes by event type.
	spec.Verifier = webhooks.HMAC
	spec.VerifierParams = []byte(`{"signature":{"header":"X-Hub-Signature-256","prefix":"sha256="},"delivery_id":{"header":"X-GitHub-Delivery"}}`)
	spec.Ingress = []byte(`{"route":{"event":{"header":"X-GitHub-Event"},"events":{"push":"pushes"}}}`)
	if err := api.ReconcileWebhookEndpoints(context.Background()); err != nil {
		t.Fatal(err)
	}
	updated := load()
	if updated.Verifier != webhooks.HMAC || updated.Ingress.Route == nil || updated.Ingress.Route.Events["push"] != "pushes" ||
		updated.Revision != saved.Revision+1 {
		t.Fatalf("reconciled endpoint = %+v", updated)
	}
	// Another pass finds nothing to change.
	if err := api.ReconcileWebhookEndpoints(context.Background()); err != nil {
		t.Fatal(err)
	}
	if again := load(); again.Revision != updated.Revision {
		t.Fatalf("second pass rewrote the endpoint: revision %d, want %d", again.Revision, updated.Revision)
	}

	// Reading an endpoint reconciles it too.
	spec.Verifier, spec.VerifierParams, spec.Ingress = webhooks.StandardWebhooks, nil, nil
	got, err := api.GetWebhookEndpoint(testCtx(), connect.NewRequest(&ingestionv1.GetWebhookEndpointRequest{SourceConnectionId: sourceID}))
	if err != nil {
		t.Fatal(err)
	}
	if got.Msg.GetEndpoint().GetVerifier() != webhooks.StandardWebhooks || !load().Ingress.IsZero() {
		t.Fatalf("read endpoint = %+v", got.Msg.GetEndpoint())
	}
}
