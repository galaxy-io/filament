package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ingestionTypes is the full menu validation walks, in display order.
var ingestionTypes = []filament.IngestionType{
	filament.IngestionSnapshotReplace,
	filament.IngestionSnapshotUpsert,
	filament.IngestionAppend,
	filament.IngestionUpsert,
	filament.IngestionDelete,
	filament.IngestionCDC,
}

// GetConnectionCapabilities reports what one connection's connector declares
// and which ingestion types it can serve, with the validator's reason for
// each type it cannot.
func (a *Server) GetConnectionCapabilities(ctx context.Context, req *connect.Request[ingestionv1.GetConnectionCapabilitiesRequest]) (*connect.Response[ingestionv1.GetConnectionCapabilitiesResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, connectorRPCTimeout)
	defer cancel()

	conn, err := a.loadConnectionForTenant(ctx, req.Msg.GetId(), req.Msg.GetTenantId())
	if err != nil {
		if errors.Is(err, filament.ErrNotFound) {
			return nil, connect.NewError(connect.CodeNotFound, err)
		}
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	switch conn.Kind {
	case filament.ConnectorKindSource:
		source, err := a.sources.Resolve(conn.Connector)
		if err != nil {
			return nil, err
		}
		spec := source.Spec()
		return connect.NewResponse(&ingestionv1.GetConnectionCapabilitiesResponse{
			Connector:    spec.Name,
			Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
			Capabilities: sourceCapabilitiesToProto(spec),
			Ingestion: ingestionSupport(func(t filament.IngestionType) error {
				return filament.ValidateSourceIngestion(spec, t)
			}),
		}), nil
	case filament.ConnectorKindSink:
		sink, err := a.sinks.Resolve(conn.Connector)
		if err != nil {
			return nil, err
		}
		spec := sink.Spec()
		return connect.NewResponse(&ingestionv1.GetConnectionCapabilitiesResponse{
			Connector:    spec.Name,
			Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
			Capabilities: sinkCapabilitiesToProto(spec.Capabilities),
			Ingestion: ingestionSupport(func(t filament.IngestionType) error {
				return filament.ValidateSinkIngestion(spec, t)
			}),
		}), nil
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("connection %q has no connector kind", conn.ID))
	}
}

func ingestionSupport(validate func(filament.IngestionType) error) []*ingestionv1.IngestionSupport {
	out := make([]*ingestionv1.IngestionSupport, 0, len(ingestionTypes))
	for _, t := range ingestionTypes {
		entry := &ingestionv1.IngestionSupport{Type: ingestionTypeToProto(t), Supported: true}
		if err := validate(t); err != nil {
			entry.Supported = false
			entry.Reason = err.Error()
		}
		out = append(out, entry)
	}
	return out
}

