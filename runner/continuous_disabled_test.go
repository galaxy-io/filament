package runner

import (
	"context"
	"errors"
	"github.com/galaxy-io/filament"
	"testing"
)

func TestContinuousExecutionDisabledBeforeSideEffects(t *testing.T) {
	err := RunOne(context.Background(), Deps{}, filament.RunSpec{Options: filament.RunOptions{Execution: filament.ExecutionContinuous}})
	if !errors.Is(err, filament.ErrContinuousDisabled) {
		t.Fatalf("continuous execution: %v", err)
	}
}
