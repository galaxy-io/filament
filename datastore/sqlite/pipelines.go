package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite/sqlcgen"
)

// CreatePipeline stores a new pipeline.
func (s *Store) CreatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	if err := s.ensureTenantRow(ctx, p.GetTenantId()); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: ensure tenant: %w", err)
	}
	return s.createPipeline(ctx, s.q, p)
}

func (s *Store) createPipeline(ctx context.Context, q *sqlcgen.Queries, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	workerCfg, err := marshalWorkerConfiguration(p.GetWorkerConfiguration())
	if err != nil {
		return nil, err
	}
	now := nowMillis()
	err = q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{
		PipelineID: p.GetId(), TenantID: p.GetTenantId(), Name: p.GetName(), Description: p.GetDescription(),
		WorkerConfiguration: workerCfg, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: create pipeline: %w", err)
	}
	out := proto.Clone(p).(*ingestionv1.Pipeline)
	out.CreatedAt = now
	out.UpdatedAt = now
	return out, nil
}

// CreatePipelineWithSchedule stores a pipeline and optional schedule atomically.
func (s *Store) CreatePipelineWithSchedule(ctx context.Context, p *ingestionv1.Pipeline, schedule *filament.ScheduleState) (*ingestionv1.Pipeline, error) {
	// The tenant mint must precede the transaction: the single-connection
	// handle would deadlock on a db call while the tx holds the connection.
	if err := s.ensureTenantRow(ctx, p.GetTenantId()); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: ensure tenant: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: begin pipeline creation: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	out, err := s.createPipeline(ctx, q, p)
	if err != nil {
		return nil, err
	}
	if schedule != nil {
		if err := saveSchedule(ctx, q, *schedule); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: commit pipeline creation: %w", err)
	}
	return out, nil
}

// CreatePipelineVersion appends an immutable graph version to a pipeline. The
// single-writer connection serializes the read-max-insert the postgres store
// expresses with a locking CTE.
func (s *Store) CreatePipelineVersion(ctx context.Context, tenant filament.TenantID, pipelineID string, v *ingestionv1.PipelineVersion) (*ingestionv1.PipelineVersion, error) {
	graph, err := protojson.Marshal(v.GetGraph())
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: marshal graph: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: begin pipeline version: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)

	version, err := q.NextPipelineVersion(ctx, sqlcgen.NextPipelineVersionParams{TenantID: string(tenant), PipelineID: pipelineID})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: next pipeline version: %w", err)
	}
	now := nowMillis()
	row, err := q.InsertPipelineVersion(ctx, sqlcgen.InsertPipelineVersionParams{
		VersionID: uuid.NewString(), TenantID: string(tenant),
		PipelineID: pipelineID, VersionPipelineID: pipelineID,
		Version: version, Graph: string(graph), CreatedAt: now, UpdatedAt: now,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: create pipeline version: %w", err)
	}
	if err := q.SetCurrentPipelineVersion(ctx, sqlcgen.SetCurrentPipelineVersionParams{
		TenantID: string(tenant), PipelineID: pipelineID, VersionID: sql.NullString{String: row.ID, Valid: true}, UpdatedAt: now,
	}); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: set current pipeline version: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: commit pipeline version: %w", err)
	}
	return &ingestionv1.PipelineVersion{Id: row.ID, Version: row.Version, Graph: v.GetGraph(), CreatedAt: row.CreatedAt, UpdatedAt: row.CreatedAt}, nil
}

// UpdatePipeline updates a pipeline's mutable metadata.
func (s *Store) UpdatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	workerCfg, err := marshalWorkerConfiguration(p.GetWorkerConfiguration())
	if err != nil {
		return nil, err
	}
	n, err := s.q.UpdatePipeline(ctx, sqlcgen.UpdatePipelineParams{
		TenantID: p.GetTenantId(), PipelineID: p.GetId(), Name: p.GetName(), Description: p.GetDescription(),
		WorkerConfiguration: workerCfg, UpdatedAt: nowMillis(),
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: update pipeline: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("pipeline %q: %w", p.GetId(), filament.ErrNotFound)
	}
	return s.LoadPipeline(ctx, filament.TenantID(p.GetTenantId()), p.GetId())
}

