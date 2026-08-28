package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// CreatePipeline stores a new pipeline.
func (s *Store) CreatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	workerCfg, err := marshalWorkerConfiguration(p.GetWorkerConfiguration())
	if err != nil {
		return nil, err
	}
	createdAt, err := s.q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{PipelineID: p.GetId(), TenantID: p.GetTenantId(), Name: p.GetName(), Description: p.GetDescription(), WorkerConfiguration: workerCfg})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline: %w", err)
	}
	out := cloneProto(p)
	out.CreatedAt = timestampMillis(createdAt)
	out.UpdatedAt = out.CreatedAt
	return out, nil
}

// CreatePipelineWithSchedule stores a pipeline and optional schedule atomically.
func (s *Store) CreatePipelineWithSchedule(ctx context.Context, p *ingestionv1.Pipeline, schedule *filament.ScheduleState) (*ingestionv1.Pipeline, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: begin pipeline creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	workerCfg, err := marshalWorkerConfiguration(p.GetWorkerConfiguration())
	if err != nil {
		return nil, err
	}
	createdAt, err := q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{
		PipelineID:          p.GetId(),
		TenantID:            p.GetTenantId(),
		Name:                p.GetName(),
		Description:         p.GetDescription(),
		WorkerConfiguration: workerCfg,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline: %w", err)
	}
	if schedule != nil {
		if err := saveSchedule(ctx, q, *schedule); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("datastore/postgres: commit pipeline creation: %w", err)
	}
	out := cloneProto(p)
	out.CreatedAt = timestampMillis(createdAt)
	out.UpdatedAt = out.CreatedAt
	return out, nil
}

// CreatePipelineVersion appends an immutable graph version to a pipeline.
func (s *Store) CreatePipelineVersion(ctx context.Context, pipelineID string, v *ingestionv1.PipelineVersion) (*ingestionv1.PipelineVersion, error) {
	graph, err := protojson.Marshal(v.GetGraph())
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: marshal graph: %w", err)
	}
	id := uuid.NewString()
	row, err := s.q.CreatePipelineVersion(ctx, sqlcgen.CreatePipelineVersionParams{PipelineID: pipelineID, ID: id, Graph: graph})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline version: %w", err)
	}
	createdAt := timestampMillis(row.CreatedAt)
	return &ingestionv1.PipelineVersion{Id: row.ID, Version: row.Version, Graph: v.GetGraph(), CreatedAt: createdAt, UpdatedAt: createdAt}, nil
}

// UpdatePipeline updates a pipeline's mutable metadata.
func (s *Store) UpdatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	workerCfg, err := marshalWorkerConfiguration(p.GetWorkerConfiguration())
	if err != nil {
		return nil, err
	}
	n, err := s.q.UpdatePipeline(ctx, sqlcgen.UpdatePipelineParams{PipelineID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(), WorkerConfiguration: workerCfg})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: update pipeline: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("pipeline %q: %w", p.GetId(), filament.ErrNotFound)
	}
	return s.LoadPipeline(ctx, p.GetId())
}

// LoadPipeline returns a pipeline by ID, including soft-deleted ones so callers
// can still read a deleted pipeline's metadata. DeletedAt tells them apart.
func (s *Store) LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
	row, err := s.q.GetPipeline(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get pipeline %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: get pipeline: %w", err)
	}
	out := pipelineFromRow(row.ID, row.TenantID, row.Name, row.Description)
	if row.CurrentVersionID.Valid {
		out.CurrentVersion, err = s.LoadPipelineVersion(ctx, row.ID, 0)
		if err != nil {
			return nil, err
		}
	}
	out.CreatedAt = timestampMillis(row.CreatedAt)
	out.UpdatedAt = timestampMillis(row.UpdatedAt)
	out.DeletedAt = timestampMillis(row.DeletedAt)
	out.CreatedByUserId = row.CreatedByUserID.String
	out.UpdatedByUserId = row.UpdatedByUserID.String
	out.DeletedByUserId = row.DeletedByUserID.String
	if out.WorkerConfiguration, err = unmarshalWorkerConfiguration(row.WorkerConfiguration); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadPipelineVersion returns a specific immutable pipeline graph version.
func (s *Store) LoadPipelineVersion(ctx context.Context, pipelineID string, version int64) (*ingestionv1.PipelineVersion, error) {
	row, err := s.q.GetPipelineVersion(ctx, sqlcgen.GetPipelineVersionParams{PipelineID: pipelineID, Version: version})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q version %d: %w", pipelineID, version, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: get pipeline version: %w", err)
	}
	graph, err := unmarshalGraph(row.Graph)
	if err != nil {
		return nil, err
	}
	return &ingestionv1.PipelineVersion{Id: row.ID, Version: row.Version, Graph: graph, CreatedAt: timestampMillis(row.CreatedAt), UpdatedAt: timestampMillis(row.UpdatedAt), CreatedByUserId: row.CreatedByUserID.String, UpdatedByUserId: row.UpdatedByUserID.String, DeletedByUserId: row.DeletedByUserID.String}, nil
}

