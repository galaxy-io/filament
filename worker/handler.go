package worker

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	workerv1 "github.com/galaxy-io/filament/api/worker/v1"
	"github.com/galaxy-io/filament/api/worker/v1/workerv1connect"
	"github.com/galaxy-io/filament/internal/convert"
)

// Handler serves a Worker over ConnectRPC. The persistent worker mounts it so
// the control services reach the drivers it carries. It returns the mount
// pattern and the handler, as the generated constructor does.
func Handler(w filament.Worker, opts ...connect.HandlerOption) (string, http.Handler) {
	return workerv1connect.NewWorkerServiceHandler(handler{worker: w}, opts...)
}

type handler struct {
	worker filament.Worker
}

var _ workerv1connect.WorkerServiceHandler = handler{}

// connectError maps a Worker failure onto the Connect code its sentinel
// implies: unknown connector, missing contract, or refused config. Anything
// else is a host failure.
func connectError(err error) error {
	switch {
	case errors.Is(err, filament.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, filament.ErrUnsupported):
		return connect.NewError(connect.CodeUnimplemented, err)
	case errors.Is(err, filament.ErrConfigure):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	var remote *connect.Error
	if errors.As(err, &remote) {
		return remote
	}
	return connect.NewError(connect.CodeInternal, err)
}

func (h handler) Describe(ctx context.Context, _ *connect.Request[workerv1.DescribeRequest]) (*connect.Response[workerv1.DescribeResponse], error) {
	catalog, err := h.worker.Describe(ctx)
	if err != nil {
		return nil, connectError(err)
	}
	resp := &workerv1.DescribeResponse{}
	for _, spec := range catalog.Sources {
		resp.Sources = append(resp.Sources, convert.SourceToProto(spec))
	}
	for _, spec := range catalog.Sinks {
		resp.Sinks = append(resp.Sinks, convert.SinkToProto(spec))
	}
	return connect.NewResponse(resp), nil
}

func (h handler) Validate(ctx context.Context, req *connect.Request[workerv1.ValidateRequest]) (*connect.Response[workerv1.ValidateResponse], error) {
	ref, err := convert.RefFromProto(req.Msg.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.worker.Validate(ctx, ref, filament.NewConfig(convert.StructMap(req.Msg.GetConfig()))); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&workerv1.ValidateResponse{}), nil
}

func (h handler) TestConnection(ctx context.Context, req *connect.Request[workerv1.TestConnectionRequest]) (*connect.Response[workerv1.TestConnectionResponse], error) {
	ref, err := convert.RefFromProto(req.Msg.GetConnector())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := h.worker.TestConnection(ctx, ref, filament.NewConfig(convert.StructMap(req.Msg.GetConfig()))); err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&workerv1.TestConnectionResponse{}), nil
}

func (h handler) Discover(ctx context.Context, req *connect.Request[workerv1.DiscoverRequest]) (*connect.Response[workerv1.DiscoverResponse], error) {
	resources, err := h.worker.Discover(ctx, req.Msg.GetSource(), filament.NewConfig(convert.StructMap(req.Msg.GetConfig())), filament.DiscoverOpts{Refresh: req.Msg.GetRefresh()})
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&workerv1.DiscoverResponse{Resources: convert.ResourcesToProto(resources)}), nil
}

func (h handler) Inspect(ctx context.Context, req *connect.Request[workerv1.InspectRequest]) (*connect.Response[workerv1.InspectResponse], error) {
	inspections, err := h.worker.Inspect(ctx, req.Msg.GetSource(), filament.NewConfig(convert.StructMap(req.Msg.GetConfig())), req.Msg.GetResources())
	if err != nil {
		return nil, connectError(err)
	}
	return connect.NewResponse(&workerv1.InspectResponse{Inspections: convert.InspectionsToProto(inspections)}), nil
}

func (h handler) PlanReplicationStream(ctx context.Context, req *connect.Request[workerv1.PlanReplicationStreamRequest]) (*connect.Response[workerv1.PlanReplicationStreamResponse], error) {
	plan, err := h.worker.PlanReplicationStream(ctx, req.Msg.GetSource(), filament.ReplicationStreamPlanningRequest{
		SourceConnectionID:  req.Msg.GetSourceConnectionId(),
		ReplicationStreamID: req.Msg.GetReplicationStreamId(),
		Config:              filament.NewConfig(convert.StructMap(req.Msg.GetConfig())),
		Resources:           req.Msg.GetResources(),
	})
	if err != nil {
		return nil, connectError(err)
	}
	encoded, err := convert.ReplicationStreamPlanToProto(plan)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&workerv1.PlanReplicationStreamResponse{Plan: encoded}), nil
}
