package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

var _ filament.ContinuousRunStore = (*Store)(nil)

// initializeContinuousActivation runs inside the same transaction as run admission.
func initializeContinuousActivation(ctx context.Context, q *sqlcgen.Queries, r filament.RunState) error {
	ref := r.Request.ReplicationStream
	if ref == nil || len(r.Request.Resources) == 0 {
		return errors.New("stream: identity and selected resources required")
	}
	stream, err := q.LockReplicationStreamGeneration(ctx, sqlcgen.LockReplicationStreamGenerationParams{StreamID: ref.ID, TenantID: string(r.Tenant), Generation: ref.Generation})
	if err != nil {
		return err
	}
	previous, err := q.GetLatestRouteAttempt(ctx, sqlcgen.GetLatestRouteAttemptParams{PipelineID: r.Request.PipelineID, Route: r.Request.CheckpointRoute, TenantID: string(r.Tenant)})
	if err == nil && previous.Termination.String != string(filament.AttemptClean) {
		return filament.ErrTakeoverBlocked
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	spec := r.ExecutionSpec()
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	if stream.CurrentRunID.Valid {
		if stream.DesiredState != string(filament.StreamStopped) {
			return filament.ErrRunOverlap
		}
		// A stopped generation resumes under the new spec. Tables it has not
		// streamed yet, and those whose shape changed, start with a baseline.
		if err := reviseStreamMembership(ctx, q, stream, spec); err != nil {
			return err
		}
		n, err := q.ReplaceStoppedActivation(ctx, sqlcgen.ReplaceStoppedActivationParams{StreamID: ref.ID, TenantID: string(r.Tenant), RunID: pgtype.Text{String: string(r.Run), Valid: true}, RunSpec: data})
		if err != nil {
			return err
		}
		if n != 1 {
			return filament.ErrVersionConflict
		}
		return nil
	}
	if err := q.InitializeStreamExecution(ctx, sqlcgen.InitializeStreamExecutionParams{StreamID: ref.ID, TenantID: string(r.Tenant), RunID: pgtype.Text{String: string(r.Run), Valid: true}, RunSpec: data}); err != nil {
		return err
	}
	for _, resource := range r.Request.Resources {
		if err := q.ActivateStreamResource(ctx, sqlcgen.ActivateStreamResourceParams{StreamID: ref.ID, TenantID: string(r.Tenant), ResourceName: resource}); err != nil {
			return err
		}
	}
	return nil
}

// reviseStreamMembership reconciles a generation's members to a new spec. A
// resource the stream has not streamed yet becomes pending, one it no longer
// selects retires, and one the spec asks to resync or whose columns or
// destination changed goes back to pending.
func reviseStreamMembership(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream, spec filament.RunSpec) error {
	var previous filament.RunSpec
	if stream.RunSpec != nil {
		if err := json.Unmarshal(stream.RunSpec, &previous); err != nil {
			return err
		}
	}
	reset := append(append([]string(nil), spec.Resync...), spec.ChangedResources(previous)...)
	_, err := reconcileStreamMembership(ctx, q, stream.ID, filament.TenantID(stream.TenantID), spec.Resources, "snapshot", reset)
	return err
}

// queueStreamSuccessor parks a run submitted against a live generation and
// asks the live run to stop. The successor is the generation's scheduled run;
// it takes over once the live run has drained, through
// promoteStreamSuccessor, which also runs here in case nothing is live.
func queueStreamSuccessor(ctx context.Context, q *sqlcgen.Queries, r filament.RunState) error {
	ref := r.Request.ReplicationStream
	if ref == nil || len(r.Request.Resources) == 0 {
		return errors.New("stream: identity and selected resources required")
	}
	stream, err := q.LockReplicationStreamGeneration(ctx, sqlcgen.LockReplicationStreamGenerationParams{StreamID: ref.ID, TenantID: string(r.Tenant), Generation: ref.Generation})
	if err != nil {
		return err
	}
	if err := cancelStreamSuccessor(ctx, q, stream, string(r.Run)); err != nil {
		return err
	}
	n, err := q.StopStreamForSuccessor(ctx, sqlcgen.StopStreamForSuccessorParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if err != nil {
		return err
	}
	if n != 1 {
		return filament.ErrVersionConflict
	}
	return settleStoppedStream(ctx, q, stream.ID, stream.TenantID)
}

// settleStoppedStream finishes a stopped run once nothing is live on its
// generation, then promotes a waiting successor. Every path that can end a
// run's last attempt or change its intent calls it.
func settleStoppedStream(ctx context.Context, q *sqlcgen.Queries, streamID, tenant string) error {
	if err := q.SyncContinuousRunPhase(ctx, sqlcgen.SyncContinuousRunPhaseParams{StreamID: streamID, TenantID: tenant}); err != nil {
		return err
	}
	if err := q.FinishStoppedContinuousRun(ctx, sqlcgen.FinishStoppedContinuousRunParams{StreamID: streamID, TenantID: tenant}); err != nil {
		return err
	}
	return promoteStreamSuccessor(ctx, q, streamID, tenant)
}

// promoteStreamSuccessor makes a waiting successor the generation's current
// run once the previous run has ended and no attempt is live. The successor's
// spec becomes the activation, its membership revision is applied, and the
// run leaves the scheduled state so the supervisor dispatches it.
func promoteStreamSuccessor(ctx context.Context, q *sqlcgen.Queries, streamID, tenant string) error {
	stream, err := q.GetReplicationStream(ctx, streamID)
	if err != nil {
		return err
	}
	successor, err := q.StreamSuccessor(ctx, sqlcgen.StreamSuccessorParams{TenantID: tenant, StreamID: stream.ID, Generation: strconv.FormatInt(stream.Generation, 10)})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	settled, err := q.StreamSettled(ctx, sqlcgen.StreamSettledParams{StreamID: streamID, TenantID: tenant})
	if err != nil || !settled {
		return err
	}
	var request filament.RunRequest
	if err := json.Unmarshal(successor.Request, &request); err != nil {
		return err
	}
	spec := filament.RunState{Run: filament.RunID(successor.ID), Tenant: filament.TenantID(tenant), Request: request}.ExecutionSpec()
	if err := reviseStreamMembership(ctx, q, stream, spec); err != nil {
		return err
	}
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	n, err := q.PromoteStreamSuccessor(ctx, sqlcgen.PromoteStreamSuccessorParams{StreamID: streamID, TenantID: tenant, RunID: pgtype.Text{String: successor.ID, Valid: true}, RunSpec: data})
	if err != nil {
		return err
	}
	if n != 1 {
		return filament.ErrVersionConflict
	}
	if n, err := q.RequestSuccessorRun(ctx, sqlcgen.RequestSuccessorRunParams{RunID: successor.ID, TenantID: tenant}); err != nil || n != 1 {
		return errors.Join(err, errors.New("stream: successor run is no longer scheduled"))
	}
	return q.SyncContinuousRunPhase(ctx, sqlcgen.SyncContinuousRunPhaseParams{StreamID: streamID, TenantID: tenant})
}

// cancelStreamSuccessor withdraws a waiting successor other than keep, so a
// deliberate stop is not undone by a promotion and a newer successor replaces
// an older one.
func cancelStreamSuccessor(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream, keep string) error {
	successor, err := q.StreamSuccessor(ctx, sqlcgen.StreamSuccessorParams{TenantID: stream.TenantID, StreamID: stream.ID, Generation: strconv.FormatInt(stream.Generation, 10)})
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && successor.ID == keep) {
		return nil
	}
	if err != nil {
		return err
	}
	return q.CancelSuccessorRun(ctx, sqlcgen.CancelSuccessorRunParams{RunID: successor.ID, TenantID: stream.TenantID})
}

// ClaimStreamAttempt is a durable, one-time claim. Duplicate dispatches cannot
// run an admitted attempt twice, even after the first worker exits.
func (s *Store) ClaimStreamAttempt(ctx context.Context, lease filament.LeaseToken) error {
	if err := lease.Attempt.Validate(); err != nil {
		return err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	stream, err := lockRuntime(ctx, q, requestOf(lease))
	if err != nil {
		return err
	}
	if err := currentAttempt(ctx, q, stream, lease, true); err != nil {
		return err
	}
	n, err := q.ClaimStreamAttempt(ctx, sqlcgen.ClaimStreamAttemptParams{Token: lease.Attempt.Token, TenantID: string(lease.Tenant), ExecutionID: lease.Attempt.ExecutionID, RunID: pgtype.Text{String: string(lease.Attempt.RunID), Valid: true}})
	if err != nil {
		return err
	}
	if n != 1 {
		return filament.ErrFenced
	}
	if err := q.MarkContinuousRunStarted(ctx, sqlcgen.MarkContinuousRunStartedParams{RunID: string(lease.Attempt.RunID), TenantID: string(lease.Tenant)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// RetireUnclaimedAttempt ends an unclaimed dispatch after expiry or disabled
// intent, then synchronizes the logical run without retiring a claimed worker.
func (s *Store) RetireUnclaimedAttempt(ctx context.Context, lease filament.LeaseToken) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := lockRuntime(ctx, q, requestOf(lease)); err != nil {
		return err
	}
	if _, err := q.RetireUnclaimedAttempt(ctx, sqlcgen.RetireUnclaimedAttemptParams{Token: lease.Attempt.Token, TenantID: string(lease.Tenant)}); err != nil {
		return err
	}
	if err := settleStoppedStream(ctx, q, lease.Attempt.StreamID, string(lease.Tenant)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PendingStreamRuns is a bounded cross-tenant scan used only by the supervisor.
func (s *Store) PendingStreamRuns(ctx context.Context, after string, limit int) ([]filament.RunState, error) {
	if limit < 1 || limit > 1000 {
		return nil, errors.New("stream: invalid reconciliation page size")
	}
	ids, err := s.q.PendingContinuousRuns(ctx, sqlcgen.PendingContinuousRunsParams{AfterID: after, PageLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]filament.RunState, 0, len(ids))
	for _, id := range ids {
		r, err := s.LoadRun(ctx, filament.TenantID(id.TenantID), filament.RunID(id.ID))
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

// StreamProgress reads totals maintained atomically by the epoch-insert query.
// Idempotent CommitEpoch retries never insert again and cannot double-count.
func (s *Store) StreamProgress(ctx context.Context, tenant filament.TenantID, run filament.RunID) (int64, int64, time.Time, error) {
	var records, nbytes int64
	var last time.Time
	err := s.pool.QueryRow(ctx, `SELECT records,bytes,last_committed_at FROM runs WHERE tenant_id=$1 AND id=$2 AND last_committed_at IS NOT NULL`, string(tenant), string(run)).Scan(&records, &nbytes, &last)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, 0, time.Time{}, nil
	}
	return records, nbytes, last, err
}
