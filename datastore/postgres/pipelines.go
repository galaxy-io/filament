package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/encoding/protojson"

	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/postgres/sqlcgen"
)

// CreatePipeline inserts a pipeline at version 1.
func (s *Store) CreatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
	nodesJSON, edgesJSON, err := marshalGraph(p)
	if err != nil {
		return nil, err
	}
	err = s.q.CreatePipeline(ctx, sqlcgen.CreatePipelineParams{
		PipelineID: p.GetId(),
		TenantID:   p.GetTenant(),
		Name:       p.GetName(),
		Nodes:      nodesJSON,
		Edges:      edgesJSON,
	})
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: create pipeline: %w", err)
	}
	created := cloneProto(p)
	created.Version = 1
	return created, nil
}

// UpdatePipeline applies p when its version matches and returns version+1.
func (s *Store) UpdatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error) {
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
			return nil, fmt.Errorf("update pipeline %q at version %d: %w", p.GetId(), p.GetVersion(), ingestion.ErrVersionConflict)
		}
		return nil, fmt.Errorf("datastore/postgres: update pipeline: %w", err)
	}
	next := cloneProto(p)
	next.Version = newVersion
	return next, nil
}

// LoadPipeline loads one pipeline by id.
func (s *Store) LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error) {
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

// ListPipelines returns pipelines, optionally filtered by tenant.
func (s *Store) ListPipelines(ctx context.Context, tenant string) ([]*ingestionv1.Pipeline, error) {
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

// DeletePipeline removes a pipeline by id.
func (s *Store) DeletePipeline(ctx context.Context, id string) error {
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
