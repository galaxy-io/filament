package server

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
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
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	options, err := listOptionsOf(req.Msg.GetPagination(), req.Msg.GetSearch(), req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_NAME:       "name",
		ingestionv1.SortBy_SORT_BY_CREATED_AT: "created_at",
		ingestionv1.SortBy_SORT_BY_UPDATED_AT: "updated_at",
	}, "started_at", true)
	if err != nil {
		return nil, err
	}
	filter := filament.RunFilter{
		Tenant:            tenant,
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
		info, err := a.runInfo(ctx, state)
		if err != nil {
			return nil, err
		}
		runs = append(runs, info)
	}
	return connect.NewResponse(&ingestionv1.ListRunsResponse{
		Runs:       runs,
		Pagination: paginationOf(req.Msg.GetPagination(), options, total),
	}), nil
}

// GetRun returns the run's state and per-resource progress.
func (a *Server) GetRun(ctx context.Context, req *connect.Request[ingestionv1.GetRunRequest]) (*connect.Response[ingestionv1.GetRunResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	state, err := a.store.LoadRun(ctx, tenant, filament.RunID(req.Msg.GetRunId()))
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
	info, err := a.runInfo(ctx, state)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.GetRunResponse{Snapshot: &ingestionv1.RunSnapshot{Run: info, Resources: resources}}), nil
}

// SignalRun validates transport concerns and delegates lifecycle policy to the
// shared run command layer.
func (a *Server) SignalRun(ctx context.Context, req *connect.Request[ingestionv1.SignalRunRequest]) (*connect.Response[ingestionv1.SignalRunResponse], error) {
	if a.store == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("datastore is not configured"))
	}
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetRunId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}
	run := filament.RunID(req.Msg.GetRunId())
	state, err := a.store.LoadRun(ctx, tenant, run)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if state.Request.Options.Execution.Normalize() == filament.ExecutionContinuous {
		return a.signalContinuous(ctx, state, req.Msg)
	}
	if req.Msg.GetSignal() == ingestionv1.RunSignal_RUN_SIGNAL_STOP || req.Msg.ExpectedRevision != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, filament.ErrContinuousDisabled)
	}
	if a.bus == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("event bus is not configured"))
	}
	transitions, ok := a.store.(filament.RunTransitionStore)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("datastore does not support atomic run transitions"))
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
		if signal == filament.SignalCancel && a.isRunStale(state) {
			graceCtx, cancelGrace := context.WithTimeout(ctx, a.cancelGrace())
			ackErr := awaitWorkerSignalFact(graceCtx, ack, signal)
			cancelGrace()
			if ackErr == nil {
				if err := awaitRunStatus(ctx, a.store, tenant, state.Run, filament.RunCanceled); err != nil {
					return nil, signalRunError(err)
				}
				return connect.NewResponse(&ingestionv1.SignalRunResponse{}), nil
			}
			if errors.Is(ackErr, context.DeadlineExceeded) && ctx.Err() == nil {
				// Re-load the run before forcing cancellation: a live worker blocked
				// in a slow sink write still emits background liveness heartbeats or
				// batch flushes that refresh UpdatedAt. Only force-cancel if the run
				// remained stale throughout the grace window.
				fresh, loadErr := a.store.LoadRun(ctx, tenant, state.Run)
				if loadErr != nil {
					return nil, signalRunError(loadErr)
				}
				if fresh.Status == filament.RunCanceled {
					return connect.NewResponse(&ingestionv1.SignalRunResponse{}), nil
				}
				if runStatusTerminal(fresh.Status) {
					return nil, signalRunError(fmt.Errorf("%w: run reached %v before cancel", runcommands.ErrSignalTransition, fresh.Status))
				}
				if !a.isRunStale(fresh) {
					if err := awaitWorkerAcknowledgement(ctx, ack, a.store, tenant, state.Run, signal); err != nil {
						return nil, signalRunError(err)
					}
					return connect.NewResponse(&ingestionv1.SignalRunResponse{}), nil
				}
				if err := a.forceCancelStaleRun(ctx, transitions, tenant, state.Run); err != nil {
					return nil, signalRunError(err)
				}
				return connect.NewResponse(&ingestionv1.SignalRunResponse{}), nil
			}
			return nil, signalRunError(ackErr)
		}
		if err := awaitWorkerAcknowledgement(ctx, ack, a.store, tenant, state.Run, signal); err != nil {
			return nil, signalRunError(err)
		}
	}
	return connect.NewResponse(&ingestionv1.SignalRunResponse{}), nil
}

const (
	defaultStaleRunAfter    = 5 * time.Minute
	defaultStaleCancelGrace = 2 * time.Second
)

func (a *Server) staleThreshold() time.Duration {
	if a.staleRunAfter > 0 {
		return a.staleRunAfter
	}
	if v := os.Getenv("REAPER_STALE_AFTER_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Second
		}
	}
	return defaultStaleRunAfter
}

func (a *Server) cancelGrace() time.Duration {
	if a.staleCancelGrace > 0 {
		return a.staleCancelGrace
	}
	return defaultStaleCancelGrace
}

func (a *Server) isRunStale(state filament.RunState) bool {
	ref := state.UpdatedAt
	if ref.IsZero() {
		ref = state.StartedAt
	}
	if ref.IsZero() {
		return false
	}
	return time.Since(ref) >= a.staleThreshold()
}

