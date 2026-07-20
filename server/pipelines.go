package server

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// CreatePipeline stores a new pipeline and assigns its id.
func (a *Server) CreatePipeline(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineRequest]) (*connect.Response[ingestionv1.CreatePipelineResponse], error) {
	id := uuid.NewString()
	pipeline := &ingestionv1.Pipeline{
		Id:      id,
		Tenant:  defaultTenant(req.Msg.GetTenant()),
		Name:    req.Msg.GetName(),
		Nodes:   req.Msg.GetNodes(),
		Edges:   req.Msg.GetEdges(),
		Version: 1,
	}
	created, err := a.store.CreatePipeline(ctx, pipeline)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.CreatePipelineResponse{Pipeline: created}), nil
}

// UpdatePipeline replaces the stored pipeline, bumping its version.
func (a *Server) UpdatePipeline(ctx context.Context, req *connect.Request[ingestionv1.UpdatePipelineRequest]) (*connect.Response[ingestionv1.UpdatePipelineResponse], error) {
	pipeline := req.Msg.GetPipeline()
	if pipeline == nil || pipeline.GetId() == "" {
		return nil, fmt.Errorf("pipeline.id is required")
	}
	if pipeline.Tenant == "" {
		pipeline.Tenant = "t1"
	}
	next, err := a.store.UpdatePipeline(ctx, pipeline)
	if errors.Is(err, filament.ErrVersionConflict) {
		return nil, connect.NewError(connect.CodeAborted, err)
	}
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.UpdatePipelineResponse{Pipeline: next}), nil
}

// GetPipeline returns the pipeline by id.
func (a *Server) GetPipeline(ctx context.Context, req *connect.Request[ingestionv1.GetPipelineRequest]) (*connect.Response[ingestionv1.GetPipelineResponse], error) {
	pipeline, err := a.store.LoadPipeline(ctx, req.Msg.GetId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.GetPipelineResponse{Pipeline: pipeline}), nil
}

// ListPipelines returns pipelines, optionally filtered by tenant.
func (a *Server) ListPipelines(ctx context.Context, req *connect.Request[ingestionv1.ListPipelinesRequest]) (*connect.Response[ingestionv1.ListPipelinesResponse], error) {
	pipelines, err := a.store.ListPipelines(ctx, req.Msg.GetTenant())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.ListPipelinesResponse{Pipelines: pipelines}), nil
}

// DeletePipeline removes the pipeline by id.
func (a *Server) DeletePipeline(ctx context.Context, req *connect.Request[ingestionv1.DeletePipelineRequest]) (*connect.Response[ingestionv1.DeletePipelineResponse], error) {
	if err := a.store.DeletePipeline(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.DeletePipelineResponse{}), nil
}