// ListPipelineVersions returns all of a pipeline's graph versions, newest
// first. An unknown pipeline yields an empty slice.
func (s *Store) ListPipelineVersions(ctx context.Context, f filament.PipelineVersionFilter) ([]*ingestionv1.PipelineVersion, int, error) {
	if f.SortBy == "" {
		f.SortBy = "version"
		f.SortDescending = true
	}
	count, err := s.q.CountPipelineVersions(ctx, f.PipelineID)
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/postgres: count pipeline versions: %w", err)
	}
	rows, err := s.q.ListPipelineVersions(ctx, sqlcgen.ListPipelineVersionsParams{
		PipelineID: f.PipelineID, SortBy: f.SortBy, SortDesc: f.SortDescending,
		Lim: int32(f.Limit), OffsetRows: int32(f.Offset), //nolint:gosec // API pagination is capped
	})
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/postgres: list pipeline versions: %w", err)
	}
	out := make([]*ingestionv1.PipelineVersion, len(rows))
	for i, row := range rows {
		graph, err := unmarshalGraph(row.Graph)
		if err != nil {
			return nil, 0, err
		}
		out[i] = &ingestionv1.PipelineVersion{Id: row.ID, Version: row.Version, Graph: graph, CreatedAt: timestampMillis(row.CreatedAt), UpdatedAt: timestampMillis(row.UpdatedAt), CreatedByUserId: row.CreatedByUserID.String, UpdatedByUserId: row.UpdatedByUserID.String, DeletedByUserId: row.DeletedByUserID.String}
	}
	return out, int(count), nil
}

// ListPipelines returns pipelines matching the filter.
func (s *Store) ListPipelines(ctx context.Context, f filament.PipelineFilter) ([]*ingestionv1.Pipeline, int, error) {
	search := escapeLikePattern(strings.TrimSpace(f.Search))
	count, err := s.q.CountPipelines(ctx, sqlcgen.CountPipelinesParams{TenantID: f.Tenant, IncludeDeleted: f.IncludeDeleted, Search: search})
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/postgres: count pipelines: %w", err)
	}
	rows, err := s.q.ListPipelines(ctx, sqlcgen.ListPipelinesParams{
		TenantID: f.Tenant, IncludeDeleted: f.IncludeDeleted, Search: search,
		SortBy: f.SortBy, SortDesc: f.SortDescending,
		Lim: int32(f.Limit), OffsetRows: int32(f.Offset), //nolint:gosec // API pagination is capped
	})
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/postgres: list pipelines: %w", err)
	}
	out := make([]*ingestionv1.Pipeline, len(rows))
	for i, row := range rows {
		out[i] = pipelineFromRow(row.ID, row.TenantID, row.Name, row.Description)
		if row.CurrentVersionID.Valid {
			out[i].CurrentVersion, err = s.LoadPipelineVersion(ctx, row.ID, 0)
			if err != nil {
				return nil, 0, err
			}
		}
		out[i].CreatedAt = timestampMillis(row.CreatedAt)
		out[i].UpdatedAt = timestampMillis(row.UpdatedAt)
		out[i].DeletedAt = timestampMillis(row.DeletedAt)
		out[i].CreatedByUserId = row.CreatedByUserID.String
		out[i].UpdatedByUserId = row.UpdatedByUserID.String
		out[i].DeletedByUserId = row.DeletedByUserID.String
		if out[i].WorkerConfiguration, err = unmarshalWorkerConfiguration(row.WorkerConfiguration); err != nil {
			return nil, 0, err
		}
	}
	return out, int(count), nil
}

func timestampMillis(ts pgtype.Timestamptz) int64 {
	if !ts.Valid {
		return 0
	}
	return ts.Time.UnixMilli()
}

func pipelineFromRow(id, tenant, name, description string) *ingestionv1.Pipeline {
	return &ingestionv1.Pipeline{Id: id, TenantId: tenant, Name: name, Description: description}
}

// DeletePipeline soft-deletes a pipeline and removes its schedules and pending
// scheduled runs so the scheduler stops firing it and nothing lingers as
// upcoming work. Versions and run history are kept.
func (s *Store) DeletePipeline(ctx context.Context, id string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("datastore/postgres: begin pipeline delete: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.DeletePipeline(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete pipeline: %w", err)
	}
	// Schedules go before runs: a concurrent reconcile insert either commits
	// ahead of this delete's parent-row lock (the runs reap below still sees
	// it) or fails its schedules FK once the lock is taken.
	if err := q.DeletePipelineSchedules(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete pipeline schedules: %w", err)
	}
	if err := q.DeletePipelineScheduledRuns(ctx, sqlcgen.DeletePipelineScheduledRunsParams{
		PipelineID: toText(id),
		Status:     int16(filament.RunScheduled), //nolint:gosec // small enum
	}); err != nil {
		return fmt.Errorf("datastore/postgres: delete pipeline scheduled runs: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("datastore/postgres: commit pipeline delete: %w", err)
	}
	return nil
}

// marshalWorkerConfiguration renders the pipeline's worker configuration for
// the JSONB column. A nil configuration marshals to nil, which the update
// coalesces to the stored value: a client that sends a Pipeline without this
// field (renaming one, say) leaves the configuration alone instead of erasing
// it.
func marshalWorkerConfiguration(cfg *ingestionv1.WorkerConfiguration) ([]byte, error) {
	if cfg == nil {
		return nil, nil
	}
	b, err := protojson.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: marshal worker configuration: %w", err)
	}
	return b, nil
}

func unmarshalWorkerConfiguration(raw []byte) (*ingestionv1.WorkerConfiguration, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	cfg := &ingestionv1.WorkerConfiguration{}
	if err := protojson.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("datastore/postgres: unmarshal worker configuration: %w", err)
	}
	if cfg.GetResources() == nil && len(cfg.GetNodeSelector()) == 0 && len(cfg.GetTolerations()) == 0 {
		return nil, nil
	}
	return cfg, nil
}

func unmarshalGraph(graphJSON []byte) (*ingestionv1.PipelineGraph, error) {
	graph := &ingestionv1.PipelineGraph{}
	if err := protojson.Unmarshal(graphJSON, graph); err != nil {
		return nil, fmt.Errorf("datastore/postgres: unmarshal graph: %w", err)
	}
	return graph, nil
}
