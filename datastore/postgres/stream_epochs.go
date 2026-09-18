package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
	"github.com/galaxy-io/filament/rowmodel"
)

var _ filament.StreamRuntimeStore = (*RuntimeStore)(nil)

// CommitEpoch atomically stores immutable certificate bytes and domain progress.
// Identical retries return historical success before checking current authority;
// they never renew a lease. New commits require a bound pipeline completion and
// a live owner, current membership, sequential epoch, and comparable progress.
func (s *RuntimeStore) CommitEpoch(ctx context.Context, request filament.EpochCommit) (filament.CommittedEpoch, error) {
	data, err := request.Certificate.CanonicalBytes(s.codecs)
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	var cert filament.EpochCertificate
	if err := json.Unmarshal(data, &cert); err != nil {
		return filament.CommittedEpoch{}, err
	}
	if len(cert.Coverage.Claims) != 0 {
		return filament.CommittedEpoch{}, filament.ErrIncompleteCoverage
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	lease := filament.LeaseToken{Tenant: cert.Tenant, Attempt: cert.Ref.Attempt}
	stream, err := lockRuntime(ctx, q, requestOf(lease))
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	old, err := q.GetStreamEpoch(ctx, sqlcgen.GetStreamEpochParams{StreamID: stream.ID, TenantID: stream.TenantID, Epoch: cert.Ref.Epoch})
	if err == nil {
		if !bytes.Equal(old.Certificate, data) {
			return filament.CommittedEpoch{}, filament.ErrEpochConflict
		}
		return committedFromRow(old)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return filament.CommittedEpoch{}, err
	}
	if stream.Status != 0 {
		return filament.CommittedEpoch{}, filament.ErrFenced
	}
	if err := request.ValidateCompletion(s.codecs); err != nil {
		return filament.CommittedEpoch{}, err
	}
	activation, err := q.GetStreamExecution(ctx, sqlcgen.GetStreamExecutionParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if err != nil {
		return filament.CommittedEpoch{}, runtimeError(err)
	}
	if activation.PipelineVersionID != cert.PipelineVersionID || activation.LastEpoch == math.MaxInt64 || cert.Ref.Epoch != activation.LastEpoch+1 {
		return filament.CommittedEpoch{}, filament.ErrEpochConflict
	}
	if cert.Ref.MembershipRevision != stream.MembershipRevision {
		return filament.CommittedEpoch{}, filament.ErrMembershipChanged
	}
	if err := validateEpochResources(ctx, q, stream, activation, cert); err != nil {
		return filament.CommittedEpoch{}, err
	}
	snapshot, err := s.validateProgress(ctx, q, stream, activation, cert.Coverage.Positions)
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	if err := currentAttempt(ctx, q, stream, lease, true); err != nil {
		return filament.CommittedEpoch{}, err
	}
	positions, err := json.Marshal(snapshot)
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	row, err := q.CreateStreamEpoch(ctx, sqlcgen.CreateStreamEpochParams{StreamID: stream.ID, TenantID: stream.TenantID, Epoch: cert.Ref.Epoch, AttemptToken: cert.Ref.Attempt.Token, Certificate: data, Positions: positions})
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	n, err := q.AdvanceStreamEpoch(ctx, sqlcgen.AdvanceStreamEpochParams{StreamID: stream.ID, TenantID: stream.TenantID, Epoch: cert.Ref.Epoch, PreviousEpoch: activation.LastEpoch})
	if err != nil {
		return filament.CommittedEpoch{}, err
	}
	if n != 1 {
		return filament.CommittedEpoch{}, filament.ErrEpochConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.CommittedEpoch{}, err
	}
	return committedFromRow(row)
}

func validateEpochResources(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream, activation *sqlcgen.GetStreamExecutionRow, cert filament.EpochCertificate) error {
	rows, err := q.ListReplicationStreamResources(ctx, stream.ID)
	if err != nil {
		return err
	}
	active := make(map[string]bool, len(rows))
	for _, r := range rows {
		active[r.ResourceName] = r.Status == int16(filament.ReplicationStreamResourceActive)
	}
	var spec filament.RunSpec
	if err := json.Unmarshal(activation.RunSpec, &spec); err != nil {
		return err
	}
	for _, resource := range spec.Resources {
		if !active[resource] {
			return filament.ErrMembershipChanged
		}
	}
	for _, r := range cert.Receipts {
		if !active[r.Resource] {
			return filament.ErrMembershipChanged
		}
	}
	return nil
}

// validateProgress checks the new coverage against the last snapshot and returns
// the merged snapshot to store with the new epoch. Domains not covered by this
// epoch keep their previous position.
func (s *RuntimeStore) validateProgress(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream, activation *sqlcgen.GetStreamExecutionRow, positions filament.DomainPositions) (filament.DomainPositions, error) {
	snapshot := make(filament.DomainPositions)
	if activation.LastEpoch > 0 {
		previous, err := latestPositions(ctx, q, stream)
		if err != nil {
			return nil, err
		}
		snapshot = previous
	}
	byDomain := make(map[string]filament.DomainKey, len(snapshot))
	for d := range snapshot {
		byDomain[d.Domain] = d
	}
	seen := make(map[string]bool, len(positions))
	for d, p := range positions {
		if seen[d.Domain] {
			return nil, filament.ErrPositionIncomparable
		}
		seen[d.Domain] = true
		oldKey, ok := byDomain[d.Domain]
		if ok {
			order, err := rowmodel.ComparePositions(s.codecs, oldKey, snapshot[oldKey], d, p)
			if err != nil {
				return nil, err
			}
			switch order {
			case rowmodel.PositionEqual, rowmodel.PositionBefore:
			case rowmodel.PositionAfter:
				return nil, filament.ErrPositionRegression
			default:
				return nil, filament.ErrPositionIncomparable
			}
			delete(snapshot, oldKey)
		}
		snapshot[d] = p
	}
	return snapshot, nil
}

// GetEpoch resolves a historical commit by tenant and generation without checking
// live lease authority. A recovered certificate is not permission to acknowledge.
func (s *RuntimeStore) GetEpoch(ctx context.Context, req filament.EpochLookup) (filament.CommittedEpoch, error) {
	if req.Tenant == "" || req.Key.Epoch <= 0 {
		return filament.CommittedEpoch{}, filament.ErrNotFound
	}
	_, err := s.q.GetReplicationStreamPipelineID(ctx, sqlcgen.GetReplicationStreamPipelineIDParams{StreamID: req.Key.StreamID, TenantID: string(req.Tenant), Generation: req.Key.Generation})
	if err != nil {
		return filament.CommittedEpoch{}, runtimeError(err)
	}
	row, err := s.q.GetStreamEpoch(ctx, sqlcgen.GetStreamEpochParams{StreamID: req.Key.StreamID, TenantID: string(req.Tenant), Epoch: req.Key.Epoch})
	if err != nil {
		return filament.CommittedEpoch{}, runtimeError(err)
	}
	return committedFromRow(row)
}

func committedFromRow(row *sqlcgen.StreamEpoch) (filament.CommittedEpoch, error) {
	var cert filament.EpochCertificate
	if err := json.Unmarshal(row.Certificate, &cert); err != nil {
		return filament.CommittedEpoch{}, err
	}
	return filament.CommittedEpoch{Certificate: cert, CommittedAt: row.CommittedAt.Time}, nil
}
