package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// CreatePipeline stores a new pipeline.
func (s *Store) CreatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	err := s.q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{PipelineID: p.GetId(), TenantID: p.GetTenantId(), Name: p.GetName(), Description: p.GetDescription()})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline: %w", err)
	}
	return cloneProto(p), nil
}

// CreatePipelineWithSchedule stores a pipeline and optional schedule atomically.
func (s *Store) CreatePipelineWithSchedule(ctx context.Context, p *ingestionv1.Pipeline, schedule *filament.ScheduleState) (*ingestionv1.Pipeline, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: begin pipeline creation: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	if err := q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{
		PipelineID:  p.GetId(),
		TenantID:    p.GetTenantId(),
		Name:        p.GetName(),
		Description: p.GetDescription(),
	}); err != nil {
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
	return cloneProto(p), nil
}

// CreatePipelineVersion appends an immutable graph version to a pipeline.
func (s *Store) CreatePipelineVersion(ctx context.Context, pipelineID string, v *ingestionv1.PipelineVersion) (*ingestionv1.PipelineVersion, error) {
	nodes, err := marshalProtoSlice(v.GetNodes())
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: marshal nodes: %w", err)
	}
	edges, err := marshalProtoSlice(v.GetEdges())
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: marshal edges: %w", err)
	}
	row, err := s.q.CreatePipelineVersion(ctx, sqlcgen.CreatePipelineVersionParams{PipelineID: pipelineID, Nodes: nodes, Edges: edges})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("pipeline %q: %w", pipelineID, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline version: %w", err)
	}
	return &ingestionv1.PipelineVersion{Id: pipelineID, Version: row.Version, Nodes: v.GetNodes(), Edges: v.GetEdges(), CreatedAt: row.CreatedAt.Time.UnixMilli()}, nil
}

// UpdatePipeline updates a pipeline's mutable metadata.
func (s *Store) UpdatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	n, err := s.q.UpdatePipeline(ctx, sqlcgen.UpdatePipelineParams{PipelineID: p.GetId(), Name: p.GetName(), Description: p.GetDescription()})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: update pipeline: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("pipeline %q: %w", p.GetId(), filament.ErrNotFound)
	}
	return s.LoadPipeline(ctx, p.GetId())
}

// LoadPipeline returns a pipeline by ID.
func (s *Store) LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
	row, err := s.q.GetPipeline(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get pipeline %q: %w", id, filament.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: get pipeline: %w", err)
	}
	return pipelineFromRow(row.PipelineID, row.TenantID, row.Name, row.Description, row.CurrentVersionID, row.LastRunVersionID, row.LastRunAt.Time, row.LastRunAt.Valid, row.LastRunStatus, row.LastRunBytes), nil
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
	nodes, edges, err := unmarshalGraph(row.Nodes, row.Edges)
	if err != nil {
		return nil, err
	}
	return &ingestionv1.PipelineVersion{Id: row.PipelineID, Version: row.Version, Nodes: nodes, Edges: edges, CreatedAt: row.CreatedAt.Time.UnixMilli()}, nil
}

// ListPipelineVersions returns all of a pipeline's graph versions, newest
// first. An unknown pipeline yields an empty slice.
func (s *Store) ListPipelineVersions(ctx context.Context, pipelineID string) ([]*ingestionv1.PipelineVersion, error) {
	rows, err := s.q.ListPipelineVersions(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list pipeline versions: %w", err)
	}
	out := make([]*ingestionv1.PipelineVersion, len(rows))
	for i, row := range rows {
		nodes, edges, err := unmarshalGraph(row.Nodes, row.Edges)
		if err != nil {
			return nil, err
		}
		out[i] = &ingestionv1.PipelineVersion{Id: row.PipelineID, Version: row.Version, Nodes: nodes, Edges: edges, CreatedAt: row.CreatedAt.Time.UnixMilli()}
	}
	return out, nil
}

// ListPipelines returns pipelines, optionally filtered by tenant.
func (s *Store) ListPipelines(ctx context.Context, tenant string) ([]*ingestionv1.Pipeline, error) {
	rows, err := s.q.ListPipelines(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list pipelines: %w", err)
	}
	out := make([]*ingestionv1.Pipeline, len(rows))
	for i, row := range rows {
		out[i] = pipelineFromRow(row.PipelineID, row.TenantID, row.Name, row.Description, row.CurrentVersionID, row.LastRunVersionID, row.LastRunAt.Time, row.LastRunAt.Valid, row.LastRunStatus, row.LastRunBytes)
	}
	return out, nil
}

func pipelineFromRow(id, tenant, name, description string, current, lastVersion int64, lastAt interface{ UnixMilli() int64 }, valid bool, status int16, bytes int64) *ingestionv1.Pipeline {
	var at int64
	if valid {
		at = lastAt.UnixMilli()
	}
	lastStatus := ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED
	if lastVersion != 0 {
		lastStatus = runStatusToPipelineProto(status)
	}
	return &ingestionv1.Pipeline{Id: id, TenantId: tenant, Name: name, Description: description, CurrentVersionId: current, LastRunVersionId: lastVersion, LastRunAt: at, LastRunStatus: lastStatus, LastRunBytes: bytes}
}

func runStatusToPipelineProto(status int16) ingestionv1.RunStatus {
	return ingestionv1.RunStatus(int32(status) + 1)
}

// DeletePipeline soft-deletes a pipeline and removes its schedules so the
// scheduler stops firing it. Versions and run history are kept.
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
	if err := q.DeletePipelineSchedules(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete pipeline schedules: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("datastore/postgres: commit pipeline delete: %w", err)
	}
	return nil
}

func unmarshalGraph(nodesJSON, edgesJSON []byte) ([]*ingestionv1.PipelineNode, []*ingestionv1.PipelineEdge, error) {
	var rawNodes []json.RawMessage
	if err := json.Unmarshal(nodesJSON, &rawNodes); err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: unmarshal nodes: %w", err)
	}
	nodes := make([]*ingestionv1.PipelineNode, len(rawNodes))
	for i, raw := range rawNodes {
		nodes[i] = &ingestionv1.PipelineNode{}
		if err := protojson.Unmarshal(raw, nodes[i]); err != nil {
			return nil, nil, fmt.Errorf("datastore/postgres: unmarshal node: %w", err)
		}
	}
	var rawEdges []json.RawMessage
	if err := json.Unmarshal(edgesJSON, &rawEdges); err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: unmarshal edges: %w", err)
	}
	edges := make([]*ingestionv1.PipelineEdge, len(rawEdges))
	for i, raw := range rawEdges {
		edges[i] = &ingestionv1.PipelineEdge{}
		if err := protojson.Unmarshal(raw, edges[i]); err != nil {
			return nil, nil, fmt.Errorf("datastore/postgres: unmarshal edge: %w", err)
		}
	}
	return nodes, edges, nil
}
