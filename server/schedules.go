package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
)

// CreatePipelineSchedule attaches the primary schedule to an existing pipeline.
func (a *Server) CreatePipelineSchedule(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineScheduleRequest]) (*connect.Response[ingestionv1.CreatePipelineScheduleResponse], error) {
	pipeline, err := a.schedulePipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetSchedule() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("schedule is required"))
	}
	if _, err := a.schedules.LoadPipelineSchedule(ctx, pipeline.GetId()); err == nil {
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
	pipeline, err := a.schedulePipeline(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if req.Msg.GetSchedule() == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("schedule is required"))
	}
	current, err := a.loadPipelineSchedule(ctx, pipeline.GetId())
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

// DeletePipelineSchedule removes a pipeline's schedule without deleting the pipeline.
func (a *Server) DeletePipelineSchedule(ctx context.Context, req *connect.Request[ingestionv1.DeletePipelineScheduleRequest]) (*connect.Response[ingestionv1.DeletePipelineScheduleResponse], error) {
	if _, err := a.schedulePipeline(ctx, req.Msg.GetPipelineId()); err != nil {
		return nil, err
	}
	state, err := a.loadPipelineSchedule(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if err := a.schedules.DeleteSchedule(ctx, state.ID); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := a.DropScheduledRuns(ctx, state.ID); err != nil {
		fmt.Printf("[ingestion-api] drop scheduled runs schedule=%s err=%v\n", state.ID, err)
	}
	return connect.NewResponse(&ingestionv1.DeletePipelineScheduleResponse{}), nil
}

// PausePipelineSchedule disables a schedule and clears its next fire time.
func (a *Server) PausePipelineSchedule(ctx context.Context, req *connect.Request[ingestionv1.PausePipelineScheduleRequest]) (*connect.Response[ingestionv1.PausePipelineScheduleResponse], error) {
	if _, err := a.schedulePipeline(ctx, req.Msg.GetPipelineId()); err != nil {
		return nil, err
	}
	state, err := a.loadPipelineSchedule(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	state.Enabled = false
	state.Spec.Enabled = false
	state.NextFire = nil
	if err := a.schedules.SaveSchedule(ctx, state); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a.reconcileScheduledRunsBestEffort(ctx, state)
	return connect.NewResponse(&ingestionv1.PausePipelineScheduleResponse{
		Schedule: pipelineScheduleToProto(state),
	}), nil
}

// ResumePipelineSchedule enables a schedule from its next future occurrence.
func (a *Server) ResumePipelineSchedule(ctx context.Context, req *connect.Request[ingestionv1.ResumePipelineScheduleRequest]) (*connect.Response[ingestionv1.ResumePipelineScheduleResponse], error) {
	if _, err := a.schedulePipeline(ctx, req.Msg.GetPipelineId()); err != nil {
		return nil, err
	}
	state, err := a.loadPipelineSchedule(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, err
	}
	if state.Enabled && state.NextFire != nil {
		return connect.NewResponse(&ingestionv1.ResumePipelineScheduleResponse{
			Schedule: pipelineScheduleToProto(state),
		}), nil
	}
	state.Enabled = true
	state.Spec.Enabled = true
	next, err := scheduledomain.NextFire(state.Spec, time.Now())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	state.NextFire = next
	if err := a.schedules.SaveSchedule(ctx, state); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a.reconcileScheduledRunsBestEffort(ctx, state)
	return connect.NewResponse(&ingestionv1.ResumePipelineScheduleResponse{
		Schedule: pipelineScheduleToProto(state),
	}), nil
}

func (a *Server) schedulePipeline(ctx context.Context, pipelineID string) (*ingestionv1.Pipeline, error) {
	if pipelineID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	if a.schedules == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("schedule store is not configured"))
	}
	pipeline, err := a.store.LoadPipeline(ctx, pipelineID)
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

func (a *Server) loadPipelineSchedule(ctx context.Context, pipelineID string) (filament.ScheduleState, error) {
	state, err := a.schedules.LoadPipelineSchedule(ctx, pipelineID)
	if errors.Is(err, filament.ErrNotFound) {
		return filament.ScheduleState{}, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return filament.ScheduleState{}, connect.NewError(connect.CodeInternal, err)
	}
	return state, nil
}
