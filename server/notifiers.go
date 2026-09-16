package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// CreatePipelineNotifier stores secret fields and adds a rule.
func (a *Server) CreatePipelineNotifier(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineNotifierRequest]) (*connect.Response[ingestionv1.CreatePipelineNotifierResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	n, cfg, refs, err := notifierFromInput(req.Msg.GetNotifier(), string(tenant))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := a.validateNotifierConfig(ctx, string(tenant), cfg, refs); err != nil {
		return nil, err
	}
	n.Id, n.TenantId, n.PipelineId = uuid.NewString(), string(tenant), req.Msg.GetPipelineId()
	written, err := a.storeNotifierSecretFields(ctx, string(tenant), n.Id, cfg, refs, false)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	config, err := structpb.NewStruct(cfg)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	n.Config, n.SecretRefs = config, refs
	saved, err := store.CreateNotifier(ctx, n)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, notifierStoreError(err)
	}
	return connect.NewResponse(&ingestionv1.CreatePipelineNotifierResponse{Notifier: saved}), nil
}

// UpdatePipelineNotifier replaces a rule's settings. The last write wins.
func (a *Server) UpdatePipelineNotifier(ctx context.Context, req *connect.Request[ingestionv1.UpdatePipelineNotifierRequest]) (*connect.Response[ingestionv1.UpdatePipelineNotifierResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetNotifierId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notifier_id is required"))
	}
	n, cfg, refs, err := notifierFromInput(req.Msg.GetNotifier(), string(tenant))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	stored, err := store.LoadNotifier(ctx, tenant, req.Msg.GetPipelineId(), req.Msg.GetNotifierId())
	if err != nil {
		return nil, notifierStoreError(err)
	}
	if stored.GetDeletedAt() != 0 {
		return nil, notifierStoreError(fmt.Errorf("notifier is deleted: %w", filament.ErrNotFound))
	}
	if stored.GetNotificationType() != n.GetNotificationType() {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notification type cannot change"))
	}
	if err := a.validateNotifierConfig(ctx, string(tenant), cfg, refs); err != nil {
		return nil, err
	}
	// A newly submitted value gets a fresh ref. The old value stays active
	// until the update succeeds.
	written, err := a.storeNotifierSecretFields(ctx, string(tenant), stored.GetId(), cfg, refs, true)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	config, err := structpb.NewStruct(cfg)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	n.Id, n.TenantId, n.PipelineId = stored.GetId(), string(tenant), stored.GetPipelineId()
	n.Config, n.SecretRefs = config, refs
	saved, err := store.UpdateNotifier(ctx, n)
	if err != nil {
		a.deleteSecretRefs(ctx, written)
		return nil, notifierStoreError(err)
	}
	a.deleteReplacedSecretRefs(ctx, stored.GetSecretRefs(), saved.GetSecretRefs())
	return connect.NewResponse(&ingestionv1.UpdatePipelineNotifierResponse{Notifier: saved}), nil
}

// ListPipelineNotifiers lists live rules, including disabled ones.
func (a *Server) ListPipelineNotifiers(ctx context.Context, req *connect.Request[ingestionv1.ListPipelineNotifiersRequest]) (*connect.Response[ingestionv1.ListPipelineNotifiersResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	rules, err := store.ListNotifiers(ctx, tenant, req.Msg.GetPipelineId(), false)
	if err != nil {
		return nil, notifierStoreError(err)
	}
	return connect.NewResponse(&ingestionv1.ListPipelineNotifiersResponse{Notifiers: rules}), nil
}

// DeletePipelineNotifier soft-deletes a rule, then removes the secrets it owned.
func (a *Server) DeletePipelineNotifier(ctx context.Context, req *connect.Request[ingestionv1.DeletePipelineNotifierRequest]) (*connect.Response[ingestionv1.DeletePipelineNotifierResponse], error) {
	store, tenant, err := a.notifierPipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetNotifierId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("notifier_id is required"))
	}
	n, err := store.DeleteNotifier(ctx, tenant, req.Msg.GetPipelineId(), req.Msg.GetNotifierId())
	if err != nil {
		return nil, notifierStoreError(err)
	}
	a.deleteSecretRefs(ctx, mapValues(n.GetSecretRefs()))
	return connect.NewResponse(&ingestionv1.DeletePipelineNotifierResponse{}), nil
}

func (a *Server) notifierPipeline(ctx context.Context, pipelineID string) (filament.DataStore, filament.TenantID, error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, "", err
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
	return a.store, tenant, nil
}

func notifierStoreError(err error) error {
	if errors.Is(err, filament.ErrNotFound) {
		return connect.NewError(connect.CodeNotFound, fmt.Errorf("pipeline or notifier not found: %w", err))
	}
	return connect.NewError(connect.CodeInternal, fmt.Errorf("notifier storage operation failed: %w", err))
}
