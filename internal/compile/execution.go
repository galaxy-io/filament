package compile

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ExecutionModeFromProto validates stored or requested execution settings.
// Zero remains unset so a run can inherit the pipeline default.
func ExecutionModeFromProto(mode ingestionv1.ExecutionMode) (filament.ExecutionMode, error) {
	switch mode {
	case ingestionv1.ExecutionMode_EXECUTION_MODE_UNSPECIFIED:
		return "", nil
	case ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED:
		return filament.ExecutionBounded, nil
	case ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS:
		return filament.ExecutionContinuous, nil
	default:
		return "", fmt.Errorf("%w: unknown execution mode %d", ErrInvalid, mode)
	}
}

// resolveExecution stamps the inherited mode before any route planning occurs.
func resolveExecution(saved ingestionv1.ExecutionMode, override filament.ExecutionMode) (filament.ExecutionMode, error) {
	mode, err := ExecutionModeFromProto(saved)
	if err != nil {
		return "", err
	}
	if override != "" {
		mode = override
	}
	if err := mode.Validate(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return mode.Normalize(), nil
}
