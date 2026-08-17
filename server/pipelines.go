package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/compile"
	"github.com/galaxy-io/filament/internal/runs"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
)

// CreatePipeline stores a new pipeline and assigns its id.
func (a *Server) CreatePipeline(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineRequest]) (*connect.Response[ingestionv1.CreatePipelineResponse], error) {
	id := uuid.NewString()
	if err := compile.ValidateWorkerConfiguration(compile.WorkerConfigurationFromProto(req.Msg.GetWorkerConfiguration())); err != nil {
		return nil, compileError(err)
	}
	pipeline := &ingestionv1.Pipeline{
		Id: id, TenantId: defaultTenant(req.Msg.GetTenantId()), Name: req.Msg.GetName(), Description: req.Msg.GetDescription(),
		WorkerConfiguration: req.Msg.GetWorkerConfiguration(),
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
	graph := req.Msg.GetGraph()
	if graph == nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("graph is required"))
	}
	nodes, edges := graph.GetNodes(), graph.GetEdges()
	if err := validateCursorConfigs(edges); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := a.normalizeEdgeModes(ctx, nodes, edges); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.defaultSinkSchemas(ctx, nodes, edges)
	v, err := a.store.CreatePipelineVersion(ctx, req.Msg.GetPipelineId(), &ingestionv1.PipelineVersion{Graph: graph})
	// A new version can change the routes a scheduled occurrence compiles into
	// (or make the pipeline compilable for the first time) — refresh the
	// schedule's pre-created rows to match.
	if err == nil && a.schedules != nil {
		if st, scheduleErr := a.schedules.LoadPipelineSchedule(ctx, req.Msg.GetPipelineId()); scheduleErr == nil {
			a.reconcileScheduledRunsBestEffort(ctx, st)
		}
	}
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.CreatePipelineVersionResponse{Version: v}), nil
}

// normalizeEdgeModes makes Replace explicit on Standard edges and validates
// the one public mode against both connector specs. CDC edges carry no mode.
func (a *Server) normalizeEdgeModes(ctx context.Context, nodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) error {
	byID := make(map[string]*ingestionv1.PipelineNode, len(nodes))
	for _, node := range nodes {
		byID[node.GetId()] = node
	}
	for _, edge := range edges {
		sourceNode := byID[edge.GetFromNode()]
		if sourceNode == nil {
			return fmt.Errorf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetFromNode())
		}
		sinkNode := byID[edge.GetToNode()]
		if sinkNode == nil {
			return fmt.Errorf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetToNode())
		}
		sourceConn, err := a.store.LoadConnection(ctx, sourceNode.GetConnectionId())
		if err != nil {
			return fmt.Errorf("load connection %q: %w", sourceNode.GetConnectionId(), err)
		}
		sinkConn, err := a.store.LoadConnection(ctx, sinkNode.GetConnectionId())
		if err != nil {
			return fmt.Errorf("load connection %q: %w", sinkNode.GetConnectionId(), err)
		}
		source, err := a.sources.Resolve(sourceConn.Connector)
		if err != nil {
			return err
		}
		sink, err := a.sinks.Resolve(sinkConn.Connector)
		if err != nil {
			return err
		}

		var ingestionType filament.IngestionType
		if filament.ReplicationOf(source, filament.NewConfig(sourceConn.Config)) == filament.ReplicationCDC {
			if edge.GetStandardSyncMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_UNSPECIFIED {
				return fmt.Errorf("edge %s -> %s: CDC connections do not accept a Standard sync mode", edge.GetFromNode(), edge.GetToNode())
			}
			ingestionType = filament.IngestionCDC
		} else {
			mode, err := standardSyncModeFromProto(edge.GetStandardSyncMode())
			if err != nil {
				return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
			}
			edge.StandardSyncMode = standardSyncModeToProto(mode)
			ingestionType = mode.IngestionType()
		}
		if err := filament.ValidateSourceIngestion(source.Spec(), ingestionType); err != nil {
			return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
		}
		if err := filament.ValidateSinkIngestion(sink.Spec(), ingestionType); err != nil {
			return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
		}
	}
	return nil
}

