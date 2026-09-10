package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/notifier"
)

// CreatePipelineNotifier stores the destination as a secret and adds a rule.
func (a *Server) CreatePipelineNotifier(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineNotifierRequest]) (*connect.Response[ingestionv1.CreatePipelineNotifierResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	n, err := notifierFromInput(req.Msg.GetNotifier())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	n.ID, n.Tenant, n.PipelineID = uuid.NewString(), tenant, req.Msg.GetPipelineId()
	ref, written, err := a.notifierDestination(ctx, n, req.Msg.GetNotifier().GetWebhook())
	if err != nil {
		return nil, err
	}
	n.SecretRefs = map[string]string{"destination": ref}
	saved, err := store.CreateNotifier(ctx, n)
	if err != nil {
		if written {
			a.deleteNotifierSecret(ctx, n, ref)
		}
		return nil, notifierStoreError(err)
	}
	return connect.NewResponse(&ingestionv1.CreatePipelineNotifierResponse{Notifier: notifierToProto(saved)}), nil
}

// UpdatePipelineNotifier replaces settings at the version the caller last read.
func (a *Server) UpdatePipelineNotifier(ctx context.Context, req *connect.Request[ingestionv1.UpdatePipelineNotifierRequest]) (*connect.Response[ingestionv1.UpdatePipelineNotifierResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetNotifierId() == "" || req.Msg.GetVersion() < 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notifier_id and a positive version are required"))
	}
	n, err := notifierFromInput(req.Msg.GetNotifier())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	stored, err := store.LoadNotifier(ctx, tenant, req.Msg.GetPipelineId(), req.Msg.GetNotifierId())
	if err != nil {
		return nil, notifierStoreError(err)
	}
	if stored.DeletedAt != 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("notifier is deleted"))
	}
	if stored.Version != req.Msg.GetVersion() {
		return nil, notifierStoreError(filament.ErrVersionConflict)
	}
	if stored.NotificationType != n.NotificationType {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notification type cannot change"))
	}
	ref, written, err := a.notifierDestination(ctx, stored, req.Msg.GetNotifier().GetWebhook())
	if err != nil {
		return nil, err
	}
	n.ID, n.Tenant, n.PipelineID, n.Version = stored.ID, tenant, stored.PipelineID, stored.Version
	n.SecretRefs = map[string]string{"destination": ref}
	saved, err := store.UpdateNotifier(ctx, n)
	if err != nil {
		if written {
			a.deleteNotifierSecret(ctx, n, ref)
		}
		return nil, notifierStoreError(err)
	}
	if old := stored.SecretRefs["destination"]; old != ref {
		a.deleteNotifierSecret(ctx, stored, old)
	}
	return connect.NewResponse(&ingestionv1.UpdatePipelineNotifierResponse{Notifier: notifierToProto(saved)}), nil
}

// ListPipelineNotifiers lists live rules, including disabled ones.
func (a *Server) ListPipelineNotifiers(ctx context.Context, req *connect.Request[ingestionv1.ListPipelineNotifiersRequest]) (*connect.Response[ingestionv1.ListPipelineNotifiersResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	rules, err := store.ListNotifiers(ctx, notifier.Filter{Tenant: tenant, PipelineID: req.Msg.GetPipelineId()})
	if err != nil {
		return nil, notifierStoreError(err)
	}
	out := make([]*ingestionv1.Notifier, 0, len(rules))
	for _, n := range rules {
		out = append(out, notifierToProto(n))
	}
	return connect.NewResponse(&ingestionv1.ListPipelineNotifiersResponse{Notifiers: out}), nil
}

// DeletePipelineNotifier soft-deletes a rule, then removes its owned destination.
func (a *Server) DeletePipelineNotifier(ctx context.Context, req *connect.Request[ingestionv1.DeletePipelineNotifierRequest]) (*connect.Response[ingestionv1.DeletePipelineNotifierResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetNotifierId() == "" || req.Msg.GetVersion() < 1 {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notifier_id and a positive version are required"))
	}
	n, err := store.DeleteNotifier(ctx, tenant, req.Msg.GetPipelineId(), req.Msg.GetNotifierId(), req.Msg.GetVersion())
	if err != nil {
		return nil, notifierStoreError(err)
	}
	a.deleteNotifierSecret(ctx, n, n.SecretRefs["destination"])
	return connect.NewResponse(&ingestionv1.DeletePipelineNotifierResponse{}), nil
}

func (a *Server) notifierPipeline(ctx context.Context, pipelineID string) (notifier.Store, filament.TenantID, error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, "", err
	}
	store, ok := a.store.(notifier.Store)
	if !ok {
		return nil, "", connect.NewError(connect.CodeUnimplemented, fmt.Errorf("datastore does not support notifiers"))
	}
	if pipelineID == "" {
		return nil, "", connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	pipeline, err := a.store.LoadPipeline(ctx, tenant, pipelineID)
	if err != nil {
		return nil, "", notifierStoreError(err)
	}
	if pipeline.GetDeletedAt() != 0 {
		return nil, "", connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("pipeline is deleted"))
	}
	return store, tenant, nil
}

func notifierStoreError(err error) error {
	switch {
	case errors.Is(err, filament.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("pipeline or notifier not found"))
	case errors.Is(err, filament.ErrVersionConflict):
		return connect.NewError(connect.CodeAborted, fmt.Errorf("notifier version conflict; reload before retrying"))
	default:
		return connect.NewError(connect.CodeInternal, fmt.Errorf("notifier storage operation failed"))
	}
}
