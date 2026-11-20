package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/webhooks"
)

// An endpoint keeps a copy of the verifier and ingress its source connector
// declared when the endpoint was saved, so the receiver verifies deliveries
// without connector code. A connector update can change either. The server
// brings stored copies up to date at startup and whenever an endpoint is read.

const (
	reconcileFirstRetry = 15 * time.Second
	reconcileMaxRetry   = 5 * time.Minute
)

// StartWebhookReconciler reconciles every webhook endpoint once in the
// background. Workers may start after the server, so a pass that cannot load
// a connector spec is retried with backoff until one completes.
func (a *Server) StartWebhookReconciler(ctx context.Context) {
	if _, ok := a.webhooks.(webhooks.VerificationStore); !ok {
		return
	}
	go func() {
		wait := reconcileFirstRetry
		for {
			err := a.ReconcileWebhookEndpoints(ctx)
			if err == nil {
				return
			}
			a.logWarn("webhook endpoint reconciliation failed; retrying",
				filament.Field{Key: "event.name", Value: "webhook.reconcile.failed"},
				filament.Field{Key: "error", Value: err.Error()},
				filament.Field{Key: "retry_in", Value: wait.String()})
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
			wait = min(wait*2, reconcileMaxRetry)
		}
	}()
}

// ReconcileWebhookEndpoints rewrites every endpoint whose stored verifier or
// ingress differs from its connector's current webhook spec. An endpoint
// saved concurrently is skipped: saving copies the current spec anyway.
func (a *Server) ReconcileWebhookEndpoints(ctx context.Context) error {
	store, ok := a.webhooks.(webhooks.VerificationStore)
	if !ok {
		return nil
	}
	endpoints, err := store.ListWebhookEndpointVerifications(ctx)
	if err != nil {
		return err
	}
	specs := map[string]*filament.WebhookSpec{}
	var errs []error
	for _, endpoint := range endpoints {
		spec, cached := specs[endpoint.Connector]
		if !cached {
			if spec, err = a.connectorWebhookSpec(ctx, endpoint.Connector); err != nil {
				errs = append(errs, err)
				continue
			}
			specs[endpoint.Connector] = spec
		}
		if _, err := a.reconcileWebhookEndpoint(ctx, store, endpoint, spec); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// reconcileWebhookEndpointOnRead brings one endpoint up to date before it is
// returned, and reports whether it changed. A failure is logged, not
// returned: the stored copy still verifies, and startup retries.
func (a *Server) reconcileWebhookEndpointOnRead(ctx context.Context, endpoint webhooks.Endpoint) bool {
	store, ok := a.webhooks.(webhooks.VerificationStore)
	if !ok {
		return false
	}
	conn, err := a.store.LoadConnection(ctx, endpoint.Tenant, endpoint.SourceID)
	if err != nil {
		return false
	}
	spec, err := a.connectorWebhookSpec(ctx, conn.Connector)
	if err == nil {
		var changed bool
		changed, err = a.reconcileWebhookEndpoint(ctx, store, webhooks.EndpointVerification{
			Tenant: endpoint.Tenant, SourceID: endpoint.SourceID, Connector: conn.Connector,
			Revision: endpoint.Revision, Verifier: endpoint.Verifier,
			VerifierParams: endpoint.VerifierParams, Ingress: endpoint.Ingress,
		}, spec)
		if err == nil {
			return changed
		}
	}
	a.logWarn("webhook endpoint reconciliation failed",
		filament.Field{Key: "event.name", Value: "webhook.reconcile.failed"},
		filament.Field{Key: "source_connection_id", Value: endpoint.SourceID},
		filament.Field{Key: "error", Value: err.Error()})
	return false
}

// connectorWebhookSpec is a connector's webhook spec, nil when the connector
// no longer receives webhooks or is no longer registered.
func (a *Server) connectorWebhookSpec(ctx context.Context, connector string) (*filament.WebhookSpec, error) {
	spec, err := a.worker.SourceSpec(ctx, connector)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load source connector %q: %w", connector, err)
	}
	return spec.Webhook, nil
}

func (a *Server) reconcileWebhookEndpoint(ctx context.Context, store webhooks.VerificationStore, endpoint webhooks.EndpointVerification, spec *filament.WebhookSpec) (bool, error) {
	// A connector that stopped receiving webhooks leaves its endpoints as
	// they are. Saving one then fails, which tells the user why.
	if spec == nil {
		return false, nil
	}
	ingress, err := webhooks.ParseIngress(spec.Ingress)
	if err != nil {
		return false, fmt.Errorf("connector %q webhook ingress: %w", endpoint.Connector, err)
	}
	if _, err := webhooks.BuildVerifier(spec.Verifier, spec.VerifierParams); err != nil {
		return false, fmt.Errorf("connector %q webhook verifier: %w", endpoint.Connector, err)
	}
	if endpoint.Matches(spec.Verifier, spec.VerifierParams, ingress) {
		return false, nil
	}
	endpoint.Verifier, endpoint.VerifierParams, endpoint.Ingress = spec.Verifier, spec.VerifierParams, ingress
	err = store.UpdateWebhookEndpointVerification(ctx, endpoint)
	if errors.Is(err, filament.ErrVersionConflict) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if a.receiver.Invalidate != nil {
		a.receiver.Invalidate(endpoint.SourceID)
	}
	a.logInfo("webhook endpoint verification updated from its connector",
		filament.Field{Key: "event.name", Value: "webhook.reconcile.updated"},
		filament.Field{Key: "source_connection_id", Value: endpoint.SourceID},
		filament.Field{Key: "connector", Value: endpoint.Connector})
	return true, nil
}

func (a *Server) logWarn(msg string, fields ...filament.Field) {
	if a.log != nil {
		a.log.Warn(msg, fields...)
	}
}

func (a *Server) logInfo(msg string, fields ...filament.Field) {
	if a.log != nil {
		a.log.Info(msg, fields...)
	}
}
