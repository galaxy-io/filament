package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// StartAttempt reserves immutable execution without dispatching a Job. A retry
// of an admitted execution ID returns the original attempt without lease renewal.
// Until destination fencing exists, every predecessor must have ended cleanly.
func (s *Store) StartAttempt(ctx context.Context, req filament.StartAttemptRequest) (filament.Attempt, error) {
	if !validTTL(req.TTL) || req.TTL%time.Microsecond != 0 || req.ExecutionID == "" {
		return filament.Attempt{}, errors.New("stream: execution ID and whole-microsecond TTL in [1us,24h] required")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return filament.Attempt{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	stream, err := lockRuntime(ctx, q, filament.StreamStateRequest{Tenant: req.Tenant, Stream: req.Stream})
	if err != nil {
		return filament.Attempt{}, err
	}
	activation, err := q.GetStreamExecution(ctx, sqlcgen.GetStreamExecutionParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if err != nil {
		return filament.Attempt{}, runtimeError(err)
	}
	if req.Run != filament.RunID(activation.RunID) || req.PipelineVersionID != activation.PipelineVersionID || req.Route != stream.RouteKey {
		return filament.Attempt{}, filament.ErrVersionConflict
	}
	existing, err := q.GetStreamAttemptByExecutionID(ctx, sqlcgen.GetStreamAttemptByExecutionIDParams{TenantID: stream.TenantID, ExecutionID: req.ExecutionID})
	if err == nil {
		if existing.StreamID != stream.ID || existing.DesiredRevision != req.ExpectedRevision || existing.RequestTtlUs != req.TTL.Microseconds() {
			return filament.Attempt{}, filament.ErrVersionConflict
		}
		return attemptFromRow(existing, stream)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return filament.Attempt{}, err
	}
	if stream.Status != 0 {
		return filament.Attempt{}, filament.ErrFenced
	}
	if activation.DesiredState != string(filament.StreamEnabled) || activation.Revision != req.ExpectedRevision {
		return filament.Attempt{}, filament.ErrVersionConflict
	}
	previous, err := q.GetLatestRouteAttempt(ctx, sqlcgen.GetLatestRouteAttemptParams{PipelineID: stream.PipelineID, Route: stream.RouteKey, TenantID: stream.TenantID})
	if err == nil && previous.Termination.String != string(filament.AttemptClean) {
		return filament.Attempt{}, filament.ErrTakeoverBlocked
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return filament.Attempt{}, err
	}
	row, err := q.CreateStreamAttempt(ctx, sqlcgen.CreateStreamAttemptParams{StreamID: stream.ID, TenantID: stream.TenantID, ExecutionID: req.ExecutionID, DesiredRevision: req.ExpectedRevision, TtlUs: req.TTL.Microseconds()})
	if err != nil {
		return filament.Attempt{}, err
	}
	attempt, err := attemptFromRow(row, stream)
	if err != nil {
		return filament.Attempt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.Attempt{}, err
	}
	return attempt, nil
}

func currentAttempt(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream, lease filament.LeaseToken, requireLive bool) error {
	if err := lease.Attempt.Validate(); err != nil {
		return err
	}
	row, err := q.GetLatestRouteAttempt(ctx, sqlcgen.GetLatestRouteAttemptParams{PipelineID: stream.PipelineID, Route: stream.RouteKey, TenantID: stream.TenantID})
	if err != nil {
		return runtimeError(err)
	}
	activation, err := q.GetStreamExecution(ctx, sqlcgen.GetStreamExecutionParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if err != nil {
		return runtimeError(err)
	}
	if row.Token != lease.Attempt.Token || row.StreamID != stream.ID || row.ExecutionID != lease.Attempt.ExecutionID || activation.RunID != string(lease.Attempt.RunID) {
		return filament.ErrFenced
	}
	if row.EndedAt.Valid {
		return filament.ErrFenced
	}
	if !requireLive {
		return nil
	}
	now, err := q.Now(ctx)
	if err != nil {
		return err
	}
	if !row.ExpiresAt.Time.After(now.Time) {
		return filament.ErrLeaseExpired
	}
	return nil
}

// RenewLease extends a currently live lease without resurrecting expired owners.
func (s *Store) RenewLease(ctx context.Context, lease filament.LeaseToken, ttl time.Duration) error {
	if !validTTL(ttl) || ttl%time.Microsecond != 0 {
		return errors.New("stream: invalid lease TTL")
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
	if stream.Status != 0 {
		return filament.ErrFenced
	}
	if err := currentAttempt(ctx, q, stream, lease, true); err != nil {
		return err
	}
	rows, err := q.RenewStreamAttempt(ctx, sqlcgen.RenewStreamAttemptParams{TtlUs: ttl.Microseconds(), Token: lease.Attempt.Token, TenantID: stream.TenantID})
	if err != nil {
		return err
	}
	if rows != 1 {
		return filament.ErrLeaseExpired
	}
	return tx.Commit(ctx)
}

// EndAttempt conditionally records termination for the latest route attempt.
// Clean means the owner has already joined its work and proven native closure;
// reaped/unproven declarations never authorize takeover. A retired generation's
// still-current owner may record closure before its lease expires. Reaped and
// unproven observations may also be recorded after expiry; neither grants takeover.
func (s *Store) EndAttempt(ctx context.Context, req filament.EndAttemptRequest) error {
	switch req.Termination {
	case filament.AttemptClean, filament.AttemptReaped, filament.AttemptUnproven:
	default:
		return errors.New("stream: invalid termination")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	stream, err := lockRuntime(ctx, q, requestOf(req.Lease))
	if err != nil {
		return err
	}
	if err := currentAttempt(ctx, q, stream, req.Lease, req.Termination == filament.AttemptClean); err != nil {
		return err
	}
	rows, err := q.EndStreamAttempt(ctx, sqlcgen.EndStreamAttemptParams{Token: req.Lease.Attempt.Token, TenantID: stream.TenantID, Termination: pgtype.Text{String: string(req.Termination), Valid: true}, Reason: req.Reason, RequireLive: req.Termination == filament.AttemptClean})
	if err != nil {
		return err
	}
	if rows != 1 {
		return filament.ErrLeaseExpired
	}
	if err := q.SyncContinuousRunPhase(ctx, sqlcgen.SyncContinuousRunPhaseParams{StreamID: stream.ID, TenantID: stream.TenantID}); err != nil {
		return err
	}
	if err := q.FinishStoppedContinuousRun(ctx, sqlcgen.FinishStoppedContinuousRunParams{StreamID: stream.ID, TenantID: stream.TenantID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
