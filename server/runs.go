package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
)

// ListRuns returns runs matching the request's tenant, pipeline, version,
// statuses, and started_at window.
func (a *Server) ListRuns(ctx context.Context, req *connect.Request[ingestionv1.ListRunsRequest]) (*connect.Response[ingestionv1.ListRunsResponse], error) {
	filter := filament.RunFilter{
		Tenant:            filament.TenantID(req.Msg.GetTenantId()),
		PipelineID:        req.Msg.GetPipelineId(),
		PipelineVersionID: req.Msg.PipelineVersionId,
		Status:            runStatusesFromProto(req.Msg.GetStatus()),
	}
	if p := req.Msg.GetPagination(); p != nil {
		offset, err := decodeCursor(p.GetCursor())
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		filter.Limit = int(pageSizeOf(p.GetTotal()))
		filter.Offset = int(offset)
	}
	if req.Msg.GetSinceMs() > 0 {
		filter.Since = time.UnixMilli(req.Msg.GetSinceMs())
	}
	if req.Msg.GetUntilMs() > 0 {
		filter.Until = time.UnixMilli(req.Msg.GetUntilMs())
	}
	states, total, err := a.store.ListRuns(ctx, filter)
	if err != nil {
		return nil, err
	}
	runs := make([]*ingestionv1.RunInfo, 0, len(states))
	for _, state := range states {
		runs = append(runs, runInfoToProto(state))
	}
	pagination := &ingestionv1.PaginationResponse{Total: int32(total)} //nolint:gosec // row counts fit int32
	if req.Msg.GetPagination() != nil {
		pagination = paginationResponse(int32(filter.Offset), int32(filter.Limit), int32(total)) //nolint:gosec // filter and row counts fit int32
	}
	return connect.NewResponse(&ingestionv1.ListRunsResponse{
		Runs:       runs,
		Pagination: pagination,
	}), nil
}

// GetRun returns the run's state and per-resource progress.
func (a *Server) GetRun(ctx context.Context, req *connect.Request[ingestionv1.GetRunRequest]) (*connect.Response[ingestionv1.GetRunResponse], error) {
	state, err := a.store.LoadRun(ctx, filament.RunID(req.Msg.GetRunId()))
	if err != nil {
		return nil, err
	}
	resources := make([]*ingestionv1.RunResourceState, 0, len(state.Resources))
	for _, resource := range state.Resources {
		resources = append(resources, &ingestionv1.RunResourceState{
			ResourceName: resource.Resource,
			Status:       runStatusToProto(resource.Status),
			Records:      resource.Records,
			Bytes:        resource.Bytes,
			Error:        resource.Error,
		})
	}
	return connect.NewResponse(&ingestionv1.GetRunResponse{Snapshot: &ingestionv1.RunSnapshot{Run: runInfoToProto(state), Resources: resources}}), nil
}

// SignalRun rejects the request; run signals are not configured in this binary.
func (a *Server) SignalRun(_ context.Context, req *connect.Request[ingestionv1.SignalRunRequest]) (*connect.Response[ingestionv1.SignalRunResponse], error) {
	if req.Msg.GetRunId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}
	return nil, connect.NewError(connect.CodeUnimplemented, fmt.Errorf("run signals are not configured in this ingestion binary"))
}

// TailRun streams run progress facts to the client, optionally replaying
// events synthesized from the current snapshot before live facts.
func (a *Server) TailRun(ctx context.Context, req *connect.Request[ingestionv1.TailRunRequest], stream *connect.ServerStream[ingestionv1.TailRunResponse]) error {
	if a.bus == nil {
		return fmt.Errorf("event bus is not configured")
	}
	send := stream.Send
	tenant := filament.TenantID(defaultTenant(req.Msg.GetTenantId()))
	run := filament.RunID(req.Msg.GetRunId())
	if run == "" {
		return fmt.Errorf("run_id is required")
	}
	if req.Msg.GetShouldReplay() {
		if err := a.replayRun(ctx, run, send); err != nil {
			return err
		}
		if state, ok, err := a.loadRunSnapshot(ctx, run); err != nil {
			return err
		} else if ok && runStatusTerminal(state.Status) {
			return nil
		}
	}

	sub, err := a.bus.Subscribe(events.RunPattern(tenant, run), eventbus.SubOpts{})
	if err != nil {
		return err
	}
	defer func() { _ = sub.Close() }()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			state, ok, err := a.loadRunSnapshot(ctx, run)
			if err != nil {
				return err
			}
			if ok && runStatusTerminal(state.Status) {
				return send(tailResponse(runSnapshotEvent(state, true)))
			}
		case msg, ok := <-sub.C():
			if !ok {
				return nil
			}
			f, err := events.Decode(msg)
			_ = msg.Ack()
			if err != nil {
				continue
			}
			if err := send(tailResponse(eventToProto(f, false))); err != nil {
				return err
			}
			switch f.Data.(type) {
			case events.RunCompletedEvent, events.RunFailedEvent:
				return nil
			}
		}
	}
}

func (a *Server) replayRun(ctx context.Context, run filament.RunID, send func(*ingestionv1.TailRunResponse) error) error {
	state, ok, err := a.loadRunSnapshot(ctx, run)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for _, resource := range state.Resources {
		if err := send(tailResponse(&ingestionv1.RunEvent{
			EventType: runStatusEventType(resource.Status),
			TenantId:  string(state.Tenant),
			RunId:     string(state.Run),
			Resource:  resource.Resource,
			Fields: &ingestionv1.RunEventFields{
				Records: resource.Records,
				Bytes:   resource.Bytes,
				Error:   resource.Error,
			},
			IsReplay: true,
		})); err != nil {
			return err
		}
	}
	if runStatusTerminal(state.Status) {
		return send(tailResponse(runSnapshotEvent(state, true)))
	}
	return nil
}

func (a *Server) loadRunSnapshot(ctx context.Context, run filament.RunID) (filament.RunState, bool, error) {
	state, err := a.store.LoadRun(ctx, run)
	if err != nil {
		if ctx.Err() != nil {
			return filament.RunState{}, false, ctx.Err()
		}
		return filament.RunState{}, false, nil
	}
	return state, true, nil
}