func validateCursorConfigs(edges []*ingestionv1.PipelineEdge) error {
	for _, edge := range edges {
		if len(edge.GetCursors()) == 0 {
			continue
		}
		if edge.GetStandardSyncMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL {
			return fmt.Errorf("cursor configuration requires Incremental sync mode")
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
	if err := compile.ValidateWorkerConfiguration(compile.WorkerConfigurationFromProto(pipeline.GetWorkerConfiguration())); err != nil {
		return nil, compileError(err)
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
	page, pagination, err := pageOf(versions, req.Msg.GetPagination())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.ListPipelineVersionsResponse{Versions: page, Pagination: pagination}), nil
}

// GetPipeline returns a pipeline with the requested related resources.
func (a *Server) GetPipeline(ctx context.Context, req *connect.Request[ingestionv1.GetPipelineRequest]) (*connect.Response[ingestionv1.GetPipelineResponse], error) {
	pipeline, err := a.store.LoadPipeline(ctx, req.Msg.GetId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := a.expandPipeline(ctx, pipeline, req.Msg.GetIncludeVersions(), req.Msg.GetIncludeLastRun(), req.Msg.GetIncludeSchedule()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.GetPipelineResponse{Pipeline: pipeline}), nil
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
		Enabled:    config.GetIsEnabled(),
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
			IsEnabled:     schedule.Enabled,
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
	pipelines, err := a.store.ListPipelines(ctx, filament.PipelineFilter{Tenant: req.Msg.GetTenantId(), IncludeDeleted: req.Msg.GetIncludeDeleted()})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	page, pagination, err := pageOf(pipelines, req.Msg.GetPagination())
	if err != nil {
		return nil, err
	}
	for _, pipeline := range page {
		if err := a.expandPipeline(ctx, pipeline, req.Msg.GetIncludeVersions(), req.Msg.GetIncludeLastRun(), req.Msg.GetIncludeSchedule()); err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(&ingestionv1.ListPipelinesResponse{Pipelines: page, Pagination: pagination}), nil
}

func (a *Server) expandPipeline(ctx context.Context, pipeline *ingestionv1.Pipeline, includeVersions, includeLastRun, includeSchedule bool) error {
	if includeVersions {
		versions, err := a.store.ListPipelineVersions(ctx, pipeline.GetId())
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		currentID := pipeline.GetCurrentVersion().GetId()
		for _, version := range versions {
			if version.GetId() != currentID {
				pipeline.Versions = append(pipeline.Versions, version)
			}
		}
	}
	if includeLastRun {
		states, _, err := a.store.ListRuns(ctx, filament.RunFilter{
			PipelineID: pipeline.GetId(),
			Status: []filament.RunStatus{
				filament.RunRequested, filament.RunRunning, filament.RunCompleted,
				filament.RunFailed, filament.RunCanceled, filament.RunPaused, filament.RunPartial,
			},
			Limit: 1,
		})
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if len(states) > 0 {
			pipeline.LastRun = runInfoToProto(states[0])
		}
	}
	if includeSchedule && a.schedules != nil {
		schedule, err := a.schedules.LoadPipelineSchedule(ctx, pipeline.GetId())
		if err == nil {
			pipeline.Schedule = pipelineScheduleToProto(schedule)
		} else if !errors.Is(err, filament.ErrNotFound) {
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return nil
}

// DeletePipeline removes the pipeline by id.
func (a *Server) DeletePipeline(ctx context.Context, req *connect.Request[ingestionv1.DeletePipelineRequest]) (*connect.Response[ingestionv1.DeletePipelineResponse], error) {
	if err := a.store.DeletePipeline(ctx, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	// The schedule RPCs reject deleted pipelines, so this is the last chance to
	// reap the schedule's pending pre-created runs.
	if a.schedules != nil {
		if st, err := a.schedules.LoadPipelineSchedule(ctx, req.Msg.GetId()); err == nil {
			if err := runs.DropScheduled(ctx, a.store, st.ID); err != nil {
				fmt.Printf("[ingestion-api] drop scheduled runs schedule=%s err=%v\n", st.ID, err)
			}
		}
	}
	return connect.NewResponse(&ingestionv1.DeletePipelineResponse{}), nil
}

// RunPipeline groups the pipeline's edges into per-route runs and submits each
// to the orchestrator.
func (a *Server) RunPipeline(ctx context.Context, req *connect.Request[ingestionv1.RunPipelineRequest]) (*connect.Response[ingestionv1.RunPipelineResponse], error) {
	edgeRuns, err := a.submitPipeline(ctx, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.RunPipelineResponse{EdgeRuns: edgeRuns}), nil
}

func (a *Server) submitPipeline(ctx context.Context, req *ingestionv1.RunPipelineRequest) ([]*ingestionv1.PipelineEdgeRun, error) {
	token := req.GetClientToken()
	if token == "" {
		token = uuid.NewString()
	}
	workerCfg := compile.WorkerConfigurationFromProto(req.GetWorkerConfiguration())
	if err := compile.ValidateWorkerConfiguration(workerCfg); err != nil {
		return nil, compileError(err)
	}
	compiled, err := a.compiler.Compile(ctx, req.GetPipelineId(), token, runOptionsFromProto(req.GetOptions()), "", workerCfg)
	if err != nil {
		return nil, compileError(err)
	}
	var edgeRuns []*ingestionv1.PipelineEdgeRun
	for _, c := range compiled {
		run, err := a.orch.Submit(ctx, c.Req)
		if err != nil {
			return nil, err
		}
		fmt.Printf("[ingestion-api] RunPipeline route=%s source=%s sink=%s resources=%v run=%s\n", c.Edge, c.Req.Source.Provider, c.Req.Sink.Provider, c.Req.Resources, run)
		state := filament.RunState{Run: run, Tenant: c.Req.Tenant, Request: c.Req, ScheduleID: c.Req.ScheduleID, Status: filament.RunRequested}
		edgeRuns = append(edgeRuns, &ingestionv1.PipelineEdgeRun{PipelineEdgeKey: c.Edge, Run: runInfoToProto(state)})
	}
	fmt.Printf("[ingestion-api] RunPipeline pipeline=%s runs=%d\n", req.GetPipelineId(), len(edgeRuns))
	return edgeRuns, nil
}

// reconcileScheduledRunsBestEffort refreshes the schedule's pre-created
// RunScheduled rows without failing the caller: the rows are a visibility
// artifact, and a schedule on a pipeline with no compilable version yet is
// expected to fail here until the first version lands.
func (a *Server) reconcileScheduledRunsBestEffort(ctx context.Context, st filament.ScheduleState) {
	if err := runs.ReconcileScheduled(ctx, a.store, a.compiler, st); err != nil {
		fmt.Printf("[ingestion-api] reconcile scheduled runs schedule=%s err=%v\n", st.ID, err)
	}
}

// compileError maps compiler sentinels onto Connect codes.
func compileError(err error) error {
	switch {
	case errors.Is(err, filament.ErrNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case errors.Is(err, compile.ErrPrecondition):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, compile.ErrInvalid):
		return connect.NewError(connect.CodeInvalidArgument, err)
	default:
		return connect.NewError(connect.CodeInternal, err)
	}
}