// ValidatePipeline checks every edge of a graph: the ingestion types the
// source/sink pair supports, whether the chosen type is among them, and what
// per-resource configuration is still missing, with candidate values where
// the source can enumerate them.
func (a *Server) ValidatePipeline(ctx context.Context, req *connect.Request[ingestionv1.ValidatePipelineRequest]) (*connect.Response[ingestionv1.ValidatePipelineResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	nodes := make(map[string]*ingestionv1.PipelineNode, len(req.Msg.GetNodes()))
	for _, node := range req.Msg.GetNodes() {
		nodes[node.GetId()] = node
	}

	resp := &ingestionv1.ValidatePipelineResponse{}
	probes := &sourceProbes{server: a, sources: map[string]filament.Source{}, errs: map[string]error{}}
	defer probes.teardown(ctx)

	for _, edge := range req.Msg.GetEdges() {
		if err := a.validateEdge(ctx, edge, nodes, req.Msg.GetTenantId(), probes, resp); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}

	resp.Valid = len(resp.Errors) == 0
	for _, ev := range resp.Edges {
		if len(ev.Errors) > 0 || len(ev.Requirements) > 0 {
			resp.Valid = false
		}
	}
	return connect.NewResponse(resp), nil
}

// validateEdge appends the edge's verdict to resp; a non-nil return is an
// internal failure that aborts the RPC, not a validation finding.
func (a *Server) validateEdge(ctx context.Context, edge *ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode, tenant string, probes *sourceProbes, resp *ingestionv1.ValidatePipelineResponse) error {
	ev := &ingestionv1.EdgeValidation{FromNode: edge.GetFromNode(), ToNode: edge.GetToNode(), Resource: edge.GetResource()}
	resp.Edges = append(resp.Edges, ev)

	from, ok := nodes[edge.GetFromNode()]
	if !ok {
		resp.Errors = append(resp.Errors, graphError(fmt.Sprintf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetFromNode())))
		return nil
	}
	to, ok := nodes[edge.GetToNode()]
	if !ok {
		resp.Errors = append(resp.Errors, graphError(fmt.Sprintf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetToNode())))
		return nil
	}

	srcConn, err := a.loadEdgeConnection(ctx, from, tenant, filament.ConnectorKindSource, "from_node", ev)
	if err != nil {
		return err
	}
	snkConn, err := a.loadEdgeConnection(ctx, to, tenant, filament.ConnectorKindSink, "to_node", ev)
	if err != nil {
		return err
	}
	if srcConn == nil || snkConn == nil {
		return nil
	}

	source, err := a.sources.Resolve(srcConn.Connector)
	if err != nil {
		edgeError(ev, "from_node", err.Error())
		return nil
	}
	sink, err := a.sinks.Resolve(snkConn.Connector)
	if err != nil {
		edgeError(ev, "to_node", err.Error())
		return nil
	}
	srcSpec, snkSpec := source.Spec(), sink.Spec()

	for _, t := range ingestionTypes {
		if filament.ValidateSourceIngestion(srcSpec, t) == nil && filament.ValidateSinkIngestion(snkSpec, t) == nil {
			ev.SupportedIngestionTypes = append(ev.SupportedIngestionTypes, ingestionTypeToProto(t))
		}
	}

	chosen := ingestionTypeFromProto(edge.GetIngestionType()).OrDefault()
	if err := filament.ValidateSourceIngestion(srcSpec, chosen); err != nil {
		edgeError(ev, "ingestion_type", err.Error())
	}
	if err := filament.ValidateSinkIngestion(snkSpec, chosen); err != nil {
		edgeError(ev, "ingestion_type", err.Error())
	}
	if len(ev.Errors) > 0 {
		return nil
	}

	a.edgeRequirements(ctx, edge, from, *srcConn, chosen, probes, ev)
	return nil
}

// loadEdgeConnection loads one node's connection and verifies its kind. A nil
// connection with a nil error means the finding was recorded on ev.
func (a *Server) loadEdgeConnection(ctx context.Context, node *ingestionv1.PipelineNode, tenant string, kind filament.ConnectorKind, field string, ev *ingestionv1.EdgeValidation) (*filament.Connection, error) {
	if node.GetConnectionId() == "" {
		edgeError(ev, field, fmt.Sprintf("node %q has no connection", node.GetId()))
		return nil, nil
	}
	conn, err := a.loadConnectionForTenant(ctx, node.GetConnectionId(), tenant)
	if err != nil {
		if errors.Is(err, filament.ErrNotFound) {
			edgeError(ev, field, fmt.Sprintf("connection %q not found", node.GetConnectionId()))
			return nil, nil
		}
		return nil, err
	}
	if conn.Kind != kind {
		want := "source"
		if kind == filament.ConnectorKindSink {
			want = "sink"
		}
		edgeError(ev, field, fmt.Sprintf("connection %q is not a %s", conn.ID, want))
		return nil, nil
	}
	return &conn, nil
}