// LoadPipeline returns a pipeline by ID, including soft-deleted ones so
// callers can still read a deleted pipeline's metadata. DeletedAt tells them
// apart.
func (s *Store) LoadPipeline(ctx context.Context, tenant filament.TenantID, id string) (*ingestionv1.Pipeline, error) {
	row, err := s.q.GetPipeline(ctx, sqlcgen.GetPipelineParams{TenantID: string(tenant), PipelineID: id})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get pipeline %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: get pipeline: %w", err)
	}
	return s.pipelineFromRow(ctx, tenant, row.ID, row.TenantID, row.Name, row.Description, row.CurrentVersionID,
		row.WorkerConfiguration, row.CreatedAt, row.UpdatedAt, row.DeletedAt, row.CreatedByUserID, row.UpdatedByUserID, row.DeletedByUserID)
}

func (s *Store) pipelineFromRow(ctx context.Context, tenant filament.TenantID, id, tenantID, name, description string,
	currentVersionID sql.NullString, workerCfg string, createdAt, updatedAt int64, deletedAt sql.NullInt64,
	createdBy, updatedBy, deletedBy string,
) (*ingestionv1.Pipeline, error) {
	out := &ingestionv1.Pipeline{Id: id, TenantId: tenantID, Name: name, Description: description}
	var err error
	if currentVersionID.Valid && currentVersionID.String != "" {
		out.CurrentVersion, err = s.loadPipelineVersionByID(ctx, tenant, id, currentVersionID.String)
		if err != nil {
			return nil, err
		}
	}
	out.CreatedAt = createdAt
	out.UpdatedAt = updatedAt
	out.DeletedAt = deletedAt.Int64
	out.CreatedByUserId = createdBy
	out.UpdatedByUserId = updatedBy
	out.DeletedByUserId = deletedBy
	if out.WorkerConfiguration, err = unmarshalWorkerConfiguration(workerCfg); err != nil {
		return nil, err
	}
	return out, nil
}

// LoadPipelineVersion returns a specific immutable pipeline graph version;
// version 0 means the current one.
func (s *Store) LoadPipelineVersion(ctx context.Context, tenant filament.TenantID, pipelineID string, version int64) (*ingestionv1.PipelineVersion, error) {
	if version == 0 {
		currentID, err := s.q.GetCurrentPipelineVersionID(ctx, sqlcgen.GetCurrentPipelineVersionIDParams{TenantID: string(tenant), PipelineID: pipelineID})
		if errors.Is(err, sql.ErrNoRows) || (err == nil && currentID == "") {
			return nil, fmt.Errorf("pipeline %q version %d: %w", pipelineID, version, filament.ErrNotFound)
		}
		if err != nil {
			return nil, fmt.Errorf("datastore/sqlite: get current pipeline version: %w", err)
		}
		return s.loadPipelineVersionByID(ctx, tenant, pipelineID, currentID)
	}
	row, err := s.q.GetPipelineVersionByNumber(ctx, sqlcgen.GetPipelineVersionByNumberParams{
		TenantID: string(tenant), PipelineID: pipelineID, Version: version,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q version %d: %w", pipelineID, version, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: get pipeline version: %w", err)
	}
	return versionFromRow(row.ID, row.Version, row.Graph, row.CreatedAt, row.UpdatedAt, row.CreatedByUserID, row.UpdatedByUserID, row.DeletedByUserID)
}

func (s *Store) loadPipelineVersionByID(ctx context.Context, tenant filament.TenantID, pipelineID, versionID string) (*ingestionv1.PipelineVersion, error) {
	row, err := s.q.GetPipelineVersionByID(ctx, sqlcgen.GetPipelineVersionByIDParams{
		TenantID: string(tenant), PipelineID: pipelineID, VersionID: versionID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q version %q: %w", pipelineID, versionID, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/sqlite: get pipeline version: %w", err)
	}
	return versionFromRow(row.ID, row.Version, row.Graph, row.CreatedAt, row.UpdatedAt, row.CreatedByUserID, row.UpdatedByUserID, row.DeletedByUserID)
}

