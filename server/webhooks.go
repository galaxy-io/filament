package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/webhooks"
)

// GetWebhookEndpoint returns a source connection's webhook endpoint.
func (a *Server) GetWebhookEndpoint(ctx context.Context, req *connect.Request[ingestionv1.GetWebhookEndpointRequest]) (*connect.Response[ingestionv1.GetWebhookEndpointResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if a.webhooks == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("webhook endpoints require an endpoint store"))
	}
	if req.Msg.GetSourceConnectionId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source connection ID is required"))
	}
	endpoint, err := a.webhooks.LoadWebhookEndpointForTenant(ctx, tenant, req.Msg.GetSourceConnectionId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.GetWebhookEndpointResponse{Endpoint: a.webhookEndpointProto(endpoint, time.Now())}), nil
}

// GetWebhookDeliveryURL returns the URL a source connection's deliveries go
// to. It needs no endpoint, so a user can register the URL with a provider
// before that provider issues the signing secret the endpoint is saved with.
func (a *Server) GetWebhookDeliveryURL(ctx context.Context, req *connect.Request[ingestionv1.GetWebhookDeliveryURLRequest]) (*connect.Response[ingestionv1.GetWebhookDeliveryURLResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	sourceID := req.Msg.GetSourceConnectionId()
	if sourceID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("source connection ID is required"))
	}
	if _, err := a.webhookSpec(ctx, tenant, sourceID, false); err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.GetWebhookDeliveryURLResponse{DeliveryUrl: a.deliveryURL(sourceID)}), nil
}

// PutWebhookEndpoint creates or updates a source connection's webhook endpoint.
func (a *Server) PutWebhookEndpoint(ctx context.Context, req *connect.Request[ingestionv1.PutWebhookEndpointRequest]) (*connect.Response[ingestionv1.PutWebhookEndpointResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if a.webhooks == nil || a.secrets == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("webhook endpoints require an endpoint store and secrets"))
	}
	in := req.Msg.GetEndpoint()
	expected := req.Msg.GetExpectedRevision()
	if in == nil || in.GetSourceConnectionId() == "" || expected < 0 ||
		(req.Msg.GetMintSecret() && req.Msg.GetImportedSecret() != "") {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("invalid webhook endpoint request"))
	}
	sourceID := in.GetSourceConnectionId()
	spec, err := a.webhookSpec(ctx, tenant, sourceID, in.GetEnabled())
	if err != nil {
		return nil, err
	}
	var previous webhooks.Endpoint
	if expected > 0 {
		previous, err = a.webhooks.LoadWebhookEndpointForTenant(ctx, tenant, sourceID)
		if errors.Is(err, filament.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if previous.Revision != expected {
			return nil, connect.NewError(connect.CodeAborted, filament.ErrVersionConflict)
		}
	}
	ingress, err := webhooks.ParseIngress(spec.Ingress)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("source webhook ingress: %w", err))
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: sourceID, Verifier: spec.Verifier, VerifierParams: spec.VerifierParams, Ingress: ingress,
		PipelineID: in.GetPipelineId(), RouteKey: in.GetRouteKey(), Enabled: in.GetEnabled(),
		SecretRefs: previous.SecretRefs, PreviousKeyExpiresAt: previous.PreviousKeyExpiresAt,
		SuccessStatus: int(in.GetSuccessStatus()), MaxBodyBytes: in.GetMaxBodyBytes(),
		ExpectedRevision: expected, Resources: slices.Clone(in.GetResources()),
		ExpectedPipelineVersionID: req.Msg.GetExpectedPipelineVersionId(),
	}
	mutation.ApplyDefaults()
	if !spec.DynamicResources {
		for _, resource := range mutation.Resources {
			if !slices.Contains(spec.Resources, resource) {
				return nil, connect.NewError(connect.CodeInvalidArgument,
					fmt.Errorf("%q is not a webhook resource of this source", resource))
			}
		}
	}
	if a.receiver.MaxBody > 0 && mutation.MaxBodyBytes > a.receiver.MaxBody {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("max body bytes may not exceed the inbox limit of %d", a.receiver.MaxBody))
	}
	verifier, err := webhooks.BuildVerifier(spec.Verifier, spec.VerifierParams)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("source webhook verifier: %w", err))
	}
	keyFormat, _ := verifier.(webhooks.KeyFormat)
	minted, writtenRef, err := a.writeWebhookKey(ctx, tenant, sourceID, expected, req.Msg, spec, keyFormat, &mutation)
	if err != nil {
		return nil, err
	}
	endpoint, err := a.webhooks.PutWebhookEndpoint(ctx, mutation)
	if err != nil {
		if writtenRef != "" {
			a.deleteSecretRefs(ctx, []string{writtenRef})
		}
		switch {
		case errors.Is(err, filament.ErrVersionConflict):
			return nil, connect.NewError(connect.CodeAborted, err)
		case errors.Is(err, webhooks.ErrRouteNotReady):
			return nil, connect.NewError(connect.CodeFailedPrecondition, err)
		case errors.Is(err, webhooks.ErrInvalidEndpoint):
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if writtenRef != "" && len(previous.SecretRefs) > 1 {
		a.deleteSecretRefs(ctx, previous.SecretRefs[1:])
	}
	if a.receiver.Invalidate != nil {
		a.receiver.Invalidate(sourceID)
	}
	return connect.NewResponse(&ingestionv1.PutWebhookEndpointResponse{
		Endpoint: a.webhookEndpointProto(endpoint, time.Now()), MintedSecret: minted,
	}), nil
}

// webhookSpec returns the webhook spec of an active source connection whose
// connector receives webhooks. Enabling the endpoint also needs webhook
// ingestion turned on for the connection, so a user who turns it off can
// still disable the endpoint.
func (a *Server) webhookSpec(ctx context.Context, tenant filament.TenantID, sourceID string, enabling bool) (*filament.WebhookSpec, error) {
	conn, err := a.store.LoadConnection(ctx, tenant, sourceID)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if conn.Kind != filament.ConnectorKindSource || conn.DeletedAt != 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("connection is not an active source"))
	}
	spec, err := a.worker.SourceSpec(ctx, conn.Connector)
	if errors.Is(err, filament.ErrNotFound) || (err == nil && spec.Webhook == nil) {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("source connector %q does not receive webhooks", conn.Connector))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeUnavailable, fmt.Errorf("load source connector %q: %w", conn.Connector, err))
	}
	if enabling && !spec.Webhook.Enabled(filament.NewConfig(conn.Config)) {
		return nil, connect.NewError(connect.CodeFailedPrecondition,
			fmt.Errorf("this connection reads only the API; set %q to %q or %q first", spec.Webhook.IngestionField, filament.IngestionWebhooks, filament.IngestionBoth))
	}
	return spec.Webhook, nil
}

