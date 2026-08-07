package server

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
)

// CreatePipeline stores a new pipeline and assigns its id.
func (a *Server) CreatePipeline(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineRequest]) (*connect.Response[ingestionv1.CreatePipelineResponse], error) {
	id := uuid.NewString()
	pipeline := &ingestionv1.Pipeline{
		Id: id, TenantId: defaultTenant(req.Msg.GetTenantId()), Name: req.Msg.GetName(), Description: req.Msg.GetDescription(),
	}
	var schedule *filament.ScheduleState
	if config := req.Msg.GetSchedule(); config != nil {
		if a.schedules == nil {
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("schedule store is not configured"))
		}
		state, err := newPipelineSchedule(pipeline, config)
		if err != nil {
			return nil, connect.NewError(connect.CodeInvalidArgument, err)
		}
		schedule = &state
	}
	var created *ingestionv1.Pipeline
	var err error
	if a.schedules != nil {
		created, err = a.schedules.CreatePipelineWithSchedule(ctx, pipeline, schedule)
	} else {
		created, err = a.store.CreatePipeline(ctx, pipeline)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := &ingestionv1.CreatePipelineResponse{Pipeline: created}
	if schedule != nil {
		res.Schedule = pipelineScheduleToProto(*schedule)
	}
	return connect.NewResponse(res), nil
}

// CreatePipelineVersion appends an immutable graph version to a pipeline.
func (a *Server) CreatePipelineVersion(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineVersionRequest]) (*connect.Response[ingestionv1.CreatePipelineVersionResponse], error) {
	if req.Msg.GetPipelineId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	nodes, edges := req.Msg.GetNodes(), req.Msg.GetEdges()
	if err := validateCursorConfigs(edges); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := a.deriveEdgeTypes(ctx, nodes, edges); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.defaultSinkSchemas(ctx, nodes, edges)
	v, err := a.store.CreatePipelineVersion(ctx, req.Msg.GetPipelineId(), &ingestionv1.PipelineVersion{Nodes: nodes, Edges: edges})
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.CreatePipelineVersionResponse{Version: v}), nil
}

// deriveEdgeTypes compiles every edge's read/write levers — or its source
// connection's CDC replication — into the stored ingestion type. Clients
// never set ingestion_type; it is derived here at save time.
func (a *Server) deriveEdgeTypes(ctx context.Context, nodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) error {
	byID := make(map[string]*ingestionv1.PipelineNode, len(nodes))
	for _, node := range nodes {
		byID[node.GetId()] = node
	}
	cdcByConnection := map[string]bool{}
	for _, edge := range edges {
		node := byID[edge.GetFromNode()]
		if node == nil {
			return fmt.Errorf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetFromNode())
		}
		isCDC, ok := cdcByConnection[node.GetConnectionId()]
		if !ok {
			conn, err := a.store.LoadConnection(ctx, node.GetConnectionId())
			if err != nil {
				return fmt.Errorf("load connection %q: %w", node.GetConnectionId(), err)
			}
			source, err := a.sources.Resolve(conn.Connector)
			if err != nil {
				return err
			}
			isCDC = filament.ReplicationOf(source, filament.NewConfig(conn.Config)) == filament.ReplicationCDC
			cdcByConnection[node.GetConnectionId()] = isCDC
		}
		if isCDC {
			edge.IngestionType = ingestionTypeToProto(filament.IngestionCDC)
			continue
		}
		compiled, err := filament.IngestionFor(readModeFromProto(edge.GetReadMode()), writeModeFromProto(edge.GetWriteMode()))
		if err != nil {
			return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
		}
		edge.IngestionType = ingestionTypeToProto(compiled)
	}
	return nil
}

func validateCursorConfigs(edges []*ingestionv1.PipelineEdge) error {
	for _, edge := range edges {
		if len(edge.GetCursors()) == 0 {
			continue
		}
		if readModeFromProto(edge.GetReadMode()) != filament.ModeIncremental {
			return fmt.Errorf("cursor configuration requires an incremental read mode")
		}
		seen := make(map[string]struct{}, len(edge.GetCursors()))
		for _, cursor := range edge.GetCursors() {
			resource := cursor.GetResource()
			if resource == "" {
				return fmt.Errorf("cursor resource is required")
			}
			if edge.GetResource() != "" && resource != edge.GetResource() {
				return fmt.Errorf("cursor resource %q does not match edge resource %q", resource, edge.GetResource())
			}
			if cursor.GetField() == "" {
				return fmt.Errorf("cursor field is required for resource %q", resource)
			}
			if cursor.GetLookbackSeconds() < 0 {
				return fmt.Errorf("cursor lookback_seconds must be non-negative for resource %q", resource)
			}
			if _, duplicate := seen[resource]; duplicate {
				return fmt.Errorf("duplicate cursor configuration for resource %q", resource)
			}
			seen[resource] = struct{}{}
		}
	}
	return nil
}

