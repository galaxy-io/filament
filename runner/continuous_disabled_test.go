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

func TestExecutionAvailabilityAndSuppliedOwnership(t *testing.T) {
	if err := filament.ExecutionContinuous.Validate(); err != nil {
		t.Fatal("continuous is a valid mode", err)
	}
	valid := filament.RunSpec{Run: "r", ExecutionID: "e", Options: filament.RunOptions{Execution: filament.ExecutionContinuous}, StreamAttempt: &filament.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}}
	if err := RunOne(context.Background(), Deps{}, valid); !errors.Is(err, filament.ErrContinuousDisabled) {
		t.Fatal("valid continuous execution should remain disabled", err)
	}
	valid.ExecutionID = "conflict"
	if err := RunOne(context.Background(), Deps{}, valid); err == nil || errors.Is(err, filament.ErrContinuousDisabled) {
		t.Fatal("supplied ownership conflict not checked", err)
	}
	if err := RunOne(context.Background(), Deps{}, filament.RunSpec{Options: filament.RunOptions{Execution: "unknown"}}); err == nil || errors.Is(err, filament.ErrContinuousDisabled) {
		t.Fatal("unknown mode not distinguished", err)
	}
}