func versionFromRow(id string, version int64, graphJSON string, createdAt, updatedAt int64, createdBy, updatedBy, deletedBy string) (*ingestionv1.PipelineVersion, error) {
	graph := &ingestionv1.PipelineGraph{}
	if err := protojson.Unmarshal([]byte(graphJSON), graph); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: unmarshal graph: %w", err)
	}
	return &ingestionv1.PipelineVersion{
		Id: id, Version: version, Graph: graph, CreatedAt: createdAt, UpdatedAt: updatedAt,
		CreatedByUserId: createdBy, UpdatedByUserId: updatedBy, DeletedByUserId: deletedBy,
	}, nil
}

// ListPipelineVersions returns a pipeline's graph versions plus the pre-page
// total; the default order is newest version first. An unknown pipeline
// yields an empty slice.
func (s *Store) ListPipelineVersions(ctx context.Context, f filament.PipelineVersionFilter) ([]*ingestionv1.PipelineVersion, int, error) {
	if f.SortBy == "" {
		f.SortBy = "version"
		f.SortDescending = true
	}
	count, err := s.q.CountPipelineVersions(ctx, sqlcgen.CountPipelineVersionsParams{TenantID: string(f.Tenant), PipelineID: f.PipelineID})
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: count pipeline versions: %w", err)
	}
	// #nosec G202 -- concatenated fragments come from sortClause/pageClause's fixed vocabulary, never caller input
	query := `SELECT ` + pipelineVersionColumns + ` FROM pipeline_versions
		WHERE tenant_id = ?1 AND pipeline_id = ?2` +
		sortClause(f.SortBy, f.SortDescending) + pageClause(f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, query, string(f.Tenant), f.PipelineID)
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list pipeline versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []*ingestionv1.PipelineVersion
	for rows.Next() {
		var (
			id, pipelineID, graph, createdBy, updatedBy, deletedBy string
			version, createdAt, updatedAt                          int64
		)
		if err := rows.Scan(&id, &pipelineID, &version, &graph, &createdAt, &updatedAt, &createdBy, &updatedBy, &deletedBy); err != nil {
			return nil, 0, fmt.Errorf("datastore/sqlite: scan pipeline version: %w", err)
		}
		v, err := versionFromRow(id, version, graph, createdAt, updatedAt, createdBy, updatedBy, deletedBy)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list pipeline versions: %w", err)
	}
	return out, int(count), nil
}