// writeWebhookKey stores a minted or imported key and makes it the primary
// key. The key it replaces keeps verifying for webhooks.PreviousKeyGrace. A
// verifier with a key format mints and checks keys in its provider's form. A
// secret the provider creates is never minted.
func (a *Server) writeWebhookKey(ctx context.Context, tenant filament.TenantID, sourceID string, expected int64,
	msg *ingestionv1.PutWebhookEndpointRequest, spec *filament.WebhookSpec, format webhooks.KeyFormat, mutation *webhooks.EndpointMutation,
) (minted, ref string, err error) {
	key := msg.GetImportedSecret()
	switch {
	case msg.GetMintSecret() && spec.ProviderIssuesSecret():
		return "", "", connect.NewError(connect.CodeInvalidArgument,
			errors.New("the provider creates this webhook's signing secret; supply it as importedSecret"))
	case msg.GetMintSecret() && format != nil:
		if minted, err = format.MintKey(); err != nil {
			return "", "", connect.NewError(connect.CodeInternal, err)
		}
		key = minted
	case msg.GetMintSecret():
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return "", "", connect.NewError(connect.CodeInternal, err)
		}
		minted = base64.RawURLEncoding.EncodeToString(raw)
		key = minted
	case key != "" && format != nil:
		if err := format.ValidateKey(key); err != nil {
			return "", "", connect.NewError(connect.CodeInvalidArgument, err)
		}
	}
	if key == "" {
		if len(mutation.SecretRefs) == 0 {
			return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("a signing secret is required"))
		}
		return "", "", nil
	}
	if len(key) < 16 || len(key) > 4096 {
		return "", "", connect.NewError(connect.CodeInvalidArgument, errors.New("signing secret length must be between 16 and 4096 bytes"))
	}
	ref = filament.ConnectionSecretRef(string(tenant), sourceID, "webhook_secret", expected+1) + "/" + uuid.NewString()
	if err := a.secrets.Write(ctx, ref, filament.Secret{
		Tenant: tenant, Value: []byte(key),
		Meta: map[string]string{"tenant": string(tenant), "connection": sourceID, "field": "webhook_secret"},
	}); err != nil {
		return "", "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("store webhook signing secret: %w", err))
	}
	refs := []string{ref}
	mutation.PreviousKeyExpiresAt = time.Time{}
	if len(mutation.SecretRefs) > 0 {
		refs = append(refs, mutation.SecretRefs[0])
		mutation.PreviousKeyExpiresAt = time.Now().Add(webhooks.PreviousKeyGrace)
	}
	mutation.SecretRefs = refs
	return minted, ref, nil
}

