package server

import (
	"fmt"

	"github.com/galaxy-io/filament"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// validateExecutionMode checks the wire enum. Profile and store
// admission are resolved by graph validation and compilation.
func validateExecutionMode(mode ingestionv1.ExecutionMode) error {
	switch mode {
	case ingestionv1.ExecutionMode_EXECUTION_MODE_UNSPECIFIED, ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED:
		return nil
	case ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS:
		return nil
	default:
		return connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("unknown execution mode %d", mode))
	}
}

func (a *Server) validateExecution(mode ingestionv1.ExecutionMode) error {
	if err := validateExecutionMode(mode); err != nil {
		return err
	}
	if mode == ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS && !a.continuousSupported() {
		return connect.NewError(connect.CodeFailedPrecondition, filament.ErrContinuousDisabled)
	}
	return nil
}

func (a *Server) continuousSupported() bool {
	_, ok := a.store.(filament.ContinuousRunStore)
	return ok
}

// sourceExecutionModes lists the modes a source can run in on this server,
// independent of any sink. A message-stream source without bounded policies is
// continuous only.
func (a *Server) sourceExecutionModes(source filament.Source) []ingestionv1.ExecutionMode {
	spec := source.Spec()
	var modes []ingestionv1.ExecutionMode
	if len(spec.SourcePolicies) > 0 || spec.Stream == nil {
		modes = append(modes, ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED)
	}
	_, planningSupported := source.(filament.ReplicationStreamPlanner)
	_, streamSupported := source.(filament.StreamSource)
	if a.continuousSupported() && planningSupported && streamSupported {
		modes = append(modes, ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS)
	}
	return modes
}

// sinkExecutionModes lists the modes a sink can receive on this server. Write
// policy compatibility with a particular source is decided by edge validation.
func (a *Server) sinkExecutionModes(sink filament.Sink) []ingestionv1.ExecutionMode {
	modes := []ingestionv1.ExecutionMode{ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED}
	_, streamingSupported := sink.(filament.StreamingSink)
	if a.continuousSupported() && streamingSupported && sink.Spec().Capabilities.Stream != nil {
		modes = append(modes, ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS)
	}
	return modes
}
