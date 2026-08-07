package server

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ingestionTypes is the selectable menu validation walks, in display order.
// CDC is deliberately absent: it is a property of the connection, never a
// per-edge choice.
var ingestionTypes = []filament.IngestionType{
	filament.IngestionSnapshotReplace,
	filament.IngestionSnapshotUpsert,
	filament.IngestionSnapshotAppend,
	filament.IngestionIncrementalAppend,
	filament.IngestionIncrementalUpsert,
	filament.IngestionIncrementalDelete,
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
		policies := filament.EffectiveSourcePolicies(source, filament.NewConfig(conn.Config))
		replication := ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD
		if filament.IsCDCReplication(policies) {
			replication = ingestionv1.ReplicationMode_REPLICATION_MODE_CDC
		}
		return connect.NewResponse(&ingestionv1.GetConnectionCapabilitiesResponse{
			Connector:    spec.Name,
			Kind:         ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
			Capabilities: sourceCapabilitiesToProto(spec),
			Replication:  replication,
			ReadModes:    readModesForPolicies(policies),
			Ingestion: ingestionSupport(func(t filament.IngestionType) error {
				return validateSourceFor(spec, policies, t)
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
			WriteModes:   sinkWriteModes(spec),
			Ingestion: ingestionSupport(func(t filament.IngestionType) error {
				return filament.ValidateSinkIngestion(spec, t)
			}),
		}), nil
	default:
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("connection %q has no connector kind", conn.ID))
	}
}

// validateSourceFor validates against the connection's effective policies,
// falling back to the static spec for sources that declare only modes.
func validateSourceFor(spec filament.ConnectorSpec, policies []filament.SourcePolicy, t filament.IngestionType) error {
	if len(policies) > 0 {
		return filament.ValidateSourcePolicies(spec.Name, policies, t)
	}
	return filament.ValidateSourceIngestion(spec, t)
}

// readModesForPolicies reduces an effective policy set to the per-table read
// levers it offers; a CDC connection offers none.
func readModesForPolicies(policies []filament.SourcePolicy) []ingestionv1.ReadMode {
	var full, incremental bool
	for _, policy := range policies {
		switch policy.Mode {
		case filament.ModeFull:
			full = true
		case filament.ModeIncremental:
			incremental = true
		}
	}
	var out []ingestionv1.ReadMode
	if full {
		out = append(out, ingestionv1.ReadMode_READ_MODE_FULL)
	}
	if incremental {
		out = append(out, ingestionv1.ReadMode_READ_MODE_INCREMENTAL)
	}
	return out
}