// RunPipeline groups the pipeline's edges into per-route runs and submits each
// to the orchestrator.
func (a *Server) RunPipeline(ctx context.Context, req *connect.Request[ingestionv1.RunPipelineRequest]) (*connect.Response[ingestionv1.RunPipelineResponse], error) {
	pipeline, err := a.store.LoadPipeline(ctx, req.Msg.GetPipelineId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	nodes := map[string]*ingestionv1.PipelineNode{}
	connections := map[string]filament.Connection{}
	for _, node := range pipeline.GetNodes() {
		nodes[node.GetId()] = node
		if _, ok := connections[node.GetConnectionId()]; !ok {
			conn, err := a.store.LoadConnection(ctx, node.GetConnectionId())
			if err != nil {
				return nil, fmt.Errorf("load connection %q: %w", node.GetConnectionId(), err)
			}
			connections[node.GetConnectionId()] = conn
		}
	}

	var bindings []*ingestionv1.RunBinding
	token := req.Msg.GetClientToken()
	if token == "" {
		token = uuid.NewString()
	}
	groups, err := groupEdges(pipeline.GetEdges(), nodes)
	if err != nil {
		return nil, err
	}
	options := runOptionsFromProto(req.Msg.GetOptions())
	for _, group := range groups {
		key := group.key
		var resources []string
		var selectors []string
		if !group.all {
			for resource := range group.resources {
				resources = append(resources, resource)
			}
			slices.Sort(resources)
			for selector := range group.selectors {
				selectors = append(selectors, selector)
			}
			slices.Sort(selectors)
		}
		sourceRef, err := a.resolveNodeRef(group.source, connections)
		if err != nil {
			return nil, err
		}
		sinkRef, err := a.resolveNodeRef(group.sink, connections)
		if err != nil {
			return nil, err
		}
		run, err := a.orch.Submit(ctx, filament.RunRequest{
			Tenant:         filament.TenantID(defaultTenant(pipeline.GetTenant())),
			IdempotencyKey: fmt.Sprintf("%s:%s:%s", pipeline.GetId(), token, key),
			Source:         sourceRef,
			Sink:           sinkRef,
			Resources:      resources,
			Selectors:      selectors,
			IngestionType:  group.ingestionType,
			Options:        options,
		})
		if err != nil {
			return nil, err
		}
		fmt.Printf("[ingestion-api] RunPipeline route=%s source=%s sink=%s resources=%v run=%s\n", key, sourceRef.Provider, sinkRef.Provider, resources, run)
		bindings = append(bindings, &ingestionv1.RunBinding{Edge: key, RunId: string(run)})
	}
	fmt.Printf("[ingestion-api] RunPipeline pipeline=%s runs=%d\n", pipeline.GetId(), len(bindings))
	return connect.NewResponse(&ingestionv1.RunPipelineResponse{Runs: bindings}), nil
}

// routeGroup is the set of edges that share a source node, sink node, and
// ingestion type, and so collapse into a single run.
type routeGroup struct {
	key           string
	source        *ingestionv1.PipelineNode
	sink          *ingestionv1.PipelineNode
	from          string
	to            string
	ingestionType filament.IngestionType
	all           bool
	resources     map[string]bool
	selectors     map[string]bool
}

// groupEdges collapses edges into per-route groups, preserving first-seen order.
// An edge with no resource marks its group as "all resources".
func groupEdges(edges []*ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode) ([]*routeGroup, error) {
	byKey := map[string]*routeGroup{}
	var ordered []*routeGroup
	for _, edge := range edges {
		source := nodes[edge.GetFromNode()]
		sink := nodes[edge.GetToNode()]
		if source == nil || sink == nil {
			return nil, fmt.Errorf("edge references missing node")
		}
		ingestionType := ingestionTypeFromProto(edge.GetIngestionType()).OrDefault()
		key := fmt.Sprintf("route/%s/%s/%s", edge.GetFromNode(), edge.GetToNode(), ingestionType)
		group := byKey[key]
		if group == nil {
			group = &routeGroup{
				key:           key,
				source:        source,
				sink:          sink,
				from:          edge.GetFromNode(),
				to:            edge.GetToNode(),
				ingestionType: ingestionType,
				resources:     map[string]bool{},
				selectors:     map[string]bool{},
			}
			byKey[key] = group
			ordered = append(ordered, group)
		}
		if edge.GetResource() == "" {
			group.all = true
			continue
		}
		resource := edge.GetResource()
		group.resources[resource] = true
		if selector := edge.GetSelector(); selector != "" {
			group.selectors[selector] = true
		} else {
			group.selectors[resource] = true
		}
	}
	return ordered, nil
}

// resolveNodeRef builds the run-time Ref for a pipeline node from the reusable
// Connection it references, with the node's config/secret_refs shallow-merged
// on top as the PIPELINE overlay (node keys win). connection_id is required.
// It rejects an overlay that tries to set a CONNECTION-scoped field.
func (a *Server) resolveNodeRef(node *ingestionv1.PipelineNode, connections map[string]filament.Connection) (filament.Ref, error) {
	conn, ok := connections[node.GetConnectionId()]
	if !ok {
		return filament.Ref{}, fmt.Errorf("node %q references missing connection %q", node.GetId(), node.GetConnectionId())
	}
	overlay := structMap(node.GetConfig())
	if schema, err := a.schemaFor(connectionKindToProto(conn.Kind), conn.Connector); err == nil {
		if err := validateOverlayConfig(schema, overlay); err != nil {
			return filament.Ref{}, fmt.Errorf("node %q: %w", node.GetId(), err)
		}
	}
	return filament.Ref{
		Provider:   conn.Connector,
		Config:     mergeConfig(conn.Config, overlay),
		SecretRefs: mergeStrings(conn.SecretRefs, node.GetSecretRefs()),
	}, nil
}

// mergeConfig shallow-merges overlay over base; overlay keys win. base is not
// mutated.
func mergeConfig(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func mergeStrings(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
