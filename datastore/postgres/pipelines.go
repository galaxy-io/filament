package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// ErrVersionConflict is returned by PipelineStore.Update when the caller's
// pipeline.Version does not match the currently stored version — the
// optimistic-lock check server/pipelines.go's in-memory UpdatePipeline never
// actually performed (it just incremented and overwrote unconditionally).
var ErrVersionConflict = errors.New("pipeline version conflict")

// PipelineStore is a Postgres-backed store for the pipeline node graph
// (api/ingestion/v1 Pipeline). It intentionally does not implement the
// connect-go Server interface directly — server/pipelines.go should call
// through this store instead of its current in-memory map, translating
// ErrVersionConflict to a connect.CodeFailedPrecondition/Aborted response.
type PipelineStore struct {
	q *sqlcgen.Queries
}

// NewPipelineStore wraps an already-connected pool.
func NewPipelineStore(pool *pgxpool.Pool) *PipelineStore {
	return &PipelineStore{q: sqlcgen.New(pool)}
}

// Create inserts a new pipeline at version 1, ignoring any version/id set on
// the input (mirroring CreatePipeline's current semantics of assigning a
// fresh id server-side).
func (s *PipelineStore) Create(ctx context.Context, id, tenant, name string, nodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) (*ingestionv1.Pipeline, error) {
	p := &ingestionv1.Pipeline{Id: id, Tenant: tenant, Name: name, Nodes: nodes, Edges: edges, Version: 1}
	nodesJSON, edgesJSON, err := marshalGraph(p)
	if err != nil {
		return nil, err
	}
	err = s.q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{
		PipelineID: id,
		TenantID:   tenant,
		Name:       name,
		Nodes:      nodesJSON,
		Edges:      edgesJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline: %w", err)
	}
	return p, nil
}

// Update applies p if p.Version matches the stored version, then returns the
// new row (version+1). Returns ErrVersionConflict on mismatch (or on a
// not-yet-existing pipeline_id) so the caller can return a real conflict
// instead of silently overwriting a concurrent edit.
func (s *PipelineStore) Update(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	if p.GetId() == "" {
		return nil, fmt.Errorf("datastore/postgres: pipeline id is required")
	}
	nodesJSON, edgesJSON, err := marshalGraph(p)
	if err != nil {
		return nil, err
	}
	newVersion, err := s.q.UpdatePipeline(ctx, sqlcgen.UpdatePipelineParams{
		Name:            p.GetName(),
		Nodes:           nodesJSON,
		Edges:           edgesJSON,
		PipelineID:      p.GetId(),
		ExpectedVersion: p.GetVersion(),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("update pipeline %q at version %d: %w", p.GetId(), p.GetVersion(), ErrVersionConflict)
		}
		return nil, fmt.Errorf("datastore/postgres: update pipeline: %w", err)
	}
	next := cloneProto(p)
	next.Version = newVersion
	return next, nil
}

// Get loads one pipeline by id.
func (s *PipelineStore) Get(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
	row, err := s.q.GetPipeline(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get pipeline %q: %w", id, ingestion.ErrNotFound)
		}
		return nil, fmt.Errorf("datastore/postgres: get pipeline: %w", err)
	}
	nodes, edges, err := unmarshalGraph(row.Nodes, row.Edges)
	if err != nil {
		return nil, err
	}
	return &ingestionv1.Pipeline{Id: row.PipelineID, Tenant: row.TenantID, Name: row.Name, Nodes: nodes, Edges: edges, Version: row.Version}, nil
}

// List returns a tenant's pipelines.
func (s *PipelineStore) List(ctx context.Context, tenant string) ([]*ingestionv1.Pipeline, error) {
	rows, err := s.q.ListPipelines(ctx, tenant)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: list pipelines: %w", err)
	}
	out := make([]*ingestionv1.Pipeline, len(rows))
	for i, row := range rows {
		nodes, edges, err := unmarshalGraph(row.Nodes, row.Edges)
		if err != nil {
			return nil, err
		}
		out[i] = &ingestionv1.Pipeline{Id: row.PipelineID, Tenant: row.TenantID, Name: row.Name, Nodes: nodes, Edges: edges, Version: row.Version}
	}
	return out, nil
}

// Delete removes a pipeline by id.
func (s *PipelineStore) Delete(ctx context.Context, id string) error {
	if err := s.q.DeletePipeline(ctx, id); err != nil {
		return fmt.Errorf("datastore/postgres: delete pipeline: %w", err)
	}
	return nil
}

func marshalGraph(p *ingestionv1.Pipeline) (nodesJSON, edgesJSON []byte, err error) {
	nodesJSON, err = marshalProtoSlice(p.GetNodes())
	if err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: marshal nodes: %w", err)
	}
	edgesJSON, err = marshalProtoSlice(p.GetEdges())
	if err != nil {
		return nil, nil, fmt.Errorf("datastore/postgres: marshal edges: %w", err)
	}
	return nodesJSON, edgesJSON, nil
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
