// Package runs holds the run-intake primitive shared by the control-plane
// modules. Both the orchestrator (API/manual intake) and the scheduler (cron/
// timer intake) need to turn a RunRequest into a persisted, dispatched run; this
// is that logic in one place, so neither module references the other.
package runs

import (
	"context"
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
	state := filament.RunState{
		Run:         id,
		Tenant:      req.Tenant,
		Status:      filament.RunRequested,
		Request:     req,
		ScheduleID:  req.ScheduleID,
		ScheduledAt: req.ScheduledFor,
		RequestedAt: now,
	}
	var createErr error
	if req.ReplicationStream != nil {
		streamStore, ok := ds.(filament.ReplicationStreamRunStore)
		if !ok {
			return "", fmt.Errorf("runs: datastore cannot durably admit replication stream %q", req.ReplicationStream.ID)
		}
		createErr = streamStore.CreateRunWithReplicationStream(ctx, state, *req.ReplicationStream)
	} else {
		createErr = ds.CreateRun(ctx, state)
	}
	if createErr != nil {
		err := createErr
		if errors.Is(err, filament.ErrVersionConflict) {
			state, loadErr := ds.LoadRun(ctx, id)
			if loadErr != nil {
				return "", fmt.Errorf("runs: inspect existing run %q: %w", id, loadErr)
			}
			// Requested with no observed start is also the shape left behind when
			// dispatch and its compensating delete both failed. Publishing the
			// idempotent trigger again repairs that row; progressed runs need no
			// further dispatch.
			if state.Status != filament.RunRequested || !state.StartedAt.IsZero() {
				return id, nil
			}
			now = state.RequestedAt
			if now.IsZero() {
				now = time.Now()
			}
			if err := dispatch(ctx, bus, state.Request.Tenant, id, now); err != nil {
				return "", err
			}
			return id, nil
		}
		return "", fmt.Errorf("runs: save run %q: %w", id, err)
	}

	if err := dispatch(ctx, bus, req.Tenant, id, now); err != nil {
		// A row with no trigger would sit Requested forever — reap it and
		// surface the failure so the caller retries the whole submit.
		if derr := ds.DeleteRun(ctx, id); derr != nil {
			err = errors.Join(err, fmt.Errorf("runs: delete undispatched run %q: %w", id, derr))
		}
		return "", err
	}
	return id, nil
}

func dispatch(ctx context.Context, bus eventbus.Bus, tenant filament.TenantID, id filament.RunID, at time.Time) error {
	env := events.Envelope{Tenant: tenant, Run: id, At: at}
	if err := events.Emit(ctx, bus, events.RunRequested, env, events.RunRequestedEvent{}); err != nil {
		return fmt.Errorf("runs: dispatch run %q: %w", id, err)
	}
	return nil
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
		return filament.RunID(uuid.NewSHA1(uuid.NameSpaceOID, []byte(string(req.Tenant)+"|"+req.IdempotencyKey)).String())
	}
	return filament.RunID(uuid.NewString())
}
