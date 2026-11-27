package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func (a *Server) runInfo(ctx context.Context, r filament.RunState) (*ingestionv1.RunInfo, error) {
	out := runInfoToProto(r)
	out.ExecutionMode = ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED
	if r.Request.Options.Execution.Normalize() != filament.ExecutionContinuous {
		return out, nil
	}
	out.ExecutionMode = ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS
	store, ok := a.store.(filament.ContinuousRunStore)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, filament.ErrContinuousDisabled)
	}
	req, err := r.StreamStateRequest()
	if err != nil {
		return nil, err
	}
	state, err := store.LoadStreamState(ctx, req)
	if err != nil {
		return nil, err
	}
	if state.Run == r.Run {
		out.ExecutionStatus = executionStatusToProto(state, time.Now())
	} else {
		out.ExecutionStatus = &ingestionv1.ExecutionStatus{DesiredState: ingestionv1.ExecutionDesiredState_EXECUTION_DESIRED_STATE_STOPPED, ObservedState: ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_STOPPED}
	}
	records, nbytes, last, err := store.StreamProgress(ctx, r.Tenant, r.Run)
	if err != nil {
		return nil, err
	}
	out.Records = records
	out.Bytes = nbytes
	if !last.IsZero() {
		out.ExecutionStatus.LastCommittedAt = last.UnixMilli()
	}
	// RunStatus describes the logical run and stays consistent with ListRuns
	// filtering. ExecutionStatus describes worker health; a blocked attempt does
	// not terminate the run or release its admission fence.
	return out, nil
}

func (a *Server) signalContinuous(ctx context.Context, r filament.RunState, req *ingestionv1.SignalRunRequest) (*connect.Response[ingestionv1.SignalRunResponse], error) {
	store, ok := a.store.(filament.ContinuousRunStore)
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, filament.ErrContinuousDisabled)
	}
	key, err := r.StreamStateRequest()
	if err != nil {
		return nil, err
	}
	state, err := store.LoadStreamState(ctx, key)
	if err != nil {
		return nil, err
	}
	if state.Run != r.Run {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("this execution has stopped; start a new run"))
	}
	var desired filament.StreamDesiredState
	switch req.Signal {
	case ingestionv1.RunSignal_RUN_SIGNAL_PAUSE:
		desired = filament.StreamPaused
	case ingestionv1.RunSignal_RUN_SIGNAL_RESUME:
		desired = filament.StreamEnabled
	case ingestionv1.RunSignal_RUN_SIGNAL_STOP:
		desired = filament.StreamStopped
	default:
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("continuous execution supports pause, resume and stop"))
	}
	if req.ExpectedRevision != nil && *req.ExpectedRevision != state.Revision {
		return nil, connect.NewError(connect.CodeAborted, filament.ErrVersionConflict)
	}
	if state.Desired == filament.StreamStopped && desired != filament.StreamStopped {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("stopped execution requires a new start"))
	}
	if desired != state.Desired {
		err = store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: key, ExpectedRevision: state.Revision, Desired: desired})
		if errors.Is(err, filament.ErrVersionConflict) {
			return nil, connect.NewError(connect.CodeAborted, err)
		}
		if err != nil {
			return nil, err
		}
	}
	info, err := a.runInfo(ctx, r)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.SignalRunResponse{ExecutionStatus: info.ExecutionStatus}), nil
}
