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
	verifier, err := a.webhookVerifier(ctx, tenant, sourceID)
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
	mutation := webhooks.EndpointMutation{
		Tenant: tenant, SourceID: sourceID, Verifier: verifier,
		PipelineID: in.GetPipelineId(), RouteKey: in.GetRouteKey(), Enabled: in.GetEnabled(),
		SecretRefs: previous.SecretRefs, PreviousKeyExpiresAt: previous.PreviousKeyExpiresAt,
		SuccessStatus: int(in.GetSuccessStatus()), MaxBodyBytes: in.GetMaxBodyBytes(),
		ExpectedRevision: expected, Resources: slices.Clone(in.GetResources()),
		ExpectedPipelineVersionID: req.Msg.GetExpectedPipelineVersionId(),
	}
	mutation.ApplyDefaults()
	if a.receiver.MaxBody > 0 && mutation.MaxBodyBytes > a.receiver.MaxBody {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			fmt.Errorf("max body bytes may not exceed the inbox limit of %d", a.receiver.MaxBody))
	}
	minted, writtenRef, err := a.writeWebhookKey(ctx, tenant, sourceID, expected, req.Msg, &mutation)
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

// webhookVerifier returns the verifier of an active source connection whose
// connector receives webhooks.
func (a *Server) webhookVerifier(ctx context.Context, tenant filament.TenantID, sourceID string) (string, error) {
	conn, err := a.store.LoadConnection(ctx, tenant, sourceID)
	if errors.Is(err, filament.ErrNotFound) {
		return "", connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, err)
	}
	if conn.Kind != filament.ConnectorKindSource || conn.DeletedAt != 0 {
		return "", connect.NewError(connect.CodeInvalidArgument, errors.New("connection is not an active source"))
	}
	verifier, err := a.sourceWebhookVerifier(ctx, conn.Connector)
	if err != nil {
		return "", err
	}
	if verifier == "" {
		return "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("source connector %q does not receive webhooks", conn.Connector))
	}
	return verifier, nil
}

// sourceWebhookVerifier is the connector's webhook verifier, empty when the
// connector does not receive webhooks.
func (a *Server) sourceWebhookVerifier(ctx context.Context, connector string) (string, error) {
	spec, err := a.worker.SourceSpec(ctx, connector)
	if errors.Is(err, filament.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", connect.NewError(connect.CodeUnavailable, fmt.Errorf("load source connector %q: %w", connector, err))
	}
	return spec.WebhookVerifier, nil
}

// writeWebhookKey stores a minted or imported key and makes it the primary
// key. The key it replaces keeps verifying for webhooks.PreviousKeyGrace.
func (a *Server) writeWebhookKey(ctx context.Context, tenant filament.TenantID, sourceID string, expected int64,
	msg *ingestionv1.PutWebhookEndpointRequest, mutation *webhooks.EndpointMutation,
) (minted, ref string, err error) {
	key := msg.GetImportedSecret()
	if msg.GetMintSecret() {
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			return "", "", connect.NewError(connect.CodeInternal, err)
		}
		minted = base64.RawURLEncoding.EncodeToString(raw)
		key = minted
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
