package compile

import (
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/convert"
)

// ExecutionModeFromProto validates stored or requested execution settings.
// Zero remains unset so a run can inherit the pipeline default.
func ExecutionModeFromProto(mode ingestionv1.ExecutionMode) (filament.ExecutionMode, error) {
	decoded, err := convert.ExecutionModeFromProto(mode)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return decoded, nil
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