// forceCancelStaleRun transitions an unresponsive stale Running run directly
// to RunCanceled and publishes run.canceled so any attached observer unblocks
// and the active replication route is released.
func (a *Server) forceCancelStaleRun(
	ctx context.Context,
	transitions filament.RunTransitionStore,
	tenant filament.TenantID,
	run filament.RunID,
) error {
	_, err := transitions.TransitionRun(
		ctx,
		tenant,
		run,
		[]filament.RunStatus{filament.RunRunning},
		filament.RunCanceled,
		filament.RunTransitionOptions{Ended: true},
	)
	if errors.Is(err, filament.ErrVersionConflict) {
		current, loadErr := a.store.LoadRun(ctx, tenant, run)
		if loadErr != nil {
			return loadErr
		}
		if current.Status != filament.RunCanceled {
			return fmt.Errorf("%w: run reached %v before cancel", runcommands.ErrSignalTransition, current.Status)
		}
		// Already RunCanceled (e.g. retry after a transient publish failure):
		// fall through to repair run.canceled publication.
	} else if err != nil {
		return err
	}
	if pubErr := events.Emit(ctx, a.bus, events.RunCanceled,
		events.Envelope{Tenant: tenant, Run: run, At: time.Now()},
		events.RunCanceledEvent{}); pubErr != nil {
		if a.log != nil {
			a.log.Error("stale run cancel event publish failed", pubErr,
				filament.Field{Key: "event.name", Value: "server.run_canceled.publish_failed"},
				filament.Field{Key: "run_id", Value: string(run)})
		}
		return fmt.Errorf("%w: %v", runcommands.ErrSignalPublish, pubErr)
	}
	return nil
}

func awaitWorkerAcknowledgement(
	ctx context.Context,
	sub eventbus.Subscription,
	store filament.DataStore,
	tenant filament.TenantID,
	run filament.RunID,
	signal filament.Signal,
) error {
	wantStatus := filament.RunPaused
	if signal == filament.SignalCancel {
		wantStatus = filament.RunCanceled
	}
	if err := awaitWorkerSignalFact(ctx, sub, signal); err != nil {
		return err
	}
	return awaitRunStatus(ctx, store, tenant, run, wantStatus)
}

func awaitWorkerSignalFact(
	ctx context.Context,
	sub eventbus.Subscription,
	signal filament.Signal,
) error {
	want := events.RunPaused.Name()
	if signal == filament.SignalCancel {
		want = events.RunCanceled.Name()
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
				return nil
			}
			switch fact.Data.(type) {
			case events.RunCompletedEvent, events.RunFailedEvent, events.RunPartialEvent,
				events.RunPausedEvent, events.RunCanceledEvent:
				return fmt.Errorf("%w: worker finished with %s before %s", runcommands.ErrSignalTransition, fact.Name, want)
			}
		}
	}
}

func awaitRunStatus(ctx context.Context, store filament.DataStore, tenant filament.TenantID, run filament.RunID, want filament.RunStatus) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, tenant, run)
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
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return err
	}
	if a.bus == nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("event bus is not configured"))
	}
	send := stream.Send
	run := filament.RunID(req.Msg.GetRunId())
	if run == "" {
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("run_id is required"))
	}
	sub, err := a.bus.Subscribe(events.RunPattern(tenant, run), eventbus.SubOpts{})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	defer func() { _ = sub.Close() }()

	if req.Msg.GetShouldReplay() {
		terminal, err := a.replayRun(ctx, tenant, run, send)
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if terminal {
			return nil
		}
	}

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			state, ok, err := a.loadRunSnapshot(ctx, tenant, run)
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
			switch f.Data.(type) {
			case events.RunCompletedEvent, events.RunFailedEvent, events.RunPausedEvent, events.RunCanceledEvent, events.RunPartialEvent:
				// The client acts on the terminal event the moment it arrives,
				// so it must not leave before the tracker has persisted it.
				if err := a.awaitTerminalPersisted(ctx, tenant, run); err != nil {
					return err
				}
				return send(tailResponse(eventToProto(f, false)))
			}
			if err := send(tailResponse(eventToProto(f, false))); err != nil {
				return err
			}
		}
	}
}

func (a *Server) awaitTerminalPersisted(ctx context.Context, tenant filament.TenantID, run filament.RunID) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, ok, err := a.loadRunSnapshot(ctx, tenant, run)
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

func (a *Server) replayRun(ctx context.Context, tenant filament.TenantID, run filament.RunID, send func(*ingestionv1.TailRunResponse) error) (bool, error) {
	state, ok, err := a.loadRunSnapshot(ctx, tenant, run)
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}
	for _, resource := range state.Resources {
		if err := send(tailResponse(&ingestionv1.RunEvent{
			EventType: runStatusEventType(resource.Status),
			CreatedAt: snapshotTime(state),
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
			return false, err
		}
	}
	if runStatusTerminal(state.Status) {
		return true, send(tailResponse(runSnapshotEvent(state, true)))
	}
	return false, nil
}

func (a *Server) loadRunSnapshot(ctx context.Context, tenant filament.TenantID, run filament.RunID) (filament.RunState, bool, error) {
	state, err := a.store.LoadRun(ctx, tenant, run)
	if err != nil {
		if errors.Is(err, filament.ErrNotFound) {
			return filament.RunState{}, false, nil
		}
		return filament.RunState{}, false, err
	}
	return state, true, nil
}
