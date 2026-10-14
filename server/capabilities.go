package server

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/compile"
	"github.com/galaxy-io/filament/internal/convert"
	"github.com/galaxy-io/filament/transform"
)

// ValidatePipeline checks every edge of a graph: the per-resource read modes
// and route-wide write modes the connector pair supports, their selected
// combination, and any cursor or primary-key requirements.
func (a *Server) ValidatePipeline(ctx context.Context, req *connect.Request[ingestionv1.ValidatePipelineRequest]) (*connect.Response[ingestionv1.ValidatePipelineResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.validateExecution(req.Msg.GetExecutionMode()); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	graph := req.Msg.GetGraph()
	resp, err := a.validatePipelineGraph(ctx, string(tenant), graph.GetNodes(), graph.GetEdges(), req.Msg.GetExecutionMode())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(resp), nil
}

// validatePipelineGraph is the shared validation path for the public probe and
// persisted pipeline versions. Callers own the context deadline.
func (a *Server) validatePipelineGraph(ctx context.Context, tenant string, graphNodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge, execution ...ingestionv1.ExecutionMode) (*ingestionv1.ValidatePipelineResponse, error) {
	mode := ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED
	if len(execution) > 0 && execution[0] != ingestionv1.ExecutionMode_EXECUTION_MODE_UNSPECIFIED {
		mode = execution[0]
	}
	nodes := make(map[string]*ingestionv1.PipelineNode, len(graphNodes))
	for _, node := range graphNodes {
		nodes[node.GetId()] = node
	}

	resp := &ingestionv1.ValidatePipelineResponse{}
	if !hasNodeOfKind(graphNodes, ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE) ||
		!hasNodeOfKind(graphNodes, ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK) ||
		len(edges) == 0 {
		resp.Errors = append(resp.Errors, graphError("Add a source, a sink, and at least one resource"))
		return resp, nil
	}
	probes := newNodeInspections(a, edges)

	if err := compile.ValidateDestinations(edges); err != nil {
		resp.Errors = append(resp.Errors, graphError(err.Error()))
	}
	for _, node := range graphNodes {
		if err := a.validateNodeConfig(ctx, tenant, node, resp); err != nil {
			return nil, err
		}
	}
	routeWriteModes := map[string]filament.WriteMode{}
	routeTransforms := map[string]map[string]bool{}
	for _, edge := range edges {
		if err := a.validateEdge(ctx, edge, nodes, tenant, probes, resp, mode); err != nil {
			return nil, err
		}
		ev := resp.Edges[len(resp.Edges)-1]
		route := edge.GetFromNode() + "\x00" + edge.GetToNode()
		// Run compile merges a route's transforms and refuses a resource named
		// twice; catch it here so the version is not saved only to fail at run.
		if compile.HasTransform(edge) {
			if resources, err := compile.TransformResources(edge); err == nil {
				seen := routeTransforms[route]
				if seen == nil {
					seen = map[string]bool{}
					routeTransforms[route] = seen
				}
				for _, resource := range slices.Sorted(maps.Keys(resources)) {
					if seen[resource] {
						edgeError(ev, "transform", fmt.Sprintf("resource %q is transformed by more than one edge", resource))
					}
					seen[resource] = true
				}
			}
		}
		writeMode, err := convert.WriteModeFromProto(ev.GetEffectiveWriteMode())
		if err != nil {
			continue
		}
		if previous, ok := routeWriteModes[route]; ok && previous != writeMode {
			edgeError(ev, "write_mode", "all resources on a source-to-destination route must use the same write mode")
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
func (a *Server) validateEdge(ctx context.Context, edge *ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode, tenant string, probes *nodeInspections, resp *ingestionv1.ValidatePipelineResponse, mode ingestionv1.ExecutionMode) error {
	ev := &ingestionv1.EdgeValidation{FromNode: edge.GetFromNode(), ToNode: edge.GetToNode(), Resource: edge.GetResource()}
	resp.Edges = append(resp.Edges, ev)

	from, to := edgeNodes(edge, nodes, resp)
	if from == nil || to == nil {
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

	srcSpec, err := a.worker.SourceSpec(ctx, srcConn.Connector)
	if err != nil {
		edgeError(ev, "from_node", err.Error())
		return nil
	}
	snkSpec, err := a.worker.SinkSpec(ctx, snkConn.Connector)
	if err != nil {
		edgeError(ev, "to_node", err.Error())
		return nil
	}
	runtimeSupported := a.edgeExecutionModes(srcSpec, snkSpec, mode, ev)
	if mode == ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
		a.validateContinuousEdge(ctx, edge, from, srcConn, srcSpec, snkSpec, runtimeSupported, probes, ev)
		return nil
	}
	replication := filament.ReplicationFor(srcSpec, filament.NewConfig(srcConn.Config))
	ev.Replication = convert.ReplicationToProto(replication)

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
		ev.EffectiveWriteMode = convert.WriteModeToProto(writeMode)
		ev.SupportedWriteModes = supportedCDCWriteModesFor(snkSpec)
		if len(edge.GetCursors()) > 0 {
			edgeError(ev, "cursors", "CDC connections manage their stream position automatically")
		}
	} else {
		readMode, err := convert.ReadModeFromProto(edge.GetReadMode())
		if err != nil {
			edgeError(ev, "read_mode", err.Error())
			return nil
		}
		writeMode, err := convert.WriteModeFromProto(edge.GetWriteMode())
		if err != nil {
			edgeError(ev, "write_mode", err.Error())
			return nil
		}
		ev.EffectiveReadMode = convert.ReadModeToProto(readMode)
		ev.EffectiveWriteMode = convert.WriteModeToProto(writeMode)
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

	a.resourceBreakdown(ctx, edge, from, *srcConn, snkSpec, chosen, supportedReadModes, probes, ev)
	validateEdgeTransform(ctx, edge, from, *srcConn, probes, ev)
	return nil
}

// validateEdgeTransform compiles the edge's transform against the schema of
// every resource it names, so a definition the source no longer satisfies
// fails validation rather than the run. Issues carry the definition path the
// builder uses to place them.
func validateEdgeTransform(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn filament.Connection, probes *nodeInspections, ev *ingestionv1.EdgeValidation) {
	if !compile.HasTransform(edge) {
		return
	}
	raw, err := edge.GetTransform().MarshalJSON()
	if err != nil {
		edgeError(ev, "transform", err.Error())
		return
	}
	def, err := transform.Parse(raw)
	if err != nil {
		transformErrors(ev, err)
		return
	}
	if own := edge.GetResource(); own != "" {
		for resource := range def.Resources {
			if resource != own {
				edgeError(ev, "transform", fmt.Sprintf("edge for %q defines resource %q", own, resource))
				return
			}
		}
	}
	for _, resource := range slices.Sorted(maps.Keys(def.Resources)) {
		inspection, err := probes.resource(ctx, from, srcConn, resource)
		if err != nil {
			edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
			return
		}
		switch {
		case inspection.Status == filament.InspectFailed:
			edgeError(ev, "transform", inspection.Message)
			continue
		case inspection.Schema == nil:
			edgeError(ev, "transform", fmt.Sprintf("connector %q does not provide resource schemas", srcConn.Connector))
			return
		}
		if _, _, err := transform.Analyze(def, *inspection.Schema); err != nil {
			transformErrors(ev, err)
		}
	}
}

// transformErrors records each path-addressed transform issue on the edge.
func transformErrors(ev *ingestionv1.EdgeValidation, err error) {
	var errs *transform.Errors
	if !errors.As(err, &errs) {
		edgeError(ev, "transform", err.Error())
		return
	}
	for _, issue := range errs.Issues {
		edgeError(ev, issue.Path, issue.Message)
	}
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
			out = append(out, convert.WriteModeToProto(candidate.mode))
		}
	}
	return out
}

func supportedReadModesFor(source filament.ConnectorSpec) []ingestionv1.ReadMode {
	var out []ingestionv1.ReadMode
	for _, read := range []filament.ReadMode{filament.ModeFull, filament.ModeIncremental} {
		ingestionType, _ := filament.IngestionFor(read, filament.WriteAppend)
		if filament.ValidateSourceIngestion(source, ingestionType) == nil {
			out = append(out, convert.ReadModeToProto(read))
		}
	}
	return out
}

func supportedWriteModesFor(sink filament.SinkSpec) []ingestionv1.WriteMode {
	var out []ingestionv1.WriteMode
	for _, write := range []filament.WriteMode{filament.WriteAppend, filament.WriteReplace, filament.WriteUpsert} {
		ingestionType, _ := filament.IngestionFor(filament.ModeFull, write)
		if filament.ValidateSinkIngestion(sink, ingestionType) == nil {
			out = append(out, convert.WriteModeToProto(write))
		}
	}
	return out
}

// loadEdgeConnection loads one node's connection and verifies its kind. A nil
// connection with a nil error means the finding was recorded on ev.
func (a *Server) loadEdgeConnection(ctx context.Context, node *ingestionv1.PipelineNode, tenant string, kind filament.ConnectorKind, field string, ev *ingestionv1.EdgeValidation) (*filament.Connection, error) {
	if node.GetConnectionId() == "" {
		edgeError(ev, field, fmt.Sprintf("node %q has no connection", node.GetId()))
		return nil, nil
	}
	conn, err := a.store.LoadConnection(ctx, filament.TenantID(tenant), node.GetConnectionId())
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
func (a *Server) resourceBreakdown(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn filament.Connection, snkSpec filament.SinkSpec, chosen filament.IngestionType, supportedReadModes []ingestionv1.ReadMode, probes *nodeInspections, ev *ingestionv1.EdgeValidation) {
	needsCursor := filament.SourcePolicyForIngestion(chosen).Mode == filament.ModeIncremental
	needsPK := chosen.WriteCapability().RequiresPK

	cursors := map[string]bool{}
	for _, cursor := range edge.GetCursors() {
		if cursor.GetField() != "" {
			cursors[cursor.GetResource()] = true
		}
	}

	var inspections []filament.Inspection
	var err error
	if edge.GetResource() != "" {
		var inspection filament.Inspection
		inspection, err = probes.resource(ctx, from, srcConn, edge.GetResource())
		inspections = []filament.Inspection{inspection}
	} else {
		inspections, err = probes.all(ctx, from, srcConn)
	}
	if err != nil {
		if errors.Is(err, filament.ErrUnsupported) && edge.GetResource() == "" {
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
		edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
		return
	}

	for _, inspection := range inspections {
		resource := inspection.Name
		rv := &ingestionv1.ResourceValidation{Resource: resource}
		ev.Resources = append(ev.Resources, rv)

		keys := inspection.PrimaryKey
		var keyErr error
		if inspection.Status == filament.InspectFailed {
			keyErr = errors.New(inspection.Message)
		}
		if keyErr != nil && needsPK {
			edgeError(ev, "from_node", fmt.Sprintf("could not inspect primary key for %q: %v", resource, keyErr))
		}
		candidates, status := cursorCandidates(inspection)
		managed := inspection.ManagedIncremental
		// When candidates are unknowable stay optimistic; runtime decides.
		cursorable := managed || cursors[resource] || len(candidates) > 0 ||
			status != ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED
		for _, mode := range supportedReadModes {
			if mode == ingestionv1.ReadMode_READ_MODE_INCREMENTAL && !cursorable {
				continue
			}
			rv.SupportedReadModes = append(rv.SupportedReadModes, mode)
		}
		if needsCursor {
			requirement := cursorRequirement(resource, candidates, status, managed || cursors[resource])
			if managed {
				requirement.Message = fmt.Sprintf("incremental state is managed by the source for resource %q", resource)
			}
			rv.Requirements = append(rv.Requirements, requirement)
		}
		if needsPK && keyErr == nil && len(keys) == 0 {
			effective := filament.IngestionForKeys(chosen, keys)
			if effective != chosen {
				if err := filament.ValidateSinkIngestion(snkSpec, effective); err != nil {
					edgeError(ev, "write_mode", fmt.Sprintf("append fallback for resource %q: %v", resource, err))
				} else {
					rv.Requirements = append(rv.Requirements, &ingestionv1.Requirement{
						Kind:            ingestionv1.RequirementKind_REQUIREMENT_KIND_PRIMARY_KEY,
						Resource:        resource,
						Satisfied:       true,
						CandidateStatus: ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED,
						Message:         fmt.Sprintf("resource %q has no primary key; upsert will use append, retaining repeated rows", resource),
					})
				}
				continue
			}
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
func cursorCandidates(inspection filament.Inspection) ([]*ingestionv1.CandidateValue, ingestionv1.CandidateStatus) {
	if inspection.Status == filament.InspectFailed {
		return nil, ingestionv1.CandidateStatus_CANDIDATE_STATUS_UNAVAILABLE
	}
	if !inspection.Ranked {
		return nil, ingestionv1.CandidateStatus_CANDIDATE_STATUS_NOT_SUPPORTED
	}
	var out []*ingestionv1.CandidateValue
	for _, column := range inspection.Columns {
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

// validateNodeConfig checks the pipeline-scoped fields of a node's effective
// config: the connection's settings overlaid with the node's own overrides.
// Missing or mismatched connections are reported by edge validation instead.
func (a *Server) validateNodeConfig(ctx context.Context, tenant string, node *ingestionv1.PipelineNode, resp *ingestionv1.ValidatePipelineResponse) error {
	if node.GetConnectionId() == "" {
		return nil
	}
	conn, err := a.store.LoadConnection(ctx, filament.TenantID(tenant), node.GetConnectionId())
	if err != nil {
		if errors.Is(err, filament.ErrNotFound) {
			return nil
		}
		return err
	}
	var schema filament.ConfigSchema
	switch conn.Kind {
	case filament.ConnectorKindSource:
		spec, err := a.worker.SourceSpec(ctx, conn.Connector)
		if err != nil {
			return nil
		}
		schema = spec.Config
	case filament.ConnectorKindSink:
		spec, err := a.worker.SinkSpec(ctx, conn.Connector)
		if err != nil {
			return nil
		}
		schema = spec.Config
	default:
		return nil
	}
	cfg := filament.NewConfig(overlayConfig(conn.Config, convert.StructMap(node.GetConfig())))
	if err := validateConfigScope(schema, cfg, filament.ScopePipeline); err != nil {
		validationErr := &ingestionv1.ValidationError{Message: fmt.Sprintf("node %q: %s", node.GetId(), err.Error())}
		if fieldErr, ok := err.(*configValidationError); ok {
			validationErr.Field = fieldErr.Field
		}
		resp.Errors = append(resp.Errors, validationErr)
	}
	return nil
}

func edgeError(ev *ingestionv1.EdgeValidation, field, message string) {
	ev.Errors = append(ev.Errors, &ingestionv1.ValidationError{Field: field, Message: message})
}

func hasNodeOfKind(nodes []*ingestionv1.PipelineNode, kind ingestionv1.ConnectorKind) bool {
	for _, node := range nodes {
		if node.GetKind() == kind {
			return true
		}
	}
	return false
}

func graphError(message string) *ingestionv1.ValidationError {
	return &ingestionv1.ValidationError{Message: message}
}

// nodeInspections inspects each source node at most once. The edges name the
// resources every node needs up front, so one Inspect call covers them all;
// a wildcard edge asks for every selectable resource instead.
type nodeInspections struct {
	server   *Server
	wanted   map[string][]string
	wildcard map[string]bool
	loaded   map[string]bool
	byName   map[string]map[string]filament.Inspection
	every    map[string][]filament.Inspection
	errs     map[string]error
}

func newNodeInspections(a *Server, edges []*ingestionv1.PipelineEdge) *nodeInspections {
	p := &nodeInspections{
		server:   a,
		wanted:   map[string][]string{},
		wildcard: map[string]bool{},
		loaded:   map[string]bool{},
		byName:   map[string]map[string]filament.Inspection{},
		every:    map[string][]filament.Inspection{},
		errs:     map[string]error{},
	}
	for _, edge := range edges {
		node := edge.GetFromNode()
		if edge.GetResource() == "" {
			p.wildcard[node] = true
			continue
		}
		if !slices.Contains(p.wanted[node], edge.GetResource()) {
			p.wanted[node] = append(p.wanted[node], edge.GetResource())
		}
	}
	return p
}

// load runs the node's one Inspect call: every selectable resource for a
// wildcard node, otherwise exactly the resources its edges name.
func (p *nodeInspections) load(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection) error {
	id := node.GetId()
	if p.loaded[id] {
		return p.errs[id]
	}
	p.loaded[id] = true
	var resources []string
	if !p.wildcard[id] {
		resources = p.wanted[id]
	}
	inspections, err := p.inspect(ctx, node, conn, resources)
	if err != nil {
		p.errs[id] = err
		return err
	}
	p.every[id] = inspections
	p.byName[id] = map[string]filament.Inspection{}
	for _, inspection := range inspections {
		p.byName[id][inspection.Name] = inspection
	}
	return nil
}

// all returns every selectable resource's inspection for a wildcard node.
func (p *nodeInspections) all(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection) ([]filament.Inspection, error) {
	if err := p.load(ctx, node, conn); err != nil {
		return nil, err
	}
	return p.every[node.GetId()], nil
}

// resource returns one resource's inspection, fetching it on its own when the
// node's load did not cover it.
func (p *nodeInspections) resource(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection, name string) (filament.Inspection, error) {
	if err := p.load(ctx, node, conn); err != nil {
		return filament.Inspection{}, err
	}
	id := node.GetId()
	if inspection, ok := p.byName[id][name]; ok {
		return inspection, nil
	}
	inspections, err := p.inspect(ctx, node, conn, []string{name})
	if err != nil {
		return filament.Inspection{}, err
	}
	for _, inspection := range inspections {
		p.byName[id][inspection.Name] = inspection
	}
	inspection, ok := p.byName[id][name]
	if !ok {
		return filament.Inspection{}, fmt.Errorf("resource %q was not inspected", name)
	}
	return inspection, nil
}

func (p *nodeInspections) inspect(ctx context.Context, node *ingestionv1.PipelineNode, conn filament.Connection, resources []string) ([]filament.Inspection, error) {
	config := compile.MergeConfig(conn.Config, convert.StructMap(node.GetConfig()))
	if err := p.server.resolveConnectionSecrets(ctx, conn, config); err != nil {
		return nil, err
	}
	return p.server.worker.Inspect(ctx, conn.Connector, filament.NewConfig(config), resources)
}

func (a *Server) validateContinuousEdge(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn *filament.Connection, srcSpec filament.ConnectorSpec, snkSpec filament.SinkSpec, runtimeSupported bool, probes *nodeInspections, ev *ingestionv1.EdgeValidation) {
	selected := edge.GetWriteMode()
	if selected == ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED {
		selected = ingestionv1.WriteMode_WRITE_MODE_APPEND
	}
	ev.EffectiveWriteMode = selected
	if caps := snkSpec.Capabilities.Stream; caps != nil {
		seen := map[filament.WriteMode]bool{}
		for _, candidate := range caps.WritePolicies {
			if _, err := filament.PlanContinuousWrite(srcSpec, snkSpec, candidate.Mode); err == nil && !seen[candidate.Mode] && convert.WriteModeToProto(candidate.Mode) != ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED {
				ev.SupportedWriteModes = append(ev.SupportedWriteModes, convert.WriteModeToProto(candidate.Mode))
				seen[candidate.Mode] = true
			}
		}
	}
	if !runtimeSupported {
		edgeError(ev, "execution_mode", filament.ErrContinuousDisabled.Error())
	}
	if err := filament.ValidateContinuous(srcSpec, snkSpec); err != nil {
		edgeError(ev, "execution_mode", err.Error())
	}
	writeMode, err := convert.WriteModeFromProto(selected)
	var writePlan filament.ContinuousWritePlan
	if err == nil {
		writePlan, err = filament.PlanContinuousWrite(srcSpec, snkSpec, writeMode)
	}
	if err != nil {
		edgeError(ev, "write_mode", err.Error())
	}
	if edge.GetReadMode() != ingestionv1.ReadMode_READ_MODE_UNSPECIFIED || len(edge.GetCursors()) > 0 {
		edgeError(ev, "read_mode", "continuous execution does not accept bounded read modes or cursors")
	}
	if edge.GetSelector() != "" && edge.GetSelector() != edge.GetResource() {
		edgeError(ev, "resource", "continuous execution requires fixed resources, not selectors")
	}
	if compile.HasTransform(edge) {
		edgeError(ev, "transform", "continuous execution does not apply transforms")
	}
	var resources []string
	if edge.Resource != "" {
		resources = []string{edge.Resource}
	}
	ref := filament.Ref{Connector: srcConn.Connector, Config: compile.MergeConfig(srcConn.Config, convert.StructMap(from.Config))}
	sourcePlan, planErr := compile.PlanContinuousSource(ctx, a.worker, ref, resources, srcConn.ID)
	if planErr != nil {
		edgeError(ev, "from_node", planErr.Error())
	}
	if err == nil && planErr == nil && writePlan.Policy.Capability.RequiresPK {
		for _, resource := range sourcePlan.Resources {
			inspection, probeErr := probes.resource(ctx, from, *srcConn, resource)
			if probeErr != nil {
				edgeError(ev, "from_node", probeErr.Error())
				break
			}
			if inspection.Status == filament.InspectFailed {
				edgeError(ev, "from_node", inspection.Message)
				continue
			}
			keys := inspection.PrimaryKey
			ev.Requirements = append(ev.Requirements, &ingestionv1.Requirement{Kind: ingestionv1.RequirementKind_REQUIREMENT_KIND_PRIMARY_KEY, Resource: resource, Satisfied: len(keys) > 0, Blocking: len(keys) == 0, Message: "streaming write policy requires a primary key"})
		}
	}
}

func (a *Server) edgeExecutionModes(srcSpec filament.ConnectorSpec, snkSpec filament.SinkSpec, mode ingestionv1.ExecutionMode, ev *ingestionv1.EdgeValidation) bool {
	ev.EffectiveExecutionMode = mode
	for _, candidate := range a.sourceExecutionModes(srcSpec) {
		if !slices.Contains(a.sinkExecutionModes(snkSpec), candidate) {
			continue
		}
		if candidate == ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED && !boundedPairSupported(srcSpec, snkSpec) {
			continue
		}
		if candidate == ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS && filament.ValidateContinuous(srcSpec, snkSpec) != nil {
			continue
		}
		ev.SupportedExecutionModes = append(ev.SupportedExecutionModes, candidate)
	}
	return a.continuousSupported()
}

func edgeNodes(edge *ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode, resp *ingestionv1.ValidatePipelineResponse) (*ingestionv1.PipelineNode, *ingestionv1.PipelineNode) {
	from, ok := nodes[edge.GetFromNode()]
	if !ok {
		resp.Errors = append(resp.Errors, graphError(fmt.Sprintf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetFromNode())))
		return nil, nil
	}
	to, ok := nodes[edge.GetToNode()]
	if !ok {
		resp.Errors = append(resp.Errors, graphError(fmt.Sprintf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetToNode())))
		return nil, nil
	}

	return from, to
}

// Advertise bounded execution only when at least one declared ingestion policy
// is shared. Resource-specific keys and cursors are checked by graph validation.
func boundedPairSupported(source filament.ConnectorSpec, sink filament.SinkSpec) bool {
	for _, ingestion := range []filament.IngestionType{filament.IngestionFullReplace, filament.IngestionFullAppend, filament.IngestionFullUpsert, filament.IngestionIncrementalAppend, filament.IngestionIncrementalUpsert, filament.IngestionIncrementalDelete, filament.IngestionCDCAppend, filament.IngestionCDCMerge} {
		if filament.ValidateSourceIngestion(source, ingestion) == nil && filament.ValidateSinkIngestion(sink, ingestion) == nil {
			return true
		}
	}
	return false
}