// sinkWriteModes lists the write levers a sink offers. Append and replace are
// universal; upsert needs the capability. append_dedupe joins once a sink
// implements it.
func sinkWriteModes(spec filament.SinkSpec) []ingestionv1.WriteMode {
	out := []ingestionv1.WriteMode{
		ingestionv1.WriteMode_WRITE_MODE_APPEND,
		ingestionv1.WriteMode_WRITE_MODE_REPLACE,
	}
	if filament.ValidateSinkIngestion(spec, filament.IngestionSnapshotUpsert) == nil {
		out = append(out, ingestionv1.WriteMode_WRITE_MODE_UPSERT)
	}
	return out
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
// source/sink pair supports, whether the chosen type is among them, and the
// per-table setup that type involves. Snapshot and append edges have nothing
// to configure and skip source probing entirely; types that read a cursor or
// write by key probe the live source for per-table menus, cursor candidates,
// and primary keys. Only blocking requirements gate valid — an unset cursor
// that auto-detection covers is advisory.
func (a *Server) ValidatePipeline(ctx context.Context, req *connect.Request[ingestionv1.ValidatePipelineRequest]) (*connect.Response[ingestionv1.ValidatePipelineResponse], error) {
	ctx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()

	nodes := make(map[string]*ingestionv1.PipelineNode, len(req.Msg.GetNodes()))
	for _, node := range req.Msg.GetNodes() {
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

	for _, edge := range req.Msg.GetEdges() {
		if err := a.validateEdge(ctx, edge, nodes, req.Msg.GetTenantId(), probes, resp); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
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
	policies := filament.EffectiveSourcePolicies(source, filament.NewConfig(srcConn.Config))
	ev.SupportedWriteModes = sinkWriteModes(snkSpec)

	// A CDC connection implies the whole recipe; standard edges compile from
	// the two levers.
	var chosen filament.IngestionType
	if filament.IsCDCReplication(policies) {
		chosen = filament.IngestionCDC
	} else {
		var err error
		chosen, err = filament.IngestionFor(readModeFromProto(edge.GetReadMode()), writeModeFromProto(edge.GetWriteMode()))
		if err != nil {
			edgeError(ev, "read_mode", err.Error())
			return nil
		}
	}
	ev.IngestionType = ingestionTypeToProto(chosen)

	if err := validateSourceFor(srcSpec, policies, chosen); err != nil {
		edgeError(ev, "read_mode", err.Error())
	}
	if err := filament.ValidateSinkIngestion(snkSpec, chosen); err != nil {
		edgeError(ev, "write_mode", err.Error())
	}
	if len(ev.Errors) > 0 {
		return nil
	}

	a.edgeRequirements(ctx, edge, from, *srcConn, chosen, policies, probes, ev)
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

// edgeRequirements builds the per-table breakdown: which read modes each
// routed table can serve, its cursor requirement (satisfied, auto-covered,
// or blocking), and its primary-key requirement. The breakdown is always
// computed so the FE can offer per-table levers before anything is chosen;
// requirements only accompany chosen levers that involve setup.
func (a *Server) edgeRequirements(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn filament.Connection, chosen filament.IngestionType, policies []filament.SourcePolicy, probes *sourceProbes, ev *ingestionv1.EdgeValidation) {
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
					Message:         fmt.Sprintf("%s reads incrementally but the source cannot list resources, so per-resource cursors cannot be verified", chosen),
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

		keys, err := filament.PrimaryKeyForResource(ctx, src, resource)
		if err != nil {
			edgeError(ev, "from_node", fmt.Sprintf("could not inspect source: %v", err))
			continue
		}
		candidates, status := cursorCandidates(ctx, src, resource)
		// When candidates are unknowable stay optimistic; runtime decides.
		cursorable := cursors[resource] || len(candidates) > 0 ||
			status != ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED
		for _, mode := range readModesForPolicies(policies) {
			if mode == ingestionv1.ReadMode_READ_MODE_INCREMENTAL && !cursorable {
				continue
			}
			rv.SupportedReadModes = append(rv.SupportedReadModes, mode)
		}
		if needsCursor {
			rv.Requirements = append(rv.Requirements, cursorRequirement(resource, chosen, candidates, status, cursors[resource]))
		}
		if needsPK && len(keys) == 0 {
			rv.Requirements = append(rv.Requirements, &ingestionv1.Requirement{
				Kind:            ingestionv1.RequirementKind_REQUIREMENT_KIND_PRIMARY_KEY,
				Resource:        resource,
				Blocking:        true,
				CandidateStatus: ingestionv1.CandidateStatus_CANDIDATE_STATUS_ENUMERATED,
				Message:         fmt.Sprintf("%s requires a primary key but none was discovered for resource %q", chosen, resource),
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
func cursorRequirement(resource string, chosen filament.IngestionType, candidates []*ingestionv1.CandidateValue, status ingestionv1.CandidateStatus, satisfied bool) *ingestionv1.Requirement {
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
		requirement.Message = fmt.Sprintf("%s reads incrementally but cursor candidates for resource %q could not be determined", chosen, resource)
	case recommended != "":
		requirement.Message = fmt.Sprintf("auto-detected cursor %q will be used for resource %q", recommended, resource)
	case len(candidates) > 0:
		requirement.Blocking = true
		requirement.Message = fmt.Sprintf("%s requires a cursor column for resource %q", chosen, resource)
	default:
		requirement.Blocking = true
		requirement.Message = fmt.Sprintf("resource %q has no usable cursor column; replicate it fully via a snapshot edge instead", resource)
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
