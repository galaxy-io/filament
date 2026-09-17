package notifier

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/galaxy-io/filament/events"
)

// The webhook event shapes. These are the customer-facing contract for each
// exported event kind, kept apart from the internal catalog so the catalog
// can change without breaking receivers. Fields are only ever added.
type (
	// RunStarted reports a worker beginning a run.
	RunStarted struct {
		Type  string    `json:"type"`
		RunID string    `json:"run_id"`
		At    time.Time `json:"at"`
	}

	// RunCompleted reports a run that landed every resource.
	RunCompleted struct {
		Type    string    `json:"type"`
		RunID   string    `json:"run_id"`
		At      time.Time `json:"at"`
		Records int64     `json:"records"`
		Bytes   int64     `json:"bytes"`
	}

	// RunFailed reports a run that ended in failure.
	RunFailed struct {
		Type  string    `json:"type"`
		RunID string    `json:"run_id"`
		At    time.Time `json:"at"`
	}

	// RunPartial reports a run that landed some resources but not all.
	RunPartial struct {
		Type  string    `json:"type"`
		RunID string    `json:"run_id"`
		At    time.Time `json:"at"`
	}

	// RunCanceled reports a run stopped by a cancellation request.
	RunCanceled struct {
		Type  string    `json:"type"`
		RunID string    `json:"run_id"`
		At    time.Time `json:"at"`
	}

	// RunPaused reports a run suspended for a later continuation. Committed
	// reports whether the sink reached a boundary that can promote checkpoints.
	RunPaused struct {
		Type      string    `json:"type"`
		RunID     string    `json:"run_id"`
		At        time.Time `json:"at"`
		Committed bool      `json:"committed"`
	}
)

// Project renders an exported fact as its webhook event. Every exported kind
// must have a case here.
func Project(f events.Fact) (json.RawMessage, error) {
	var body any
	switch d := f.Data.(type) {
	case events.RunStartedEvent:
		body = RunStarted{Type: f.Name, RunID: string(f.Run), At: f.At}
	case events.RunCompletedEvent:
		body = RunCompleted{Type: f.Name, RunID: string(f.Run), At: f.At, Records: d.Records, Bytes: d.Bytes}
	case events.RunFailedEvent:
		body = RunFailed{Type: f.Name, RunID: string(f.Run), At: f.At}
	case events.RunPartialEvent:
		body = RunPartial{Type: f.Name, RunID: string(f.Run), At: f.At}
	case events.RunCanceledEvent:
		body = RunCanceled{Type: f.Name, RunID: string(f.Run), At: f.At}
	case events.RunPausedEvent:
		body = RunPaused{Type: f.Name, RunID: string(f.Run), At: f.At, Committed: d.Committed}
	default:
		return nil, fmt.Errorf("notifier: %s has no webhook payload", f.Name)
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("notifier: encode %s payload: %w", f.Name, err)
	}
	return raw, nil
}
