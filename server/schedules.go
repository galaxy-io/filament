package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// CreatePipelineSchedule attaches the primary schedule to an existing pipeline.
func (a *Server) CreatePipelineSchedule(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineScheduleRequest]) (*connect.Response[ingestionv1.CreatePipelineScheduleResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	pipeline, err := a.schedulePipeline(ctx, tenant, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetSchedule() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("schedule is required"))
	}
	if _, err := a.schedules.LoadPipelineSchedule(ctx, tenant, pipeline.GetId()); err == nil {
		return nil, connect.NewError(connect.CodeAlreadyExists, fmt.Errorf("pipeline already has a schedule"))
	} else if !errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	state, err := newPipelineSchedule(pipeline, req.Msg.GetSchedule())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := a.schedules.SaveSchedule(ctx, state); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a.reconcileScheduledRunsBestEffort(ctx, state)
	return connect.NewResponse(&ingestionv1.CreatePipelineScheduleResponse{
		Schedule: pipelineScheduleToProto(state),
	}), nil
}

// UpdatePipelineSchedule replaces the writable configuration of a schedule.
func (a *Server) UpdatePipelineSchedule(ctx context.Context, req *connect.Request[ingestionv1.UpdatePipelineScheduleRequest]) (*connect.Response[ingestionv1.UpdatePipelineScheduleResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	pipeline, err := a.schedulePipeline(ctx, tenant, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetSchedule() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("schedule is required"))
	}
	current, err := a.loadPipelineSchedule(ctx, tenant, pipeline.GetId())
	if err != nil {
		return nil, err
	}
	next, err := newPipelineSchedule(pipeline, req.Msg.GetSchedule())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	next.ID = current.ID
	next.CreatedAt = current.CreatedAt
	next.LastFired = current.LastFired
	if err := a.schedules.SaveSchedule(ctx, next); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a.reconcileScheduledRunsBestEffort(ctx, next)
	return connect.NewResponse(&ingestionv1.UpdatePipelineScheduleResponse{
		Schedule: pipelineScheduleToProto(next),
	}), nil
}

func (a *Server) schedulePipeline(ctx context.Context, tenant filament.TenantID, pipelineID string) (*ingestionv1.Pipeline, error) {
	if pipelineID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	if a.schedules == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("schedule store is not configured"))
	}
	pipeline, err := a.store.LoadPipeline(ctx, tenant, pipelineID)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if pipeline.GetDeletedAt() != 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("pipeline %q is deleted", pipelineID))
	}
	return pipeline, nil
}

func (a *Server) loadPipelineSchedule(ctx context.Context, tenant filament.TenantID, pipelineID string) (filament.ScheduleState, error) {
	state, err := a.schedules.LoadPipelineSchedule(ctx, tenant, pipelineID)
	if errors.Is(err, filament.ErrNotFound) {
		return filament.ScheduleState{}, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return filament.ScheduleState{}, connect.NewError(connect.CodeInternal, err)
	}
	return state, nil
}
