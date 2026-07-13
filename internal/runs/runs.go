// Package runs holds the run-intake primitive shared by the control-plane
// modules. Both the orchestrator (API/manual intake) and the scheduler (cron/
// timer intake) need to turn a RunRequest into a persisted, dispatched run; this
// is that logic in one place, so neither module references the other.
package runs

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
)

// Submit registers a run and dispatches its run.requested trigger, returning the
// assigned id. It is idempotent on the request's IdempotencyKey: a run already
// filed under the derived id is returned as-is, without re-persisting or
// re-dispatching, so a retried caller never starts a duplicate extraction.
func Submit(ctx context.Context, bus eventbus.Bus, ds ingestion.DataStore, req ingestion.RunRequest) (ingestion.RunID, error) {
	if err := req.Tenant.Valid(); err != nil {
		return "", fmt.Errorf("runs: tenant %w", err)
	}
	id := IDFor(req)
	if err := id.Valid(); err != nil {
		return "", fmt.Errorf("runs: run id %w", err)
	}

	if _, err := ds.LoadRun(ctx, id); err == nil {
		return id, nil // already submitted
	} else if !errors.Is(err, ingestion.ErrNotFound) {
		return "", fmt.Errorf("runs: load run %q: %w", id, err)
	}

	if err := ds.SaveRun(ctx, ingestion.RunState{
		Run:     id,
		Tenant:  req.Tenant,
		Status:  ingestion.RunRequested,
		Request: req,
	}); err != nil {
		return "", fmt.Errorf("runs: save run %q: %w", id, err)
	}

	env := events.Envelope{Tenant: req.Tenant, Run: id, At: time.Now()}
	if err := events.Emit(ctx, bus, events.RunRequested, env, events.RunRequestedEvent{}); err != nil {
		return "", fmt.Errorf("runs: dispatch run %q: %w", id, err)
	}
	return id, nil
}

// IDFor derives a stable id from the request's idempotency key (so retries
// converge on one run) or a random id when no key is given.
func IDFor(req ingestion.RunRequest) ingestion.RunID {
	if req.IdempotencyKey != "" {
		sum := sha256.Sum256([]byte(string(req.Tenant) + "|" + req.IdempotencyKey))
		return ingestion.RunID("run_" + hex.EncodeToString(sum[:8]))
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	return ingestion.RunID("run_" + hex.EncodeToString(b[:]))
}