// ListPipelines returns pipelines matching the filter plus the pre-page total.
func (s *Store) ListPipelines(ctx context.Context, f filament.PipelineFilter) ([]*ingestionv1.Pipeline, int, error) {
	search := strings.TrimSpace(f.Search)
	count, err := s.q.CountPipelines(ctx, sqlcgen.CountPipelinesParams{
		TenantID: f.Tenant, IncludeDeleted: f.IncludeDeleted,
		Search: search, NameSearch: search, DescriptionSearch: search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: count pipelines: %w", err)
	}
	// #nosec G202 -- concatenated fragments come from sortClause/pageClause's fixed vocabulary, never caller input
	query := `SELECT ` + pipelineColumns + ` FROM pipelines
		WHERE tenant_id = ?1
		  AND (?2 OR is_deleted = 0)
		  AND (?3 = '' OR instr(lower(name), lower(?3)) > 0 OR instr(lower(description), lower(?3)) > 0)` +
		sortClause(f.SortBy, f.SortDescending) + pageClause(f.Limit, f.Offset)
	rows, err := s.db.QueryContext(ctx, query, f.Tenant, f.IncludeDeleted, search)
	if err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list pipelines: %w", err)
	}
	defer func() { _ = rows.Close() }()
	type pipelineRow struct {
		id, tenantID, name, description, workerCfg, createdBy, updatedBy, deletedBy string
		currentVersionID                                                            sql.NullString
		createdAt, updatedAt                                                        int64
		deletedAt                                                                   sql.NullInt64
	}
	var scanned []pipelineRow
	for rows.Next() {
		var r pipelineRow
		if err := rows.Scan(&r.id, &r.tenantID, &r.name, &r.description, &r.currentVersionID, &r.workerCfg,
			&r.createdAt, &r.updatedAt, &r.deletedAt, &r.createdBy, &r.updatedBy, &r.deletedBy); err != nil {
			return nil, 0, fmt.Errorf("datastore/sqlite: scan pipeline: %w", err)
		}
		scanned = append(scanned, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("datastore/sqlite: list pipelines: %w", err)
	}
	out := make([]*ingestionv1.Pipeline, len(scanned))
	for i, r := range scanned {
		out[i], err = s.pipelineFromRow(ctx, filament.TenantID(r.tenantID), r.id, r.tenantID, r.name, r.description,
			r.currentVersionID, r.workerCfg, r.createdAt, r.updatedAt, r.deletedAt, r.createdBy, r.updatedBy, r.deletedBy)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, int(count), nil
}

// DeletePipeline soft-deletes a pipeline and removes its schedules and pending
// scheduled runs so the scheduler stops firing it and nothing lingers as
// upcoming work. Versions and run history are kept.
func (s *Store) DeletePipeline(ctx context.Context, tenant filament.TenantID, id string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("datastore/sqlite: begin pipeline delete: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := s.q.WithTx(tx)
	now := nowMillis()
	if err := q.DeletePipeline(ctx, sqlcgen.DeletePipelineParams{
		TenantID: string(tenant), PipelineID: id, DeleteStamp: deleteStamp(now),
		DeletedAt: sql.NullInt64{Int64: now, Valid: true}, UpdatedAt: now,
	}); err != nil {
		return fmt.Errorf("datastore/sqlite: delete pipeline: %w", err)
	}
	if err := q.DeletePipelineSchedules(ctx, sqlcgen.DeletePipelineSchedulesParams{TenantID: string(tenant), PipelineID: id}); err != nil {
		return fmt.Errorf("datastore/sqlite: delete pipeline schedules: %w", err)
	}
	if err := q.DeletePipelineScheduledRuns(ctx, sqlcgen.DeletePipelineScheduledRunsParams{
		TenantID: string(tenant), PipelineID: sql.NullString{String: id, Valid: true}, Status: int64(filament.RunScheduled),
	}); err != nil {
		return fmt.Errorf("datastore/sqlite: delete pipeline scheduled runs: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("datastore/sqlite: commit pipeline delete: %w", err)
	}
	return nil
}

// marshalWorkerConfiguration renders the pipeline's worker configuration for
// the JSON column. A nil configuration marshals to "", which the update
// coalesces to the stored value: a client that sends a Pipeline without this
// field (renaming one, say) leaves the configuration alone instead of erasing
// it.
func marshalWorkerConfiguration(cfg *ingestionv1.WorkerConfiguration) (string, error) {
	if cfg == nil {
		return "", nil
	}
	b, err := protojson.Marshal(cfg)
	if err != nil {
		return "", fmt.Errorf("datastore/sqlite: marshal worker configuration: %w", err)
	}
	return string(b), nil
}

func unmarshalWorkerConfiguration(raw string) (*ingestionv1.WorkerConfiguration, error) {
	if raw == "" {
		return nil, nil
	}
	cfg := &ingestionv1.WorkerConfiguration{}
	if err := protojson.Unmarshal([]byte(raw), cfg); err != nil {
		return nil, fmt.Errorf("datastore/sqlite: unmarshal worker configuration: %w", err)
	}
	if cfg.GetResources() == nil && len(cfg.GetNodeSelector()) == 0 && len(cfg.GetTolerations()) == 0 {
		return nil, nil
	}
	return cfg, nil
}