// UpdatePipeline changes mutable pipeline metadata. Graph changes are stored as
// immutable versions through CreatePipelineVersion.
func (a *Server) UpdatePipeline(ctx context.Context, req *connect.Request[ingestionv1.UpdatePipelineRequest]) (*connect.Response[ingestionv1.UpdatePipelineResponse], error) {
	pipeline := req.Msg.GetPipeline()
	if pipeline == nil || pipeline.GetId() == "" {
		return nil, fmt.Errorf("pipeline.id is required")
	}
	next, err := a.store.UpdatePipeline(ctx, pipeline)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.UpdatePipelineResponse{Pipeline: next}), nil
}

// GetPipelineVersion returns a specific immutable pipeline graph version.
func (a *Server) GetPipelineVersion(ctx context.Context, req *connect.Request[ingestionv1.GetPipelineVersionRequest]) (*connect.Response[ingestionv1.GetPipelineVersionResponse], error) {
	v, err := a.store.LoadPipelineVersion(ctx, req.Msg.GetPipelineId(), req.Msg.GetVersion())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.GetPipelineVersionResponse{Version: v}), nil
}

// ListPipelineVersions returns all of a pipeline's graph versions, newest first.
func (a *Server) ListPipelineVersions(ctx context.Context, req *connect.Request[ingestionv1.ListPipelineVersionsRequest]) (*connect.Response[ingestionv1.ListPipelineVersionsResponse], error) {
	if req.Msg.GetPipelineId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	versions, err := a.store.ListPipelineVersions(ctx, req.Msg.GetPipelineId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.ListPipelineVersionsResponse{Versions: versions}), nil
}