// edgeRequirements appends what the edge still needs for the chosen ingestion
// type: a cursor column per routed resource for incremental reads, a primary
// key per resource for keyed writes.
func (a *Server) edgeRequirements(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn filament.Connection, chosen filament.IngestionType, probes *sourceProbes, ev *ingestionv1.EdgeValidation) {
	needsCursor := filament.SourcePolicyForIngestion(chosen).Mode == filament.ModeIncremental
	needsPK := chosen.WriteCapability().RequiresPK
	if !needsCursor && !needsPK {
		return
	}

	cursors := map[string]bool{}
	for _, cursor := range edge.GetCursors() {
		if cursor.GetField() != "" {
			cursors[cursor.GetResource()] = true
		}
	}

	src, err := probes.get(ctx, from, srcConn)
	if err != nil {
		edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
		return
	}

	var resources []string
	enumerated := false
	if edge.GetResource() != "" {
		resources, enumerated = []string{edge.GetResource()}, true
	} else if discoverable, ok := src.(filament.Discoverable); ok {
		result, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
		if err != nil {
			edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
			return
		}
		for _, resource := range result.Resources {
			if resource.Selectable {
				resources = append(resources, resource.Name)
			}
		}
		enumerated = true
	}

	if !enumerated {
		if needsCursor && len(cursors) == 0 {
			ev.Requirements = append(ev.Requirements, &ingestionv1.Requirement{
				Kind:    ingestionv1.RequirementKind_REQUIREMENT_KIND_CURSOR_COLUMN,
				Message: fmt.Sprintf("%s requires a cursor column for each routed resource", chosen),
			})
		}
		return
	}

	for _, resource := range resources {
		if needsCursor && !cursors[resource] {
			ev.Requirements = append(ev.Requirements, cursorRequirement(ctx, src, chosen, resource))
		}
		if needsPK {
			keys, err := filament.PrimaryKeyForResource(ctx, src, resource)
			if err != nil {
				edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
				continue
			}
			if len(keys) == 0 {
				ev.Requirements = append(ev.Requirements, &ingestionv1.Requirement{
					Kind:     ingestionv1.RequirementKind_REQUIREMENT_KIND_PRIMARY_KEY,
					Resource: resource,
					Message:  fmt.Sprintf("%s requires a primary key but none was discovered for resource %q", chosen, resource),
				})
			}
		}
	}
}

// cursorRequirement builds the pick-a-cursor requirement for one resource,
// with eligible columns as candidates when the source can enumerate them.
func cursorRequirement(ctx context.Context, src filament.Source, chosen filament.IngestionType, resource string) *ingestionv1.Requirement {
	requirement := &ingestionv1.Requirement{
		Kind:     ingestionv1.RequirementKind_REQUIREMENT_KIND_CURSOR_COLUMN,
		Resource: resource,
		Message:  fmt.Sprintf("%s requires a cursor column for resource %q", chosen, resource),
	}
	provider, ok := src.(filament.CursorColumnProvider)
	if !ok {
		return requirement
	}
	columns, err := provider.CursorColumns(ctx, resource)
	if err != nil {
		return requirement
	}
	for _, column := range columns {
		if !column.Eligible {
			continue
		}
		requirement.Candidates = append(requirement.Candidates, &ingestionv1.CandidateValue{
			Value:       column.Name,
			Recommended: column.Recommended,
			Rank:        int32(column.Rank), //nolint:gosec // tiny rank
			Warning:     column.Warning,
		})
	}
	return requirement
}

func edgeError(ev *ingestionv1.EdgeValidation, field, message string) {
	ev.Errors = append(ev.Errors, &ingestionv1.ValidationError{Field: field, Message: message})
}

func graphError(message string) *ingestionv1.ValidationError {
	return &ingestionv1.ValidationError{Message: message}
}

// sourceProbes lazily configures at most one live source per node for cursor
// and primary-key inspection, torn down together after validation.
type sourceProbes struct {
	server  *Server
	sources map[string]filament.Source
	errs    map[string]error
}

func (p *sourceProbes) get(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection) (filament.Source, error) {
	id := node.GetId()
	if src, ok := p.sources[id]; ok {
		return src, nil
	}
	if err, ok := p.errs[id]; ok {
		return nil, err
	}
	src, err := p.configure(ctx, node, conn)
	if err != nil {
		p.errs[id] = err
		return nil, err
	}
	p.sources[id] = src
	return src, nil
}

func (p *sourceProbes) configure(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection) (filament.Source, error) {
	config := mergeConfig(conn.Config, structMap(node.GetConfig()))
	if err := p.server.resolveConnectionSecrets(ctx, conn, config); err != nil {
		return nil, err
	}
	source, err := p.server.sources.Resolve(conn.Connector)
	if err != nil {
		return nil, err
	}
	if err := source.Configure(ctx, filament.NewConfig(config)); err != nil {
		return nil, err
	}
	return source, nil
}

func (p *sourceProbes) teardown(ctx context.Context) {
	for _, src := range p.sources {
		_ = src.Teardown(ctx)
	}
}