func (a *Server) webhookEndpointProto(endpoint webhooks.Endpoint, now time.Time) *ingestionv1.WebhookEndpointConfig {
	var previousExpires int64
	if len(endpoint.ActiveKeyRefs(now)) > 1 && !endpoint.PreviousKeyExpiresAt.IsZero() {
		previousExpires = endpoint.PreviousKeyExpiresAt.UnixMilli()
	}
	// A persisted success status is constrained to 200, 202, or 204.
	// #nosec G115
	successStatus := int32(endpoint.SuccessStatus)
	return &ingestionv1.WebhookEndpointConfig{
		SourceConnectionId: endpoint.SourceID,
		PipelineId:         endpoint.PipelineID, RouteKey: endpoint.RouteKey, Resources: slices.Clone(endpoint.Resources),
		Enabled: endpoint.Enabled, Revision: endpoint.Revision,
		SuccessStatus: successStatus, MaxBodyBytes: endpoint.MaxBodyBytes,
		HasSigningSecret: len(endpoint.SecretRefs) > 0, RouteReady: endpoint.RouteReady, Accepting: endpoint.Accepting,
		DeliveryUrl: a.deliveryURL(endpoint.SourceID),
		Verifier:    endpoint.Verifier, PreviousKeyExpiresAt: previousExpires,
	}
}

// deliveryURL is the absolute delivery URL without the resource, or empty
// when no public base URL is configured.
func (a *Server) deliveryURL(sourceID string) string {
	if a.receiver.PublicBaseURL == "" {
		return ""
	}
	return a.receiver.PublicBaseURL + "/hooks/" + sourceID
}

// disableWebhookEndpointWhenOff disables the connection's enabled endpoint
// when cfg turns webhook ingestion off, so deliveries stop with the setting.
// It runs before the connection is saved: if saving then fails, the endpoint
// is only disabled, and a later save enables it again.
func (a *Server) disableWebhookEndpointWhenOff(ctx context.Context, tenant filament.TenantID, sourceID string, spec *filament.WebhookSpec, cfg filament.Config) error {
	if a.webhooks == nil || spec == nil || spec.Enabled(cfg) {
		return nil
	}
	endpoint, err := a.webhooks.LoadWebhookEndpointForTenant(ctx, tenant, sourceID)
	if errors.Is(err, filament.ErrNotFound) {
		return nil
	}
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if !endpoint.Enabled {
		return nil
	}
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: sourceID, Verifier: endpoint.Verifier, VerifierParams: endpoint.VerifierParams,
		Ingress: endpoint.Ingress, PipelineID: endpoint.PipelineID, RouteKey: endpoint.RouteKey,
		Resources: endpoint.Resources, SecretRefs: endpoint.SecretRefs, PreviousKeyExpiresAt: endpoint.PreviousKeyExpiresAt,
		SuccessStatus: endpoint.SuccessStatus, MaxBodyBytes: endpoint.MaxBodyBytes, ExpectedRevision: endpoint.Revision,
	}
	mutation.ApplyDefaults()
	if _, err := a.webhooks.PutWebhookEndpoint(ctx, mutation); err != nil {
		if errors.Is(err, filament.ErrVersionConflict) {
			return connect.NewError(connect.CodeAborted, fmt.Errorf("disable webhook endpoint: %w", err))
		}
		return connect.NewError(connect.CodeInternal, fmt.Errorf("disable webhook endpoint: %w", err))
	}
	if a.receiver.Invalidate != nil {
		a.receiver.Invalidate(sourceID)
	}
	return nil
}
