package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// ResolveReplicationStream reuses the active route generation when its
// continuity fingerprint matches. Otherwise it retires that generation and
// creates the caller-supplied stream as the next generation atomically.
func (s *Store) ResolveReplicationStream(ctx context.Context, desired filament.ReplicationStream) (filament.ReplicationStream, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: begin replication stream resolution: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	stream, err := resolveReplicationStream(ctx, s.q.WithTx(tx), desired)
	if err != nil {
		return filament.ReplicationStream{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: commit replication stream resolution: %w", err)
	}
	return stream, nil
}

// resolveReplicationStream resolves a stream using the caller's transaction.
// Run admission uses this helper so retiring a predecessor and inserting the
// run either both commit or both roll back.
func resolveReplicationStream(ctx context.Context, q *sqlcgen.Queries, desired filament.ReplicationStream) (filament.ReplicationStream, error) {
	if desired.ID == "" || desired.Tenant == "" || desired.PipelineID == "" || desired.Route == "" ||
		desired.SourceConnectionID == "" || desired.SinkConnectionID == "" || desired.ConsumerName == "" ||
		desired.ContinuityFingerprint == "" || desired.CreatedFromPipelineVersionID == "" {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: resolve replication stream: required identity is missing")
	}
	consumerConfigValue := desired.ConsumerConfig
	if consumerConfigValue == nil {
		consumerConfigValue = map[string]any{}
	}
	consumerConfig, err := json.Marshal(consumerConfigValue)
	if err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: marshal replication consumer config: %w", err)
	}
	if _, err := q.LockPipelineForReplicationStream(ctx, sqlcgen.LockPipelineForReplicationStreamParams{
		PipelineID: desired.PipelineID, TenantID: string(desired.Tenant),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filament.ReplicationStream{}, fmt.Errorf("resolve replication stream for pipeline %q: %w", desired.PipelineID, filament.ErrNotFound)
		}
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: lock replication stream route: %w", err)
	}

	current, err := q.GetActiveReplicationStream(ctx, sqlcgen.GetActiveReplicationStreamParams{
		PipelineID: desired.PipelineID, RouteKey: desired.Route,
	})
	if err == nil {
		if current.ContinuityFingerprint == desired.ContinuityFingerprint {
			stream, err := replicationStreamFromRow(current)
			if err != nil {
				return filament.ReplicationStream{}, err
			}
			return stream, nil
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: load active replication stream: %w", err)
	}
	if err == nil {
		if err := q.RetireReplicationStream(ctx, current.ID); err != nil {
			return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: retire replication stream: %w", err)
		}
	}
	generation, err := q.NextReplicationStreamGeneration(ctx, sqlcgen.NextReplicationStreamGenerationParams{
		PipelineID: desired.PipelineID, RouteKey: desired.Route,
	})
	if err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: choose replication stream generation: %w", err)
	}
	row, err := q.CreateReplicationStream(ctx, sqlcgen.CreateReplicationStreamParams{
		ReplicationStreamID: desired.ID, TenantID: string(desired.Tenant), PipelineID: desired.PipelineID,
		RouteKey: desired.Route, Generation: generation, SourceConnectionID: desired.SourceConnectionID,
		SinkConnectionID: desired.SinkConnectionID, ConsumerName: desired.ConsumerName,
		ConsumerConfig: consumerConfig, ContinuityFingerprint: desired.ContinuityFingerprint,
		CreatedFromPipelineVersionID: desired.CreatedFromPipelineVersionID,
	})
	if err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: create replication stream: %w", err)
	}
	stream, err := replicationStreamFromRow(row)
	if err != nil {
		return filament.ReplicationStream{}, err
	}
	return stream, nil
}

// LoadReplicationStream returns one stream generation by ID.
func (s *Store) LoadReplicationStream(ctx context.Context, id string) (filament.ReplicationStream, error) {
	row, err := s.q.GetReplicationStream(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return filament.ReplicationStream{}, fmt.Errorf("load replication stream %q: %w", id, filament.ErrNotFound)
		}
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: load replication stream: %w", err)
	}
	return replicationStreamFromRow(row)
}

// ListRetiredReplicationStreams returns predecessor generations still eligible
// for connector-side cleanup.
func (s *Store) ListRetiredReplicationStreams(ctx context.Context, pipelineID, route string) ([]filament.ReplicationStream, error) {
	rows, err := s.q.ListRetiredReplicationStreamsForRoute(ctx, sqlcgen.ListRetiredReplicationStreamsForRouteParams{
		PipelineID: pipelineID, RouteKey: route,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list retired replication streams: %w", err)
	}
	out := make([]filament.ReplicationStream, 0, len(rows))
	for _, row := range rows {
		stream, err := replicationStreamFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, stream)
	}
	return out, nil
}

// MarkReplicationStreamCleaned removes a retired generation from subsequent
// connector cleanup scans while preserving its audit row.
func (s *Store) MarkReplicationStreamCleaned(ctx context.Context, id string) error {
	if err := s.q.MarkReplicationStreamCleaned(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: mark replication stream %q cleaned: %w", id, err)
	}
	return nil
}

