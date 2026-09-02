package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
	runcommands "github.com/galaxy-io/filament/internal/runs"
)

// ListRuns returns runs matching the request's tenant, pipeline, version,
// statuses, and started_at window.
func (a *Server) ListRuns(ctx context.Context, req *connect.Request[ingestionv1.ListRunsRequest]) (*connect.Response[ingestionv1.ListRunsResponse], error) {
	options, err := listOptionsOf(req.Msg.GetPagination(), req.Msg.GetSearch(), req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_NAME:       "name",
		ingestionv1.SortBy_SORT_BY_CREATED_AT: "created_at",
		ingestionv1.SortBy_SORT_BY_UPDATED_AT: "updated_at",
	}, "started_at", true)
	if err != nil {
		return nil, err
	}
	filter := filament.RunFilter{
		Tenant:            filament.TenantID(req.Msg.GetTenantId()),
		PipelineID:        req.Msg.GetPipelineId(),
		PipelineVersionID: req.Msg.PipelineVersionId,
		Status:            runStatusesFromProto(req.Msg.GetStatus()),
		Search:            options.Search,
		SortBy:            options.SortBy,
		SortDescending:    options.SortDescending,
		Limit:             options.Limit,
		Offset:            options.Offset,
	}
	if req.Msg.GetSinceMs() > 0 {
		filter.Since = time.UnixMilli(req.Msg.GetSinceMs())
	}
	if req.Msg.GetUntilMs() > 0 {
		filter.Until = time.UnixMilli(req.Msg.GetUntilMs())
	}
	states, total, err := a.store.ListRuns(ctx, filter)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	runs := make([]*ingestionv1.RunInfo, 0, len(states))
	for _, state := range states {
		runs = append(runs, runInfoToProto(state))
	}
	return connect.NewResponse(&ingestionv1.ListRunsResponse{
		Runs:       runs,
		Pagination: paginationOf(req.Msg.GetPagination(), options, total),
	}), nil
}

// GetRun returns the run's state and per-resource progress.
func (a *Server) GetRun(ctx context.Context, req *connect.Request[ingestionv1.GetRunRequest]) (*connect.Response[ingestionv1.GetRunResponse], error) {
	state, err := a.store.LoadRun(ctx, filament.RunID(req.Msg.GetRunId()))
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
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

// SignalRun validates transport concerns and delegates lifecycle policy to the
// shared run command layer.
func (a *Server) SignalRun(ctx context.Context, req *connect.Request[ingestionv1.SignalRunRequest]) (*connect.Response[ingestionv1.SignalRunResponse], error) {
	if req.Msg.GetRunId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}
	if a.bus == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("event bus is not configured"))
	}
	transitions, ok := a.store.(filament.RunTransitionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("datastore does not support atomic run transitions"))
	}
	run := filament.RunID(req.Msg.GetRunId())
	state, err := a.store.LoadRun(ctx, run)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if tenant := req.Msg.GetTenantId(); tenant != "" && tenant != string(state.Tenant) {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("run %q was not found for tenant", run))
	}
	signal, err := runSignalFromProto(req.Msg.GetSignal())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	var ack eventbus.Subscription
	if state.Status == filament.RunRunning && (signal == filament.SignalPause || signal == filament.SignalCancel) {
		ack, err = a.bus.Subscribe(events.RunPattern(state.Tenant, state.Run), eventbus.SubOpts{})
		if err != nil {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		defer func() { _ = ack.Close() }()
	}
	result, err := runcommands.Signal(ctx, a.bus, transitions, state, signal)
	if err != nil {
		return nil, signalRunError(err)
	}
	if result.WorkerAcknowledgement {
		if ack == nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("worker acknowledgement subscription is missing"))
		}
		if err := awaitWorkerAcknowledgement(ctx, ack, a.store, state.Run, signal); err != nil {
			return nil, signalRunError(err)
		}
	}
	return connect.NewResponse(&ingestionv1.SignalRunResponse{}), nil
}

func awaitWorkerAcknowledgement(
	ctx context.Context,
	sub eventbus.Subscription,
	store filament.DataStore,
	run filament.RunID,
	signal filament.Signal,
) error {
	want := events.RunPaused.Name()
	wantStatus := filament.RunPaused
	if signal == filament.SignalCancel {
		want = events.RunCanceled.Name()
		wantStatus = filament.RunCanceled
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-sub.C():
			if !ok {
				return fmt.Errorf("%w: worker acknowledgement stream closed", runcommands.ErrSignalPublish)
			}
			fact, err := events.Decode(msg)
			_ = msg.Ack()
			if err != nil {
				continue
			}
			if fact.Name == want {
				return awaitRunStatus(ctx, store, run, wantStatus)
			}
			switch fact.Data.(type) {
			case events.RunCompletedEvent, events.RunFailedEvent, events.RunPartialEvent,
				events.RunPausedEvent, events.RunCanceledEvent:
				return fmt.Errorf("%w: worker finished with %s before %s", runcommands.ErrSignalTransition, fact.Name, want)
			}
		}
	}
}

func awaitRunStatus(ctx context.Context, store filament.DataStore, run filament.RunID, want filament.RunStatus) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, run)
		if err != nil {
			return err
		}
		if state.Status == want {
			return nil
		}
		if runStatusTerminal(state.Status) {
			return fmt.Errorf("%w: run reached %v before %v", runcommands.ErrSignalTransition, state.Status, want)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func signalRunError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return connect.NewError(connect.CodeCanceled, err)
	case errors.Is(err, context.DeadlineExceeded):
		return connect.NewError(connect.CodeDeadlineExceeded, err)
	case errors.Is(err, runcommands.ErrSignalTransition),
		errors.Is(err, filament.ErrVersionConflict):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, runcommands.ErrSignalPublish):
		return connect.NewError(connect.CodeUnavailable, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}

// TailRun streams run progress facts to the client, optionally replaying
// events synthesized from the current snapshot before live facts.
func (a *Server) TailRun(ctx context.Context, req *connect.Request[ingestionv1.TailRunRequest], stream *connect.ServerStream[ingestionv1.TailRunResponse]) error {
	if a.bus == nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("event bus is not configured"))
	}
	send := stream.Send
	tenant := filament.TenantID(defaultTenant(req.Msg.GetTenantId()))
	run := filament.RunID(req.Msg.GetRunId())
	if run == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}
	if req.Msg.GetShouldReplay() {
		if err := a.replayRun(ctx, run, send); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if state, ok, err := a.loadRunSnapshot(ctx, run); err != nil {
			return connect.NewError(connect.CodeInternal, err)
		} else if ok && runStatusTerminal(state.Status) {
			// Terminal since replay; emit the terminal event replay skipped.
			return send(tailResponse(runSnapshotEvent(state, true)))
		}
	}

	sub, err := a.bus.Subscribe(events.RunPattern(tenant, run), eventbus.SubOpts{})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
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
				return connect.NewError(connect.CodeInternal, err)
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
			case events.RunCompletedEvent, events.RunFailedEvent, events.RunPausedEvent, events.RunCanceledEvent:
				// Do not close before the tracker persists the terminal status.
				return a.awaitTerminalPersisted(ctx, run)
			}
		}
	}
}

func (a *Server) awaitTerminalPersisted(ctx context.Context, run filament.RunID) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, ok, err := a.loadRunSnapshot(ctx, run)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if ok && runStatusTerminal(state.Status) {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
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
		if errors.Is(err, filament.ErrNotFound) {
			return filament.RunState{}, false, nil
		}
		return filament.RunState{}, false, err
	}
	return state, true, nil
}
