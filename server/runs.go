package server

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func (a *Server) ListRuns(ctx context.Context, req *connect.Request[ingestionv1.ListRunsRequest]) (*connect.Response[ingestionv1.ListRunsResponse], error) {
	states, err := a.store.ListRuns(ctx, ingestion.RunFilter{
		Tenant: ingestion.TenantID(req.Msg.GetTenant()),
		Source: req.Msg.GetSource(),
		Limit:  int(req.Msg.GetLimit()),
	})
	if err != nil {
		return nil, err
	}
	runs := make([]*ingestionv1.RunInfo, 0, len(states))
	for _, state := range states {
		runs = append(runs, runInfoToProto(state))
	}
	return connect.NewResponse(&ingestionv1.ListRunsResponse{Runs: runs}), nil
}

func (a *Server) GetRun(ctx context.Context, req *connect.Request[ingestionv1.GetRunRequest]) (*connect.Response[ingestionv1.GetRunResponse], error) {
	state, err := a.store.LoadRun(ctx, ingestion.RunID(req.Msg.GetRunId()))
	if err != nil {
		return nil, err
	}
	resources := make([]*ingestionv1.RunResourceState, 0, len(state.Resources))
	for _, resource := range state.Resources {
		resources = append(resources, &ingestionv1.RunResourceState{
			Resource: resource.Resource,
			Enabled:  resource.Enabled,
			Status:   runStatusToProto(resource.Status),
			Records:  resource.Records,
			Bytes:    resource.Bytes,
			Error:    resource.Error,
		})
	}
	return connect.NewResponse(&ingestionv1.GetRunResponse{Snapshot: &ingestionv1.RunSnapshot{Run: runInfoToProto(state), Resources: resources}}), nil
}

func (a *Server) SignalRun(_ context.Context, req *connect.Request[ingestionv1.SignalRunRequest]) (*connect.Response[ingestionv1.SignalRunResponse], error) {
	if req.Msg.GetRunId() == "" {
		return nil, fmt.Errorf("run_id is required")
	}
	return nil, fmt.Errorf("run signals are not configured in this ingestion binary")
}

func (a *Server) TailRun(ctx context.Context, req *connect.Request[ingestionv1.TailRunRequest], stream *connect.ServerStream[ingestionv1.TailRunResponse]) error {
	if a.bus == nil {
		return fmt.Errorf("event bus is not configured")
	}
	send := stream.Send
	tenant := ingestion.TenantID(defaultTenant(req.Msg.GetTenant()))
	run := ingestion.RunID(req.Msg.GetRunId())
	if run == "" {
		return fmt.Errorf("run_id is required")
	}
	if req.Msg.GetReplay() {
		if err := a.replayRun(ctx, run, send); err != nil {
			return err
		}
		if state, ok, err := a.loadRunSnapshot(ctx, run); err != nil {
			return err
		} else if ok && runStatusTerminal(state.Status) {
			return nil
		}
	}

	sub, err := a.bus.Subscribe(ingestion.RunPattern(tenant, run), eventbus.SubOpts{})
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
			ev, err := ingestion.EventOf(msg)
			_ = msg.Ack()
			if err != nil {
				continue
			}
			if err := send(tailResponse(eventToProto(ev, false))); err != nil {
				return err
			}
			switch ev.Type {
			case ingestion.EvRunCompleted, ingestion.EvRunFailed:
				return nil
			}
		}
	}
}

func (a *Server) replayRun(ctx context.Context, run ingestion.RunID, send func(*ingestionv1.TailRunResponse) error) error {
	state, ok, err := a.loadRunSnapshot(ctx, run)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	for _, resource := range state.Resources {
		if err := send(tailResponse(&ingestionv1.RunEvent{
			Type:     runStatusEventType(resource.Status),
			Tenant:   string(state.Tenant),
			Run:      string(state.Run),
			Resource: resource.Resource,
			Fields: &ingestionv1.RunEventFields{
				Records: resource.Records,
				Bytes:   resource.Bytes,
				Error:   resource.Error,
			},
			Replay: true,
		})); err != nil {
			return err
		}
	}
	if runStatusTerminal(state.Status) {
		return send(tailResponse(runSnapshotEvent(state, true)))
	}
	return nil
}

func (a *Server) loadRunSnapshot(ctx context.Context, run ingestion.RunID) (ingestion.RunState, bool, error) {
	state, err := a.store.LoadRun(ctx, run)
	if err != nil {
		if ctx.Err() != nil {
			return ingestion.RunState{}, false, ctx.Err()
		}
		return ingestion.RunState{}, false, nil
	}
	return state, true, nil
}