// ReconcileReplicationStreamResources records the complete desired membership
// of a stream. New and re-added names are pending; omitted names are retired.
func (s *Store) ReconcileReplicationStreamResources(ctx context.Context, streamID string, tenant filament.TenantID, resources []string, bootstrapMode string) ([]filament.ReplicationStreamResource, error) {
	if streamID == "" || tenant == "" || strings.TrimSpace(bootstrapMode) == "" {
		return nil, fmt.Errorf("datastore/postgres: reconcile replication stream resources: required identity is missing")
	}
	names := append([]string(nil), resources...)
	slices.Sort(names)
	names = slices.Compact(names)
	for _, name := range names {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("datastore/postgres: reconcile replication stream resources: resource name is empty")
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: begin replication resource reconciliation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if _, err := q.LockReplicationStream(ctx, sqlcgen.LockReplicationStreamParams{
		ReplicationStreamID: streamID, TenantID: string(tenant),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("reconcile replication stream %q: %w", streamID, filament.ErrNotFound)
		}
		return nil, fmt.Errorf("datastore/postgres: lock replication stream: %w", err)
	}
	for _, name := range names {
		if err := q.UpsertReplicationStreamResource(ctx, sqlcgen.UpsertReplicationStreamResourceParams{
			ReplicationStreamID: streamID, TenantID: string(tenant), ResourceName: name,
			BootstrapMode: bootstrapMode, BootstrapConfig: []byte(`{}`),
		}); err != nil {
			return nil, fmt.Errorf("datastore/postgres: reconcile replication resource %q: %w", name, err)
		}
	}
	if err := q.RetireReplicationStreamResources(ctx, sqlcgen.RetireReplicationStreamResourcesParams{
		ReplicationStreamID: streamID, ResourceNames: names,
	}); err != nil {
		return nil, fmt.Errorf("datastore/postgres: retire removed replication resources: %w", err)
	}
	rows, err := q.ListReplicationStreamResources(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list reconciled replication resources: %w", err)
	}
	out, err := replicationStreamResourcesFromRows(rows)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("datastore/postgres: commit replication resource reconciliation: %w", err)
	}
	return out, nil
}

// ListReplicationStreamResources returns current and historical resource
// membership for one stream, sorted by resource name.
func (s *Store) ListReplicationStreamResources(ctx context.Context, streamID string) ([]filament.ReplicationStreamResource, error) {
	if _, err := s.q.GetReplicationStream(ctx, streamID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("list replication stream %q resources: %w", streamID, filament.ErrNotFound)
		}
		return nil, fmt.Errorf("datastore/postgres: load replication stream: %w", err)
	}
	rows, err := s.q.ListReplicationStreamResources(ctx, streamID)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list replication stream resources: %w", err)
	}
	return replicationStreamResourcesFromRows(rows)
}

func replicationStreamFromRow(row *sqlcgen.ReplicationStream) (filament.ReplicationStream, error) {
	var consumerConfig map[string]any
	if err := json.Unmarshal(row.ConsumerConfig, &consumerConfig); err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("datastore/postgres: unmarshal replication consumer config: %w", err)
	}
	return filament.ReplicationStream{
		ID: row.ID, Tenant: filament.TenantID(row.TenantID), PipelineID: row.PipelineID,
		Route: row.RouteKey, Generation: row.Generation, SourceConnectionID: row.SourceConnectionID,
		SinkConnectionID: row.SinkConnectionID, ConsumerName: row.ConsumerName,
		ConsumerConfig: consumerConfig, ContinuityFingerprint: row.ContinuityFingerprint,
		Status: filament.ReplicationStreamStatus(row.Status), CreatedFromPipelineVersionID: row.CreatedFromPipelineVersionID,
		Error: row.Error.String, RetiredAt: fromTimestamptz(row.RetiredAt),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}, nil
}

func replicationStreamResourcesFromRows(rows []*sqlcgen.ReplicationStreamResource) ([]filament.ReplicationStreamResource, error) {
	out := make([]filament.ReplicationStreamResource, 0, len(rows))
	for _, row := range rows {
		var bootstrapConfig map[string]any
		if err := json.Unmarshal(row.BootstrapConfig, &bootstrapConfig); err != nil {
			return nil, fmt.Errorf("datastore/postgres: unmarshal replication bootstrap config: %w", err)
		}
		out = append(out, filament.ReplicationStreamResource{
			ID: row.ID, ReplicationStreamID: row.ReplicationStreamID, Tenant: filament.TenantID(row.TenantID),
			Resource: row.ResourceName, Status: filament.ReplicationStreamResourceStatus(row.Status),
			BootstrapMode: row.BootstrapMode, BootstrapConfig: bootstrapConfig,
			SchemaFingerprint: row.SchemaFingerprint.String, BootstrapRun: filament.RunID(row.BootstrapRunID.String),
			BootstrapStartedAt: fromTimestamptz(row.BootstrapStartedAt), ActivatedAt: fromTimestamptz(row.ActivatedAt),
			RetiredAt: fromTimestamptz(row.RetiredAt), Error: row.Error.String,
			CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		})
	}
	return out, nil
}
