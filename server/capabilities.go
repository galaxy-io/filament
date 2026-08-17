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

// ValidatePipeline checks every edge of a graph: the Standard sync modes the
// source/sink pair supports, whether the chosen mode is among them, and the
// per-table setup that mode involves. Replace and Append edges have nothing
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
	replication := filament.ReplicationOf(source, filament.NewConfig(srcConn.Config))
	ev.Replication = replicationToProto(replication)

	// A CDC connection implies the complete recipe. Standard edges expose one
	// destination-oriented mode which compiles to a canonical internal recipe.
	var chosen filament.IngestionType
	if replication == filament.ReplicationCDC {
		chosen = filament.IngestionCDC
		if edge.GetStandardSyncMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_UNSPECIFIED {
			edgeError(ev, "standard_sync_mode", "CDC connections do not accept a Standard sync mode")
		}
		if len(edge.GetCursors()) > 0 {
			edgeError(ev, "cursors", "CDC connections manage their stream position automatically")
		}
	} else {
		ev.SupportedModes = supportedStandardSyncModes(srcSpec, snkSpec)
		mode, err := standardSyncModeFromProto(edge.GetStandardSyncMode())
		if err != nil {
			edgeError(ev, "standard_sync_mode", err.Error())
			return nil
		}
		ev.EffectiveMode = standardSyncModeToProto(mode)
		chosen = mode.IngestionType()
		if !containsStandardSyncMode(ev.SupportedModes, ev.EffectiveMode) {
			edgeError(ev, "standard_sync_mode", fmt.Sprintf("%s is not supported by this source and sink", mode))
		}
	}

	if err := filament.ValidateSourceIngestion(srcSpec, chosen); err != nil {
		edgeError(ev, "standard_sync_mode", err.Error())
	}
	if err := filament.ValidateSinkIngestion(snkSpec, chosen); err != nil {
		edgeError(ev, "standard_sync_mode", err.Error())
	}
	if len(ev.Errors) > 0 && replication == filament.ReplicationCDC {
		return nil
	}

	a.resourceBreakdown(ctx, edge, from, *srcConn, chosen, ev.SupportedModes, probes, ev)
	return nil
}

func supportedStandardSyncModes(source filament.ConnectorSpec, sink filament.SinkSpec) []ingestionv1.StandardSyncMode {
	modes := []filament.StandardSyncMode{
		filament.StandardSyncReplace,
		filament.StandardSyncAppend,
		filament.StandardSyncIncremental,
	}
	out := make([]ingestionv1.StandardSyncMode, 0, len(modes))
	for _, mode := range modes {
		ingestionType := mode.IngestionType()
		if filament.ValidateSourceIngestion(source, ingestionType) == nil && filament.ValidateSinkIngestion(sink, ingestionType) == nil {
			out = append(out, standardSyncModeToProto(mode))
		}
	}
	return out
}

func containsStandardSyncMode(modes []ingestionv1.StandardSyncMode, want ingestionv1.StandardSyncMode) bool {
	for _, mode := range modes {
		if mode == want {
			return true
		}
	}
	return false
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

// resourceBreakdown narrows the pair-level modes using each table's cursor and
// primary-key reality, then reports requirements for the selected mode.
func (a *Server) resourceBreakdown(ctx context.Context, edge *ingestionv1.PipelineEdge, from *ingestionv1.PipelineNode, srcConn filament.Connection, chosen filament.IngestionType, supportedModes []ingestionv1.StandardSyncMode, probes *sourceProbes, ev *ingestionv1.EdgeValidation) {
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
		for _, mode := range supportedModes {
			if mode == ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL && (!cursorable || keyErr != nil || len(keys) == 0) {
				continue
			}
			rv.SupportedModes = append(rv.SupportedModes, mode)
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
