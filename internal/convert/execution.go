package convert

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ExecutionModeToProto encodes an execution mode, preserving an unset default.
func ExecutionModeToProto(mode filament.ExecutionMode) ingestionv1.ExecutionMode {
	switch mode {
	case filament.ExecutionBounded:
		return ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED
	case filament.ExecutionContinuous:
		return ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS
	default:
		return ingestionv1.ExecutionMode_EXECUTION_MODE_UNSPECIFIED
	}
}

// ExecutionModeFromProto validates a wire mode and preserves an unset default.
func ExecutionModeFromProto(mode ingestionv1.ExecutionMode) (filament.ExecutionMode, error) {
	switch mode {
	case ingestionv1.ExecutionMode_EXECUTION_MODE_UNSPECIFIED:
		return "", nil
	case ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED:
		return filament.ExecutionBounded, nil
	case ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS:
		return filament.ExecutionContinuous, nil
	default:
		return "", fmt.Errorf("unknown execution mode %d", mode)
	}
}
