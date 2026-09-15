package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// RuntimeStore adds opt-in continuous persistence to Store. Codecs must be pure,
// deterministic, concurrency-safe, and fixed for the lifetime of this store.
// Constructing it does not enable continuous execution or dispatch workers.
type RuntimeStore struct {
	*Store
	codecs filament.CodecResolver
}

// NewStreamRuntime configures the codecs used to certify and compare progress.
func NewStreamRuntime(store *Store, codecs filament.CodecResolver) *RuntimeStore {
	return &RuntimeStore{Store: store, codecs: codecs}
}

// lockRuntime uses the same pipeline-first order as replication generation
// resolution. The pipeline lock serializes all generations of a route, including
// their admission, renewal, termination, desired-state changes and certification.
func lockRuntime(ctx context.Context, q *sqlcgen.Queries, req filament.StreamStateRequest) (*sqlcgen.ReplicationStream, error) {
	if req.Tenant == "" || req.Stream.ID == "" || req.Stream.Generation <= 0 {
		return nil, filament.ErrNotFound
	}
	id, err := q.GetReplicationStreamPipelineID(ctx, sqlcgen.GetReplicationStreamPipelineIDParams{StreamID: req.Stream.ID, TenantID: string(req.Tenant), Generation: req.Stream.Generation})
	if err != nil {
		return nil, runtimeError(err)
	}
	if _, err := q.LockPipelineForReplicationStream(ctx, sqlcgen.LockPipelineForReplicationStreamParams{PipelineID: id, TenantID: string(req.Tenant)}); err != nil {
		return nil, runtimeError(err)
	}
	row, err := q.LockReplicationStreamGeneration(ctx, sqlcgen.LockReplicationStreamGenerationParams{StreamID: req.Stream.ID, TenantID: string(req.Tenant), Generation: req.Stream.Generation})
	return row, runtimeError(err)
}

func runtimeError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return filament.ErrNotFound
	}
	return err
}

func requestOf(lease filament.LeaseToken) filament.StreamStateRequest {
	return filament.StreamStateRequest{Tenant: lease.Tenant, Stream: filament.StreamRef{ID: lease.Attempt.StreamID, Generation: lease.Attempt.Generation}}
}

