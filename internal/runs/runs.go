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

	if existing, err := ds.LoadRun(ctx, id); err == nil {
		if existing.Status != filament.RunScheduled {
			return id, nil // already submitted
		}
	} else if !errors.Is(err, filament.ErrNotFound) {
		return "", fmt.Errorf("runs: load run %q: %w", id, err)
	}

	now := time.Now()
	if err := ds.SaveRun(ctx, filament.RunState{
		Run:         id,
		Tenant:      req.Tenant,
		Status:      filament.RunRequested,
		Request:     req,
		ScheduleID:  req.ScheduleID,
		ScheduledAt: req.ScheduledFor,
		RequestedAt: now,
	}); err != nil {
		return "", fmt.Errorf("runs: save run %q: %w", id, err)
	}

	env := events.Envelope{Tenant: req.Tenant, Run: id, At: now}
	if err := events.Emit(ctx, bus, events.RunRequested, env, events.RunRequestedEvent{}); err != nil {
		return "", fmt.Errorf("runs: dispatch run %q: %w", id, err)
	}
	return id, nil
}

// Schedule pre-creates the run for an upcoming schedule occurrence: persisted
// at RunScheduled with no start time and no run.requested emitted, so it shows
// in listings but is invisible to dispatch. Submit with the same idempotency
// key promotes it at fire time. Idempotent: a run already filed under the
// derived id is returned untouched.
func Schedule(ctx context.Context, ds filament.DataStore, req filament.RunRequest) (filament.RunID, error) {
	if err := req.Tenant.Valid(); err != nil {
		return "", fmt.Errorf("runs: tenant %w", err)
	}
	id := IDFor(req)
	if err := id.Valid(); err != nil {
		return "", fmt.Errorf("runs: run id %w", err)
	}

	if _, err := ds.LoadRun(ctx, id); err == nil {
		return id, nil // already filed
	} else if !errors.Is(err, filament.ErrNotFound) {
		return "", fmt.Errorf("runs: load run %q: %w", id, err)
	}

	if err := ds.SaveRun(ctx, filament.RunState{
		Run:         id,
		Tenant:      req.Tenant,
		Status:      filament.RunScheduled,
		Request:     req,
		ScheduleID:  req.ScheduleID,
		ScheduledAt: req.ScheduledFor,
	}); err != nil {
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
