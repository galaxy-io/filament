package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/compile"
)

// ValidatePipeline checks every edge of a graph: the per-resource read modes
// and route-wide write modes the connector pair supports, their selected
// combination, and any cursor or primary-key requirements.
func (a *Server) ValidatePipeline(ctx context.Context, req *connect.Request[ingestionv1.ValidatePipelineRequest]) (*connect.Response[ingestionv1.ValidatePipelineResponse], error) {
	tenant, err := tenantForRequest(ctx, req.Msg.GetTenantId())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	graph := req.Msg.GetGraph()
	resp, err := a.validatePipelineGraph(ctx, tenant, graph.GetNodes(), graph.GetEdges())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// validatePipelineGraph is the shared validation path for the public probe and
// persisted pipeline versions. Callers own the context deadline.
func (a *Server) validatePipelineGraph(ctx context.Context, tenant string, graphNodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) (*ingestionv1.ValidatePipelineResponse, error) {
	nodes := make(map[string]*ingestionv1.PipelineNode, len(graphNodes))
	for _, node := range graphNodes {
		nodes[node.GetId()] = node
	}

	resp := &ingestionv1.ValidatePipelineResponse{}
	probes := &sourceProbes{
		server:     a,
		sources:    map[string]filament.Source{},
		errs:       map[string]error{},
		discovered: map[string][]filament.Resource{},
	}
	defer probes.teardown(ctx)

	routeWriteModes := map[string]filament.WriteMode{}
	for _, edge := range edges {
		if err := a.validateEdge(ctx, edge, nodes, tenant, probes, resp); err != nil {
			return nil, err
		}
		ev := resp.Edges[len(resp.Edges)-1]
		writeMode, err := writeModeFromProto(ev.GetEffectiveWriteMode())
		if err != nil {
			continue
		}
		route := edge.GetFromNode() + "\x00" + edge.GetToNode()
		if previous, ok := routeWriteModes[route]; ok && previous != writeMode {
			edgeError(resp.Edges[len(resp.Edges)-1], "write_mode", "all resources on a source-to-destination route must use the same write mode")
		} else {
			routeWriteModes[route] = writeMode
		}
	}

	resp.Valid = len(resp.Errors) == 0
	for _, ev := range resp.Edges {
		if len(ev.Errors) > 0 {
			resp.Valid = false
		}
		for _, requirement := range ev.Requirements {
			if requirement.GetBlocking() {
				resp.Valid = false
			}
		}
		for _, resource := range ev.Resources {
			for _, requirement := range resource.Requirements {
				if requirement.GetBlocking() {
					resp.Valid = false
				}
			}
		}
	}
	return resp, nil
}

func pipelineValidationMessage(resp *ingestionv1.ValidatePipelineResponse) string {
	for _, validationErr := range resp.GetErrors() {
		if validationErr.GetMessage() != "" {
			return validationErr.GetMessage()
		}
	}
	for _, edge := range resp.GetEdges() {
		for _, validationErr := range edge.GetErrors() {
			if validationErr.GetMessage() != "" {
				return validationErr.GetMessage()
			}
		}
		for _, requirement := range edge.GetRequirements() {
			if requirement.GetBlocking() && requirement.GetMessage() != "" {
				return requirement.GetMessage()
			}
		}
		for _, resource := range edge.GetResources() {
			for _, requirement := range resource.GetRequirements() {
				if requirement.GetBlocking() && requirement.GetMessage() != "" {
					return requirement.GetMessage()
				}
			}
		}
	}
	return "pipeline graph is invalid"
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
	replication := filament.ReplicationOf(source, filament.NewConfig(srcConn.Config))
	ev.Replication = replicationToProto(replication)

	// CDC connections fix the read side to the change stream and expose append
	// (the history-preserving default) or merge. Standard edges expose both.
	var chosen filament.IngestionType
	var supportedReadModes []ingestionv1.ReadMode
	if replication == filament.ReplicationCDC {
		if edge.GetReadMode() != ingestionv1.ReadMode_READ_MODE_UNSPECIFIED {
			edgeError(ev, "read_mode", "CDC connections do not accept a read mode")
		}
		writeMode := filament.WriteAppend
		switch edge.GetWriteMode() {
		case ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED, ingestionv1.WriteMode_WRITE_MODE_APPEND:
			chosen = filament.IngestionCDCAppend
		case ingestionv1.WriteMode_WRITE_MODE_MERGE:
			writeMode = filament.WriteMerge
			chosen = filament.IngestionCDCMerge
		default:
			edgeError(ev, "write_mode", "CDC connections support append or merge write mode")
			chosen = filament.IngestionCDCAppend
		}
		ev.EffectiveWriteMode = writeModeToProto(writeMode)
		ev.SupportedWriteModes = supportedCDCWriteModesFor(snkSpec)
		if len(edge.GetCursors()) > 0 {
			edgeError(ev, "cursors", "CDC connections manage their stream position automatically")
		}
	} else {
		readMode, err := readModeFromProto(edge.GetReadMode())
		if err != nil {
			edgeError(ev, "read_mode", err.Error())
			return nil
		}
		writeMode, err := writeModeFromProto(edge.GetWriteMode())
		if err != nil {
			edgeError(ev, "write_mode", err.Error())
			return nil
		}
		ev.EffectiveReadMode = readModeToProto(readMode)
		ev.EffectiveWriteMode = writeModeToProto(writeMode)
		supportedReadModes = supportedReadModesFor(srcSpec)
		ev.SupportedWriteModes = supportedWriteModesFor(snkSpec)
		chosen, err = filament.IngestionFor(readMode, writeMode)
		if err != nil {
			edgeError(ev, "write_mode", err.Error())
			return nil
		}
	}

	if err := filament.ValidateSourceIngestion(srcSpec, chosen); err != nil {
		edgeError(ev, "read_mode", err.Error())
	}
	if err := filament.ValidateSinkIngestion(snkSpec, chosen); err != nil {
		edgeError(ev, "write_mode", err.Error())
	}
	if len(ev.Errors) > 0 && replication == filament.ReplicationCDC {
		return nil
	}

	a.resourceBreakdown(ctx, edge, from, *srcConn, chosen, supportedReadModes, probes, ev)
	return nil
}

func supportedCDCWriteModesFor(sink filament.SinkSpec) []ingestionv1.WriteMode {
	var out []ingestionv1.WriteMode
	for _, candidate := range []struct {
		mode      filament.WriteMode
		ingestion filament.IngestionType
	}{
		{filament.WriteAppend, filament.IngestionCDCAppend},
		{filament.WriteMerge, filament.IngestionCDCMerge},
	} {
		if filament.ValidateSinkIngestion(sink, candidate.ingestion) == nil {
			out = append(out, writeModeToProto(candidate.mode))
		}
	}
	return out
}

func supportedReadModesFor(source filament.ConnectorSpec) []ingestionv1.ReadMode {
	var out []ingestionv1.ReadMode
	for _, read := range []filament.ReadMode{filament.ModeFull, filament.ModeIncremental} {
		ingestionType, _ := filament.IngestionFor(read, filament.WriteAppend)
		if filament.ValidateSourceIngestion(source, ingestionType) == nil {
			out = append(out, readModeToProto(read))
		}
	}
	return out
}

func supportedWriteModesFor(sink filament.SinkSpec) []ingestionv1.WriteMode {
	var out []ingestionv1.WriteMode
	for _, write := range []filament.WriteMode{filament.WriteAppend, filament.WriteReplace, filament.WriteUpsert} {
		ingestionType, _ := filament.IngestionFor(filament.ModeFull, write)
		if filament.ValidateSinkIngestion(sink, ingestionType) == nil {
			out = append(out, writeModeToProto(write))
		}
	}
	return out
}

// loadEdgeConnection loads one node's connection and verifies its kind. A nil
// connection with a nil error means the finding was recorded on ev.
func (a *Server) loadEdgeConnection(ctx context.Context, node *ingestionv1.PipelineNode, _ string, kind filament.ConnectorKind, field string, ev *ingestionv1.EdgeValidation) (*filament.Connection, error) {
	if node.GetConnectionId() == "" {
		edgeError(ev, field, fmt.Sprintf("node %q has no connection", node.GetId()))
		return nil, nil
	}
	conn, err := a.store.LoadConnection(ctx, node.GetConnectionId())
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

// resourceBreakdown narrows read modes using each table's cursor reality, then
// reports requirements for the selected read/write combination.
func (a *Server) resourceBreakdown(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn filament.Connection, chosen filament.IngestionType, supportedReadModes []ingestionv1.ReadMode, probes *sourceProbes, ev *ingestionv1.EdgeValidation) {
	needsCursor := filament.SourcePolicyForIngestion(chosen).Mode == filament.ModeIncremental
	needsPK := chosen.WriteCapability().RequiresPK

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
	if edge.GetResource() != "" {
		resources = []string{edge.GetResource()}
	} else {
		discovered, ok, err := probes.discover(ctx, from.GetId(), src)
		if err != nil {
			edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
			return
		}
		if !ok {
			if needsCursor {
				ev.Requirements = append(ev.Requirements, &ingestionv1.Requirement{
					Kind:            ingestionv1.RequirementKind_REQUIREMENT_KIND_CURSOR_COLUMN,
					Satisfied:       len(cursors) > 0,
					CandidateStatus: ingestionv1.CandidateStatus_CANDIDATE_STATUS_UNAVAILABLE,
					Message:         "incremental reads need a cursor column for each resource, but the source cannot list its resources",
				})
			}
			return
		}
		for _, resource := range discovered {
			if resource.Selectable {
				resources = append(resources, resource.Name)
			}
		}
	}

	for _, resource := range resources {
		rv := &ingestionv1.ResourceValidation{Resource: resource}
		ev.Resources = append(ev.Resources, rv)

		keys, keyErr := filament.PrimaryKeyForResource(ctx, src, resource)
		if keyErr != nil && needsPK {
			edgeError(ev, "from_node", fmt.Sprintf("could not inspect primary key for %q: %v", resource, keyErr))
		}
		candidates, status := cursorCandidates(ctx, src, resource)
		// When candidates are unknowable stay optimistic; runtime decides.
		cursorable := cursors[resource] || len(candidates) > 0 ||
			status != ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED
		for _, mode := range supportedReadModes {
			if mode == ingestionv1.ReadMode_READ_MODE_INCREMENTAL && !cursorable {
				continue
			}
			rv.SupportedReadModes = append(rv.SupportedReadModes, mode)
		}
		if needsCursor {
			rv.Requirements = append(rv.Requirements, cursorRequirement(resource, candidates, status, cursors[resource]))
		}
		if needsPK && keyErr == nil && len(keys) == 0 {
			rv.Requirements = append(rv.Requirements, &ingestionv1.Requirement{
				Kind:            ingestionv1.RequirementKind_REQUIREMENT_KIND_PRIMARY_KEY,
				Resource:        resource,
				Blocking:        true,
				CandidateStatus: ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED,
				Message:         fmt.Sprintf("writing by key requires a primary key, but none was discovered for resource %q", resource),
			})
		}
	}
}

// cursorCandidates enumerates a resource's eligible cursor columns, reporting
// how an empty list should be read.
func cursorCandidates(ctx context.Context, src filament.Source, resource string) ([]*ingestionv1.CandidateValue, ingestionv1.CandidateStatus) {
	provider, ok := src.(filament.CursorColumnProvider)
	if !ok {
		return nil, ingestionv1.CandidateStatus_CANDIDATE_STATUS_NOT_SUPPORTED
	}
	columns, err := provider.CursorColumns(ctx, resource)
	if err != nil {
		return nil, ingestionv1.CandidateStatus_CANDIDATE_STATUS_UNAVAILABLE
	}
	var out []*ingestionv1.CandidateValue
	for _, column := range columns {
		if !column.Eligible {
			continue
		}
		out = append(out, &ingestionv1.CandidateValue{
			Value:       column.Name,
			Recommended: column.Recommended,
			Rank:        int32(column.Rank), //nolint:gosec // tiny rank
			Warning:     column.Warning,
		})
	}
	return out, ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED
}

// cursorRequirement mirrors runtime cursor resolution: configured wins, an
// auto-detectable (recommended) candidate keeps the run viable without
// configuration, and only a resource with neither blocks.
func cursorRequirement(resource string, candidates []*ingestionv1.CandidateValue, status ingestionv1.CandidateStatus, satisfied bool) *ingestionv1.Requirement {
	requirement := &ingestionv1.Requirement{
		Kind:            ingestionv1.RequirementKind_REQUIREMENT_KIND_CURSOR_COLUMN,
		Resource:        resource,
		Candidates:      candidates,
		CandidateStatus: status,
		Satisfied:       satisfied,
	}
	recommended := ""
	for _, candidate := range candidates {
		if candidate.GetRecommended() {
			recommended = candidate.GetValue()
			break
		}
	}
	switch {
	case satisfied:
		requirement.Message = fmt.Sprintf("cursor column configured for resource %q", resource)
	case status != ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED:
		requirement.Message = fmt.Sprintf("incremental reads need a cursor column, but candidates for resource %q could not be determined", resource)
	case recommended != "":
		requirement.Message = fmt.Sprintf("auto-detected cursor %q will be used for resource %q", recommended, resource)
	case len(candidates) > 0:
		requirement.Blocking = true
		requirement.Message = fmt.Sprintf("incremental reads require a cursor column for resource %q", resource)
	default:
		requirement.Blocking = true
		requirement.Message = fmt.Sprintf("resource %q has no usable cursor column; use full replication for it instead", resource)
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
// and primary-key inspection, memoizes discovery, and tears everything down
// together after validation.
type sourceProbes struct {
	server     *Server
	sources    map[string]filament.Source
	errs       map[string]error
	discovered map[string][]filament.Resource
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

// discover lists a node's resources once; ok is false when the source does
// not support discovery.
func (p *sourceProbes) discover(ctx context.Context, nodeID string, src filament.Source) ([]filament.Resource, bool, error) {
	discoverable, ok := src.(filament.Discoverable)
	if !ok {
		return nil, false, nil
	}
	if resources, ok := p.discovered[nodeID]; ok {
		return resources, true, nil
	}
	result, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		return nil, true, err
	}
	p.discovered[nodeID] = result.Resources
	return result.Resources, true, nil
}

func (p *sourceProbes) configure(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection) (filament.Source, error) {
	config := compile.MergeConfig(conn.Config, structMap(node.GetConfig()))
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
