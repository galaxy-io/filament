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
	if mode == ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
		if _, ok := a.store.(filament.ContinuousRunStore); !ok {
			return connect.NewError(connect.CodeFailedPrecondition, filament.ErrContinuousDisabled)
		}
	}
	return nil
}
