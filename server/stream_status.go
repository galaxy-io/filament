package server

import (
	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"time"
)

// executionStatusToProto maps durable runtime state to its public API view.
func executionStatusToProto(s filament.StreamState, now time.Time) *ingestionv1.ExecutionStatus {
	out := &ingestionv1.ExecutionStatus{Revision: s.Revision}
	switch s.Desired {
	case filament.StreamEnabled:
		out.DesiredState = ingestionv1.ExecutionDesiredState_EXECUTION_DESIRED_STATE_ENABLED
	case filament.StreamPaused:
		out.DesiredState = ingestionv1.ExecutionDesiredState_EXECUTION_DESIRED_STATE_PAUSED
	case filament.StreamStopped:
		out.DesiredState = ingestionv1.ExecutionDesiredState_EXECUTION_DESIRED_STATE_STOPPED
	}
	a := s.Attempt
	switch {
	case a != nil && (a.Termination == filament.AttemptUnproven || a.Termination == filament.AttemptReaped || (a.EndedAt == nil && !a.ExpiresAt.After(now) && a.Claimed)):
		out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_BLOCKED
		out.Reason = "The previous worker did not confirm safe shutdown; automatic restart is blocked."
	case a != nil && a.EndedAt == nil && a.Claimed:
		out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_RUNNING
		if s.Desired != filament.StreamEnabled {
			out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_DRAINING
		}
	case s.Desired == filament.StreamPaused:
		out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_PAUSED
	case s.Desired == filament.StreamStopped:
		out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_STOPPED
	case a != nil && a.EndedAt != nil:
		out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_RETRYING
		out.Reason = a.Reason
	default:
		out.ObservedState = ingestionv1.ExecutionObservedState_EXECUTION_OBSERVED_STATE_STARTING
	}
	return out
}