// ActivateStream snapshots one run's execution inputs and initializes enabled
// intent. It neither admits a worker nor changes an existing activation's spec.
func (s *RuntimeStore) ActivateStream(ctx context.Context, req filament.StreamActivation) (filament.StreamState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return filament.StreamState{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	stream, err := lockRuntime(ctx, q, req.StreamStateRequest)
	if err != nil {
		return filament.StreamState{}, err
	}
	if stream.Status != 0 {
		return filament.StreamState{}, filament.ErrFenced
	}
	data, err := json.Marshal(req.Spec)
	if err != nil {
		return filament.StreamState{}, err
	}
	current, err := q.GetStreamExecution(ctx, sqlcgen.GetStreamExecutionParams{StreamID: stream.ID, TenantID: stream.TenantID})
	switch {
	case err == nil:
		var spec filament.RunSpec
		if err := json.Unmarshal(current.RunSpec, &spec); err != nil {
			return filament.StreamState{}, err
		}
		existing, err := json.Marshal(spec)
		if err != nil {
			return filament.StreamState{}, err
		}
		if !bytes.Equal(data, existing) {
			return filament.StreamState{}, filament.ErrVersionConflict
		}
	case errors.Is(err, pgx.ErrNoRows):
		data, err = validateActivation(ctx, q, stream, req)
		if err != nil {
			return filament.StreamState{}, err
		}
		err = q.InitializeStreamExecution(ctx, sqlcgen.InitializeStreamExecutionParams{StreamID: stream.ID, TenantID: stream.TenantID, RunID: pgtype.Text{String: string(req.Spec.Run), Valid: true}, RunSpec: data})
		if err != nil {
			return filament.StreamState{}, err
		}
	default:
		return filament.StreamState{}, err
	}
	state, err := loadRuntimeState(ctx, q, stream)
	if err != nil {
		return filament.StreamState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.StreamState{}, err
	}
	return state, nil
}

func validateActivation(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream, req filament.StreamActivation) ([]byte, error) {
	spec := req.Spec
	if spec.Tenant != req.Tenant || spec.PipelineID != stream.PipelineID || spec.CheckpointRoute != stream.RouteKey ||
		spec.ReplicationStream == nil || *spec.ReplicationStream != req.Stream || spec.Run == "" || spec.ExecutionID != "" || spec.StreamAttempt != nil ||
		len(spec.Resources) == 0 || spec.Options.Execution != filament.ExecutionContinuous || spec.SourceConnectionID != stream.SourceConnectionID || spec.SinkConnectionID != stream.SinkConnectionID {
		return nil, errors.New("stream: activation identity mismatch")
	}
	raw, err := q.LoadRunRequest(ctx, sqlcgen.LoadRunRequestParams{RunID: string(spec.Run), TenantID: stream.TenantID, PipelineID: pgtype.Text{String: stream.PipelineID, Valid: true}, PipelineVersionID: pgtype.Text{String: spec.PipelineVersionID, Valid: true}})
	if err != nil {
		return nil, runtimeError(err)
	}
	var run filament.RunRequest
	if err := json.Unmarshal(raw, &run); err != nil {
		return nil, err
	}
	if run.Tenant != spec.Tenant || run.ReplicationStream == nil || *run.ReplicationStream != req.Stream || run.CheckpointRoute != spec.CheckpointRoute || run.Options.Execution != filament.ExecutionContinuous {
		return nil, errors.New("stream: activation does not match admitted run")
	}
	return json.Marshal(spec)
}

// LoadStreamState reads a coherent snapshot under the route and membership locks.
func (s *RuntimeStore) LoadStreamState(ctx context.Context, req filament.StreamStateRequest) (filament.StreamState, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return filament.StreamState{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	stream, err := lockRuntime(ctx, q, req)
	if err != nil {
		return filament.StreamState{}, err
	}
	state, err := loadRuntimeState(ctx, q, stream)
	if err != nil {
		return filament.StreamState{}, err
	}
	return state, tx.Commit(ctx)
}

func loadRuntimeState(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream) (filament.StreamState, error) {
	a, err := q.GetStreamExecution(ctx, sqlcgen.GetStreamExecutionParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if err != nil {
		return filament.StreamState{}, runtimeError(err)
	}
	state := filament.StreamState{StreamStateRequest: filament.StreamStateRequest{Tenant: filament.TenantID(stream.TenantID), Stream: filament.StreamRef{ID: stream.ID, Generation: stream.Generation}}, Desired: filament.StreamDesiredState(a.DesiredState), Revision: a.Revision, PipelineVersionID: a.PipelineVersionID, Route: stream.RouteKey, Run: filament.RunID(a.RunID), MembershipRevision: stream.MembershipRevision, Membership: make(map[string]filament.ReplicationStreamResourceStatus), CommittedPositions: make(filament.DomainPositions)}
	if a.LastEpoch > 0 {
		state.LastEpoch = &filament.EpochKey{StreamID: stream.ID, Generation: stream.Generation, Epoch: a.LastEpoch}
	}
	attempt, err := q.GetLatestStreamAttempt(ctx, sqlcgen.GetLatestStreamAttemptParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if err == nil {
		value, err := attemptFromRow(attempt, stream)
		if err != nil {
			return filament.StreamState{}, err
		}
		state.Attempt = &value
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return filament.StreamState{}, err
	}
	membership, err := q.ListReplicationStreamResources(ctx, stream.ID)
	if err != nil {
		return filament.StreamState{}, err
	}
	for _, m := range membership {
		state.Membership[m.ResourceName] = filament.ReplicationStreamResourceStatus(m.Status)
	}
	if a.LastEpoch > 0 {
		positions, err := latestPositions(ctx, q, stream)
		if err != nil {
			return filament.StreamState{}, err
		}
		state.CommittedPositions = positions
	}
	return state, nil
}

// SetDesiredState persists intent under optimistic revision control. Existing
// owners may drain after pause/stop, but no new attempt may start until enabled.
func (s *RuntimeStore) SetDesiredState(ctx context.Context, change filament.DesiredStateChange) error {
	switch change.Desired {
	case filament.StreamEnabled, filament.StreamPaused, filament.StreamStopped:
	default:
		return errors.New("stream: invalid desired state")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	stream, err := lockRuntime(ctx, q, change.StreamStateRequest)
	if err != nil {
		return err
	}
	if stream.Status != 0 {
		return filament.ErrFenced
	}
	rows, err := q.ChangeStreamDesiredState(ctx, sqlcgen.ChangeStreamDesiredStateParams{DesiredState: string(change.Desired), StreamID: stream.ID, TenantID: stream.TenantID, Revision: change.ExpectedRevision})
	if err != nil {
		return err
	}
	if rows != 1 {
		return filament.ErrVersionConflict
	}
	return tx.Commit(ctx)
}

// ListReconcileCandidates returns a bounded tenant-scoped keyset page. Each state
// is coherent; the page is not a transaction spanning all routes. Include paused
// and stopped streams so a supervisor can reconcile draining owners.
func (s *RuntimeStore) ListReconcileCandidates(ctx context.Context, req filament.ReconcileQuery) (filament.ReconcilePage, error) {
	if req.Tenant == "" || req.Limit < 1 || req.Limit > 1000 {
		return filament.ReconcilePage{}, errors.New("stream: tenant and page limit 1..1000 required")
	}
	ids, err := s.q.ListReconcilableStreamIDs(ctx, sqlcgen.ListReconcilableStreamIDsParams{TenantID: string(req.Tenant), AfterID: req.After, PageLimit: int32(req.Limit + 1)})
	if err != nil {
		return filament.ReconcilePage{}, err
	}
	page := filament.ReconcilePage{}
	if len(ids) > req.Limit {
		ids = ids[:req.Limit]
		page.Next = ids[len(ids)-1]
	}
	for _, id := range ids {
		stream, err := s.q.GetReplicationStream(ctx, id)
		if err != nil {
			return filament.ReconcilePage{}, err
		}
		state, err := s.LoadStreamState(ctx, filament.StreamStateRequest{Tenant: req.Tenant, Stream: filament.StreamRef{ID: id, Generation: stream.Generation}})
		if err != nil {
			return filament.ReconcilePage{}, err
		}
		page.States = append(page.States, state)
	}
	return page, nil
}

func validTTL(ttl time.Duration) bool { return ttl >= time.Microsecond && ttl <= 24*time.Hour }

// attemptFromRow derives the attempt's immutable spec from the stream activation;
// ActivateStream never changes an existing spec, so every attempt shares it.
func attemptFromRow(row *sqlcgen.StreamAttempt, stream *sqlcgen.ReplicationStream) (filament.Attempt, error) {
	var spec filament.RunSpec
	if err := json.Unmarshal(stream.RunSpec, &spec); err != nil {
		return filament.Attempt{}, fmt.Errorf("stream: attempt spec: %w", err)
	}
	ref := filament.AttemptRef{RunID: spec.Run, ExecutionID: row.ExecutionID, StreamID: stream.ID, Generation: stream.Generation, Token: row.Token}
	spec.StreamAttempt = &ref
	spec.ExecutionID = row.ExecutionID
	return filament.Attempt{Lease: filament.LeaseToken{Tenant: filament.TenantID(row.TenantID), Attempt: ref}, Spec: spec, Revision: row.DesiredRevision, StartedAt: row.StartedAt.Time, ExpiresAt: row.ExpiresAt.Time, EndedAt: fromTimestamptz(row.EndedAt), Termination: filament.AttemptTermination(row.Termination.String), Reason: row.Reason}, nil
}

// latestPositions reads the cumulative snapshot stored with the last committed epoch.
func latestPositions(ctx context.Context, q *sqlcgen.Queries, stream *sqlcgen.ReplicationStream) (filament.DomainPositions, error) {
	raw, err := q.LatestStreamPositions(ctx, sqlcgen.LatestStreamPositionsParams{StreamID: stream.ID, TenantID: stream.TenantID})
	if errors.Is(err, pgx.ErrNoRows) {
		return make(filament.DomainPositions), nil
	}
	if err != nil {
		return nil, err
	}
	var positions filament.DomainPositions
	if err := json.Unmarshal(raw, &positions); err != nil {
		return nil, fmt.Errorf("stream: committed positions: %w", err)
	}
	if positions == nil {
		positions = make(filament.DomainPositions)
	}
	return positions, nil
}
