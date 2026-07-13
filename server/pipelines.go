package server

import (
	"context"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	ingestion "github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"google.golang.org/protobuf/proto"
)

func (a *Server) CreatePipeline(_ context.Context, req *connect.Request[ingestionv1.CreatePipelineRequest]) (*connect.Response[ingestionv1.CreatePipelineResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.nextID++
	id := fmt.Sprintf("pipe-%d", a.nextID)
	pipeline := &ingestionv1.Pipeline{
		Id:      id,
		Tenant:  defaultTenant(req.Msg.GetTenant()),
		Name:    req.Msg.GetName(),
		Nodes:   req.Msg.GetNodes(),
		Edges:   req.Msg.GetEdges(),
		Version: 1,
	}
	a.pipelines[id] = proto.Clone(pipeline).(*ingestionv1.Pipeline)
	return connect.NewResponse(&ingestionv1.CreatePipelineResponse{Pipeline: pipeline}), nil
}

func (a *Server) UpdatePipeline(_ context.Context, req *connect.Request[ingestionv1.UpdatePipelineRequest]) (*connect.Response[ingestionv1.UpdatePipelineResponse], error) {
	pipeline := req.Msg.GetPipeline()
	if pipeline == nil || pipeline.GetId() == "" {
		return nil, fmt.Errorf("pipeline.id is required")
	}
	a.mu.Lock()
	defer a.mu.Unlock()

	next := proto.Clone(pipeline).(*ingestionv1.Pipeline)
	next.Version++
	if next.Tenant == "" {
		next.Tenant = "t1"
	}
	a.pipelines[next.Id] = next
	return connect.NewResponse(&ingestionv1.UpdatePipelineResponse{Pipeline: proto.Clone(next).(*ingestionv1.Pipeline)}), nil
}

func (a *Server) GetPipeline(_ context.Context, req *connect.Request[ingestionv1.GetPipelineRequest]) (*connect.Response[ingestionv1.GetPipelineResponse], error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	pipeline := a.pipelines[req.Msg.GetId()]
	if pipeline == nil {
		return nil, fmt.Errorf("pipeline %q not found", req.Msg.GetId())
	}
	return connect.NewResponse(&ingestionv1.GetPipelineResponse{Pipeline: proto.Clone(pipeline).(*ingestionv1.Pipeline)}), nil
}

func (a *Server) ListPipelines(_ context.Context, req *connect.Request[ingestionv1.ListPipelinesRequest]) (*connect.Response[ingestionv1.ListPipelinesResponse], error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var pipelines []*ingestionv1.Pipeline
	for _, pipeline := range a.pipelines {
		if req.Msg.GetTenant() != "" && pipeline.GetTenant() != req.Msg.GetTenant() {
			continue
		}
		pipelines = append(pipelines, proto.Clone(pipeline).(*ingestionv1.Pipeline))
	}
	slices.SortFunc(pipelines, func(a, b *ingestionv1.Pipeline) int {
		if a.GetId() < b.GetId() {
			return -1
		}
		if a.GetId() > b.GetId() {
			return 1
		}
		return 0
	})
	return connect.NewResponse(&ingestionv1.ListPipelinesResponse{Pipelines: pipelines}), nil
}

func (a *Server) DeletePipeline(_ context.Context, req *connect.Request[ingestionv1.DeletePipelineRequest]) (*connect.Response[ingestionv1.DeletePipelineResponse], error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pipelines, req.Msg.GetId())
	return connect.NewResponse(&ingestionv1.DeletePipelineResponse{}), nil
}

func (a *Server) RunPipeline(ctx context.Context, req *connect.Request[ingestionv1.RunPipelineRequest]) (*connect.Response[ingestionv1.RunPipelineResponse], error) {
	a.mu.RLock()
	stored := a.pipelines[req.Msg.GetPipelineId()]
	connections := make(map[string]*ingestionv1.Connection, len(a.connections))
	for id, conn := range a.connections {
		connections[id] = proto.Clone(conn).(*ingestionv1.Connection)
	}
	a.mu.RUnlock()
	if stored == nil {
		return nil, fmt.Errorf("pipeline %q not found", req.Msg.GetPipelineId())
	}
	pipeline := proto.Clone(stored).(*ingestionv1.Pipeline)
	nodes := map[string]*ingestionv1.PipelineNode{}
	for _, node := range pipeline.GetNodes() {
		nodes[node.GetId()] = node
	}

	var bindings []*ingestionv1.RunBinding
	token := req.Msg.GetClientToken()
	if token == "" {
		token = fmt.Sprint(time.Now().UnixNano())
	}
	type routeGroup struct {
		source        *ingestionv1.PipelineNode
		sink          *ingestionv1.PipelineNode
		from          string
		to            string
		ingestionType ingestion.IngestionType
		all           bool
		resources     map[string]bool
		selectors     map[string]bool
	}
	groups := map[string]*routeGroup{}
	var groupOrder []string
	for _, edge := range pipeline.GetEdges() {
		source := nodes[edge.GetFromNode()]
		sink := nodes[edge.GetToNode()]
		if source == nil || sink == nil {
			return nil, fmt.Errorf("edge references missing node")
		}
		ingestionType := ingestionTypeFromProto(edge.GetIngestionType()).OrDefault()
		key := fmt.Sprintf("%s||%s||%s", edge.GetFromNode(), edge.GetToNode(), ingestionType)
		group := groups[key]
		if group == nil {
			group = &routeGroup{
				source:        source,
				sink:          sink,
				from:          edge.GetFromNode(),
				to:            edge.GetToNode(),
				ingestionType: ingestionType,
				resources:     map[string]bool{},
				selectors:     map[string]bool{},
			}
			groups[key] = group
			groupOrder = append(groupOrder, key)
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
	for _, key := range groupOrder {
		group := groups[key]
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
		run, err := a.orch.Submit(ctx, ingestion.RunRequest{
			Tenant:         ingestion.TenantID(defaultTenant(pipeline.GetTenant())),
			IdempotencyKey: fmt.Sprintf("%s:%s:%s", pipeline.GetId(), token, key),
			Source:         sourceRef,
			Sink:           sinkRef,
			DataStore:      ingestion.Ref{Provider: "default"},
			Resources:      resources,
			Selectors:      selectors,
			IngestionType:  group.ingestionType,
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

// resolveNodeRef builds the run-time Ref for a pipeline node from the reusable
// Connection it references, with the node's config/secret_refs shallow-merged
// on top as the PIPELINE overlay (node keys win). connection_id is required.
// It rejects an overlay that tries to set a CONNECTION-scoped field.
func (a *Server) resolveNodeRef(node *ingestionv1.PipelineNode, connections map[string]*ingestionv1.Connection) (ingestion.Ref, error) {
	conn := connections[node.GetConnectionId()]
	if conn == nil {
		return ingestion.Ref{}, fmt.Errorf("node %q references missing connection %q", node.GetId(), node.GetConnectionId())
	}
	overlay := structMap(node.GetConfig())
	if schema, err := a.schemaFor(conn.GetKind(), conn.GetConnector()); err == nil {
		if err := validateOverlayConfig(schema, overlay); err != nil {
			return ingestion.Ref{}, fmt.Errorf("node %q: %w", node.GetId(), err)
		}
	}
	return ingestion.Ref{
		Provider:   conn.GetConnector(),
		Config:     mergeConfig(structMap(conn.GetConfig()), overlay),
		SecretRefs: mergeStrings(conn.GetSecretRefs(), node.GetSecretRefs()),
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
