package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
	"github.com/galaxy-io/filament/internal/streamcontrol"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var _ streamcontrol.Store = (*RuntimeStore)(nil)

// initializeMessageActivation runs inside the same transaction as run admission.
func initializeMessageActivation(ctx context.Context, q *sqlcgen.Queries, r filament.RunState) error {
	ref := r.Request.ReplicationStream
	if ref == nil || r.Request.Source.Connector != "nats" || r.Request.Sink.Connector != "postgres" || len(r.Request.Resources) != 1 {
		return errors.New("stream: unsupported activation profile")
	}
	for _, mode := range r.Request.IngestionTypes {
		if mode != filament.IngestionFullAppend {
			return errors.New("stream: append required")
		}
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
	spec := streamcontrol.Spec(r)
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	if stream.CurrentRunID.Valid {
		if stream.DesiredState != string(filament.StreamStopped) {
			return filament.ErrRunOverlap
		}
		n, err := q.ReplaceStoppedActivation(ctx, sqlcgen.ReplaceStoppedActivationParams{StreamID: ref.ID, TenantID: string(r.Tenant), RunID: pgtype.Text{String: string(r.Run), Valid: true}, RunSpec: data})
		if err != nil {
			return err
		}
		if n != 1 {
			return filament.ErrVersionConflict
		}
	} else {
		if err := q.InitializeStreamExecution(ctx, sqlcgen.InitializeStreamExecutionParams{StreamID: ref.ID, TenantID: string(r.Tenant), RunID: pgtype.Text{String: string(r.Run), Valid: true}, RunSpec: data}); err != nil {
			return err
		}
	}
	return q.ActivateMessageResource(ctx, sqlcgen.ActivateMessageResourceParams{StreamID: ref.ID, TenantID: string(r.Tenant), ResourceName: r.Request.Resources[0]})
}

// ClaimStreamAttempt is a durable, one-time claim. Duplicate dispatches cannot
// run an admitted attempt twice, even after the first worker exits.
func (s *RuntimeStore) ClaimStreamAttempt(ctx context.Context, lease filament.LeaseToken) error {
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

func (s *RuntimeStore) RetireUnclaimedAttempt(ctx context.Context, lease filament.LeaseToken) error {
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
	if err := q.SyncContinuousRunPhase(ctx, sqlcgen.SyncContinuousRunPhaseParams{StreamID: lease.Attempt.StreamID, TenantID: string(lease.Tenant)}); err != nil {
		return err
	}
	if err := q.FinishStoppedContinuousRun(ctx, sqlcgen.FinishStoppedContinuousRunParams{StreamID: lease.Attempt.StreamID, TenantID: string(lease.Tenant)}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// PendingStreamRuns is a bounded cross-tenant scan used only by the supervisor.
func (s *RuntimeStore) PendingStreamRuns(ctx context.Context, after string, limit int) ([]filament.RunState, error) {
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

// StreamProgress is derived from immutable certificates, never tracker facts.
func (s *RuntimeStore) StreamProgress(ctx context.Context, tenant filament.TenantID, run filament.RunID) (int64, int64, time.Time, error) {
	rows, err := s.pool.Query(ctx, `SELECT e.certificate,e.committed_at FROM stream_epochs e JOIN stream_attempts a ON a.token=e.attempt_token WHERE e.tenant_id=$1 AND a.run_spec->>'Run'=$2 ORDER BY e.epoch`, string(tenant), string(run))
	if err != nil {
		return 0, 0, time.Time{}, err
	}
	defer rows.Close()
	var records, nbytes int64
	var last time.Time
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data, &last); err != nil {
			return 0, 0, time.Time{}, err
		}
		var c filament.EpochCertificate
		if err := json.Unmarshal(data, &c); err != nil {
			return 0, 0, time.Time{}, fmt.Errorf("stream: certificate: %w", err)
		}
		records += c.Records
		nbytes += c.Bytes
	}
	return records, nbytes, last, rows.Err()
}
