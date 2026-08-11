// Package runs holds the run-intake primitive shared by the control-plane
// modules. Both the orchestrator (API/manual intake) and the scheduler (cron/
// timer intake) need to turn a RunRequest into a persisted, dispatched run; this
// is that logic in one place, so neither module references the other.
package runs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
)

// Submit registers a run and dispatches its run.requested trigger, returning the
// assigned id. It is idempotent on the request's IdempotencyKey: a run already
// filed under the derived id is returned as-is, without re-persisting or
// re-dispatching, so a retried caller never starts a duplicate extraction. The
// one exception is a run pre-created by Schedule: it holds no progress, so
// Submit promotes it — overwriting the row with the freshly compiled request —
// and dispatches it like any other run.
func Submit(ctx context.Context, bus eventbus.Bus, ds filament.DataStore, req filament.RunRequest) (filament.RunID, error) {
	if err := req.Tenant.Valid(); err != nil {
		return "", fmt.Errorf("runs: tenant %w", err)
	}
	id := IDFor(req)
	if err := id.Valid(); err != nil {
		return "", fmt.Errorf("runs: run id %w", err)
	}

	now := time.Now()
	if err := ds.CreateRun(ctx, filament.RunState{
		Run:         id,
		Tenant:      req.Tenant,
		Status:      filament.RunRequested,
		Request:     req,
		ScheduleID:  req.ScheduleID,
		ScheduledAt: req.ScheduledFor,
		RequestedAt: now,
	}); err != nil {
		if errors.Is(err, filament.ErrVersionConflict) {
			return id, nil // already submitted — the winning intake dispatched it
		}
		return "", fmt.Errorf("runs: save run %q: %w", id, err)
	}

	env := events.Envelope{Tenant: req.Tenant, Run: id, At: now}
	if err := events.Emit(ctx, bus, events.RunRequested, env, events.RunRequestedEvent{}); err != nil {
		// A row with no trigger would sit Requested forever — reap it and
		// surface the failure so the caller retries the whole submit.
		err = fmt.Errorf("runs: dispatch run %q: %w", id, err)
		if derr := ds.DeleteRun(ctx, id); derr != nil {
			err = errors.Join(err, fmt.Errorf("runs: delete undispatched run %q: %w", id, derr))
		}
		return "", err
	}
	return id, nil
}

// Schedule pre-creates the run for an upcoming schedule occurrence: persisted
// at RunScheduled with no start time and no run.requested emitted, so it shows
// in listings but is invisible to dispatch. Submit with the same idempotency
// key promotes it at fire time. Idempotent: a still-scheduled row is refreshed
// with the latest compiled request; a promoted one is returned untouched.
func Schedule(ctx context.Context, ds filament.DataStore, req filament.RunRequest) (filament.RunID, error) {
	if err := req.Tenant.Valid(); err != nil {
		return "", fmt.Errorf("runs: tenant %w", err)
	}
	id := IDFor(req)
	if err := id.Valid(); err != nil {
		return "", fmt.Errorf("runs: run id %w", err)
	}

	if err := ds.CreateRun(ctx, filament.RunState{
		Run:         id,
		Tenant:      req.Tenant,
		Status:      filament.RunScheduled,
		Request:     req,
		ScheduleID:  req.ScheduleID,
		ScheduledAt: req.ScheduledFor,
	}); err != nil {
		if errors.Is(err, filament.ErrVersionConflict) {
			return id, nil // already promoted past scheduled
		}
		return "", fmt.Errorf("runs: save run %q: %w", id, err)
	}
	return id, nil
}

// IDFor derives a stable id from the request's idempotency key (so retries
// converge on one run) or a random id when no key is given.
func IDFor(req filament.RunRequest) filament.RunID {
	if req.IdempotencyKey != "" {
		sum := sha256.Sum256([]byte(string(req.Tenant) + "|" + req.IdempotencyKey))
		return filament.RunID("run_" + hex.EncodeToString(sum[:8]))
	}
	return filament.RunID(uuid.NewString())
}
