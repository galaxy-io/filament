package server

import (
	"context"
	"time"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
)

type loggingInterceptor struct{ log filament.Logger }

func newLoggingInterceptor(log filament.Logger) loggingInterceptor {
	return loggingInterceptor{log: log.With(filament.Field{Key: "component", Value: "rpc"})}
}

func (i loggingInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		procedure := req.Spec().Procedure
		i.started(procedure)
		started := time.Now()
		response, err := next(ctx, req)
		i.completed(procedure, time.Since(started), err)
		return response, err
	}
}

func (i loggingInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i loggingInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		procedure := conn.Spec().Procedure
		i.started(procedure)
		started := time.Now()
		err := next(ctx, conn)
		i.completed(procedure, time.Since(started), err)
		return err
	}
}

func (i loggingInterceptor) started(procedure string) {
	i.log.Trace("rpc started",
		filament.Field{Key: "event.name", Value: "rpc.started"},
		filament.Field{Key: "rpc.procedure", Value: procedure})
}

func (i loggingInterceptor) completed(procedure string, duration time.Duration, err error) {
	fields := []filament.Field{
		{Key: "event.name", Value: "rpc.completed"},
		{Key: "rpc.procedure", Value: procedure},
		{Key: "duration_ms", Value: duration.Milliseconds()},
	}
	if err == nil {
		fields = append(fields,
			filament.Field{Key: "rpc.code", Value: "ok"},
			filament.Field{Key: "outcome", Value: "success"})
		if mutationProcedure(procedure) {
			i.log.Info("rpc completed", fields...)
			return
		}
		i.log.Debug("rpc completed", fields...)
		return
	}

	code := connect.CodeOf(err)
	fields = append(fields,
		filament.Field{Key: "rpc.code", Value: code.String()},
		filament.Field{Key: "outcome", Value: "failure"})
	switch code {
	case connect.CodeInternal, connect.CodeUnknown, connect.CodeDataLoss, connect.CodeUnavailable:
		i.log.Error("rpc failed", err, fields...)
	case connect.CodeDeadlineExceeded, connect.CodeAborted:
		fields = append(fields, filament.Field{Key: "error", Value: err.Error()})
		i.log.Warn("rpc did not complete", fields...)
	default:
		fields = append(fields, filament.Field{Key: "error", Value: err.Error()})
		i.log.Debug("rpc rejected", fields...)
	}
}

func mutationProcedure(procedure string) bool {
	switch procedure {
	case ingestionv1connect.IngestionServiceCreateConnectionProcedure,
		ingestionv1connect.IngestionServiceUpdateConnectionProcedure,
		ingestionv1connect.IngestionServiceDeleteConnectionProcedure,
		ingestionv1connect.IngestionServiceCreatePipelineProcedure,
		ingestionv1connect.IngestionServiceCreatePipelineVersionProcedure,
		ingestionv1connect.IngestionServiceUpdatePipelineProcedure,
		ingestionv1connect.IngestionServiceDeletePipelineProcedure,
		ingestionv1connect.IngestionServiceCreatePipelineScheduleProcedure,
		ingestionv1connect.IngestionServiceUpdatePipelineScheduleProcedure,
		ingestionv1connect.IngestionServiceRunPipelineProcedure,
		ingestionv1connect.IngestionServiceSignalRunProcedure:
		return true
	default:
		return false
	}
}