// GetPipeline returns the pipeline by id along with its current graph version
// and full version history, newest first.
func (a *Server) GetPipeline(ctx context.Context, req *connect.Request[ingestionv1.GetPipelineRequest]) (*connect.Response[ingestionv1.GetPipelineResponse], error) {
	pipeline, err := a.store.LoadPipeline(ctx, req.Msg.GetId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	versions, err := a.store.ListPipelineVersions(ctx, pipeline.GetId())
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := &ingestionv1.GetPipelineResponse{Pipeline: pipeline, Versions: versions}
	for _, v := range versions {
		if v.GetVersion() == pipeline.GetCurrentVersionId() {
			res.CurrentVersion = v
			break
		}
	}
	if a.schedules != nil {
		schedule, err := a.schedules.LoadPipelineSchedule(ctx, pipeline.GetId())
		if err == nil {
			res.Schedule = pipelineScheduleToProto(schedule)
		} else if !errors.Is(err, filament.ErrNotFound) {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(res), nil
}

func newPipelineSchedule(pipeline *ingestionv1.Pipeline, config *ingestionv1.PipelineScheduleConfig) (filament.ScheduleState, error) {
	timezone := config.GetTimezone()
	if timezone == "" {
		timezone = "UTC"
	}
	overlap, err := pipelineScheduleOverlapFromProto(config.GetOverlapPolicy())
	if err != nil {
		return filament.ScheduleState{}, err
	}
	spec := filament.ScheduleSpec{
		Tenant:     filament.TenantID(pipeline.GetTenantId()),
		Name:       pipeline.GetName(),
		PipelineID: pipeline.GetId(),
		Cron:       config.GetCron(),
		Timezone:   timezone,
		Overlap:    overlap,
		Enabled:    config.GetEnabled(),
	}
	now := time.Now()
	next, err := scheduledomain.NextFire(spec, now)
	if err != nil {
		return filament.ScheduleState{}, err
	}
	if !spec.Enabled {
		next = nil
	}
	return filament.ScheduleState{
		ID:        filament.ScheduleID(uuid.NewString()),
		Spec:      spec,
		Enabled:   spec.Enabled,
		NextFire:  next,
		CreatedAt: now,
	}, nil
}

func pipelineScheduleToProto(schedule filament.ScheduleState) *ingestionv1.PipelineSchedule {
	out := &ingestionv1.PipelineSchedule{
		Id:         string(schedule.ID),
		PipelineId: schedule.Spec.PipelineID,
		Config: &ingestionv1.PipelineScheduleConfig{
			Cron:          schedule.Spec.Cron,
			Timezone:      schedule.Spec.Timezone,
			Enabled:       schedule.Enabled,
			OverlapPolicy: pipelineScheduleOverlapToProto(schedule.Spec.Overlap),
		},
	}
	if schedule.NextFire != nil {
		out.NextFireAt = schedule.NextFire.UnixMilli()
	}
	if schedule.LastFired != nil {
		out.LastFiredAt = schedule.LastFired.UnixMilli()
	}
	return out
}

func pipelineScheduleOverlapFromProto(policy ingestionv1.PipelineScheduleOverlapPolicy) (filament.OverlapPolicy, error) {
	switch policy {
	case ingestionv1.PipelineScheduleOverlapPolicy_PIPELINE_SCHEDULE_OVERLAP_POLICY_UNSPECIFIED,
		ingestionv1.PipelineScheduleOverlapPolicy_PIPELINE_SCHEDULE_OVERLAP_POLICY_SKIP:
		return filament.OverlapSkip, nil
	case ingestionv1.PipelineScheduleOverlapPolicy_PIPELINE_SCHEDULE_OVERLAP_POLICY_ALLOW:
		return filament.OverlapAllow, nil
	default:
		return 0, fmt.Errorf("unsupported overlap policy %d", policy)
	}
}

func pipelineScheduleOverlapToProto(policy filament.OverlapPolicy) ingestionv1.PipelineScheduleOverlapPolicy {
	if policy == filament.OverlapAllow {
		return ingestionv1.PipelineScheduleOverlapPolicy_PIPELINE_SCHEDULE_OVERLAP_POLICY_ALLOW
	}
	return ingestionv1.PipelineScheduleOverlapPolicy_PIPELINE_SCHEDULE_OVERLAP_POLICY_SKIP
}

// ListPipelines returns pipelines, optionally filtered by tenant.
func (a *Server) ListPipelines(ctx context.Context, req *connect.Request[ingestionv1.ListPipelinesRequest]) (*connect.Response[ingestionv1.ListPipelinesResponse], error) {
	pipelines, err := a.store.ListPipelines(ctx, req.Msg.GetTenantId())
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
	bindings, err := a.submitPipeline(ctx, req.Msg, "")
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.RunPipelineResponse{Runs: bindings}), nil
}

// SubmitScheduledPipeline compiles and submits a pipeline for one claimed
// schedule occurrence. The occurrence token makes each cron tick idempotent.
func (a *Server) SubmitScheduledPipeline(ctx context.Context, pipelineID string, scheduleID filament.ScheduleID, token string) ([]filament.RunID, error) {
	bindings, err := a.submitPipeline(ctx, &ingestionv1.RunPipelineRequest{
		PipelineId:  pipelineID,
		ClientToken: token,
	}, scheduleID)
	if err != nil {
		return nil, err
	}
	ids := make([]filament.RunID, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, filament.RunID(binding.GetRunId()))
	}
	return ids, nil
}

func (a *Server) submitPipeline(ctx context.Context, req *ingestionv1.RunPipelineRequest, scheduleID filament.ScheduleID) ([]*ingestionv1.RunBinding, error) {
	pipeline, err := a.store.LoadPipeline(ctx, req.GetPipelineId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	version, err := a.store.LoadPipelineVersion(ctx, pipeline.GetId(), pipeline.GetCurrentVersionId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("pipeline has no version: %w", err))
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	nodes := map[string]*ingestionv1.PipelineNode{}
	connections := map[string]filament.Connection{}
	for _, node := range version.GetNodes() {
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
	token := req.GetClientToken()
	if token == "" {
		token = uuid.NewString()
	}
	groups, err := groupEdges(version.GetEdges(), nodes)
	if err != nil {
		return nil, err
	}
	options := runOptionsFromProto(req.GetOptions())
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
			Tenant:             filament.TenantID(defaultTenant(pipeline.GetTenantId())),
			PipelineID:         pipeline.GetId(),
			PipelineVersionID:  version.GetVersion(),
			IdempotencyKey:     fmt.Sprintf("%s:%s:%s", pipeline.GetId(), token, key),
			Source:             sourceRef,
			Sink:               sinkRef,
			SourceConnectionID: group.source.GetConnectionId(),
			SinkConnectionID:   group.sink.GetConnectionId(),
			Resources:          resources,
			Selectors:          selectors,
			IngestionType:      group.ingestionType,
			CheckpointRoute:    key,
			CursorConfigs:      group.cursorConfigs,
			Options:            options,
			ScheduleID:         scheduleID,
		})
		if err != nil {
			return nil, err
		}
		fmt.Printf("[ingestion-api] RunPipeline route=%s source=%s sink=%s resources=%v run=%s\n", key, sourceRef.Provider, sinkRef.Provider, resources, run)
		bindings = append(bindings, &ingestionv1.RunBinding{Edge: key, RunId: string(run)})
	}
	fmt.Printf("[ingestion-api] RunPipeline pipeline=%s runs=%d\n", pipeline.GetId(), len(bindings))
	return bindings, nil
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
	cursorConfigs map[string]filament.ResourceCursorConfig
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
				cursorConfigs: map[string]filament.ResourceCursorConfig{},
			}
			byKey[key] = group
			ordered = append(ordered, group)
		}
		for _, cursor := range edge.GetCursors() {
			config := filament.ResourceCursorConfig{Field: cursor.GetField(), LookbackSeconds: cursor.GetLookbackSeconds()}
			if previous, exists := group.cursorConfigs[cursor.GetResource()]; exists && previous != config {
				return nil, fmt.Errorf("conflicting cursor configuration for resource %q", cursor.GetResource())
			}
			group.cursorConfigs[cursor.GetResource()] = config
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
