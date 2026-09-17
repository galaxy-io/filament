package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/internal/compile"
	"github.com/galaxy-io/filament/internal/runs"
	scheduledomain "github.com/galaxy-io/filament/internal/schedule"
)

// CreatePipeline stores a new pipeline and assigns its id.
func (a *Server) CreatePipeline(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineRequest]) (*connect.Response[ingestionv1.CreatePipelineResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	id := uuid.NewString()
	workerConfiguration := defaultWorkerConfiguration()
	if supplied := req.Msg.GetWorkerConfiguration(); supplied != nil {
		proto.Merge(workerConfiguration, supplied)
	}
	if err := compile.ValidateWorkerConfiguration(compile.WorkerConfigurationFromProto(workerConfiguration)); err != nil {
		return nil, compileError(err)
	}
	pipeline := &ingestionv1.Pipeline{
		Id: id, TenantId: string(tenant), Name: req.Msg.GetName(), Description: req.Msg.GetDescription(),
		WorkerConfiguration: workerConfiguration,
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

// defaultWorkerConfiguration is persisted when a create request omits worker
// sizing. Keeping the default at the API boundary means every client gets the
// same behavior, and future default changes do not resize existing pipelines.
func defaultWorkerConfiguration() *ingestionv1.WorkerConfiguration {
	return &ingestionv1.WorkerConfiguration{
		Resources: &ingestionv1.WorkerResources{
			Requests: map[string]string{"cpu": "500m", "memory": "256Mi"},
			Limits:   map[string]string{"cpu": "1000m", "memory": "512Mi"},
		},
		NodeSelector: map[string]string{},
		Tolerations:  []*ingestionv1.WorkerToleration{},
	}
}

// CreatePipelineVersion appends an immutable graph version to a pipeline.
func (a *Server) CreatePipelineVersion(ctx context.Context, req *connect.Request[ingestionv1.CreatePipelineVersionRequest]) (*connect.Response[ingestionv1.CreatePipelineVersionResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
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
	pipeline, err := a.store.LoadPipeline(ctx, tenant, req.Msg.GetPipelineId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if pipeline.GetDeletedAt() != 0 {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("pipeline %q is deleted", pipeline.GetId()))
	}
	validationCtx, cancel := context.WithTimeout(ctx, resourceColumnsRPCTimeout)
	defer cancel()
	validation, err := a.validatePipelineGraph(validationCtx, pipeline.GetTenantId(), nodes, edges)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if !validation.GetValid() {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline graph is invalid: %s", pipelineValidationMessage(validation)))
	}
	if err := a.normalizeEdgeModes(ctx, tenant, nodes, edges); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	a.defaultSinkSchemas(ctx, tenant, nodes, edges)
	v, err := a.store.CreatePipelineVersion(ctx, tenant, req.Msg.GetPipelineId(), &ingestionv1.PipelineVersion{Graph: graph})
	// A new version can change the routes a scheduled occurrence compiles into
	// (or make the pipeline compilable for the first time) — refresh the
	// schedule's pre-created rows to match.
	if err == nil && a.schedules != nil {
		if st, scheduleErr := a.schedules.LoadPipelineSchedule(ctx, tenant, req.Msg.GetPipelineId()); scheduleErr == nil {
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

// normalizeEdgeModes makes defaults explicit, validates the available levers,
// and enforces one write mode per destination route. CDC defaults to append.
func (a *Server) normalizeEdgeModes(ctx context.Context, tenant filament.TenantID, nodes []*ingestionv1.PipelineNode, edges []*ingestionv1.PipelineEdge) error {
	byID := make(map[string]*ingestionv1.PipelineNode, len(nodes))
	for _, node := range nodes {
		byID[node.GetId()] = node
	}
	routeWriteModes := map[string]filament.WriteMode{}
	for _, edge := range edges {
		sourceNode := byID[edge.GetFromNode()]
		if sourceNode == nil {
			return fmt.Errorf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetFromNode())
		}
		sinkNode := byID[edge.GetToNode()]
		if sinkNode == nil {
			return fmt.Errorf("edge %s -> %s references unknown node %q", edge.GetFromNode(), edge.GetToNode(), edge.GetToNode())
		}
		sourceConn, err := a.store.LoadConnection(ctx, tenant, sourceNode.GetConnectionId())
		if err != nil {
			return fmt.Errorf("load connection %q: %w", sourceNode.GetConnectionId(), err)
		}
		sinkConn, err := a.store.LoadConnection(ctx, tenant, sinkNode.GetConnectionId())
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
			if edge.GetReadMode() != ingestionv1.ReadMode_READ_MODE_UNSPECIFIED {
				return fmt.Errorf("edge %s -> %s: CDC connections do not accept a read mode", edge.GetFromNode(), edge.GetToNode())
			}
			writeMode := filament.WriteAppend
			switch edge.GetWriteMode() {
			case ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED, ingestionv1.WriteMode_WRITE_MODE_APPEND:
				ingestionType = filament.IngestionCDCAppend
			case ingestionv1.WriteMode_WRITE_MODE_MERGE:
				writeMode = filament.WriteMerge
				ingestionType = filament.IngestionCDCMerge
			default:
				return fmt.Errorf("edge %s -> %s: CDC connections support append or merge write mode", edge.GetFromNode(), edge.GetToNode())
			}
			route := edge.GetFromNode() + "\x00" + edge.GetToNode()
			if previous, ok := routeWriteModes[route]; ok && previous != writeMode {
				return fmt.Errorf("edge %s -> %s: all resources on a route must use the same write mode", edge.GetFromNode(), edge.GetToNode())
			}
			routeWriteModes[route] = writeMode
			edge.WriteMode = writeModeToProto(writeMode)
		} else {
			readMode, err := readModeFromProto(edge.GetReadMode())
			if err != nil {
				return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
			}
			writeMode, err := writeModeFromProto(edge.GetWriteMode())
			if err != nil {
				return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
			}
			route := edge.GetFromNode() + "\x00" + edge.GetToNode()
			if previous, ok := routeWriteModes[route]; ok && previous != writeMode {
				return fmt.Errorf("edge %s -> %s: all resources on a route must use the same write mode", edge.GetFromNode(), edge.GetToNode())
			}
			routeWriteModes[route] = writeMode
			edge.ReadMode = readModeToProto(readMode)
			edge.WriteMode = writeModeToProto(writeMode)
			ingestionType, err = filament.IngestionFor(readMode, writeMode)
			if err != nil {
				return fmt.Errorf("edge %s -> %s: %w", edge.GetFromNode(), edge.GetToNode(), err)
			}
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
		if edge.GetReadMode() != ingestionv1.ReadMode_READ_MODE_INCREMENTAL {
			return fmt.Errorf("cursor configuration requires Incremental read mode")
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
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetPipelineId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	if err := compile.ValidateWorkerConfiguration(compile.WorkerConfigurationFromProto(req.Msg.GetWorkerConfiguration())); err != nil {
		return nil, compileError(err)
	}
	pipeline := &ingestionv1.Pipeline{
		Id: req.Msg.GetPipelineId(), TenantId: string(tenant), Name: req.Msg.GetName(), Description: req.Msg.GetDescription(),
		WorkerConfiguration: req.Msg.GetWorkerConfiguration(),
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
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	v, err := a.store.LoadPipelineVersion(ctx, tenant, req.Msg.GetPipelineId(), req.Msg.GetVersion())
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
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetPipelineId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("pipeline_id is required"))
	}
	options, err := listOptionsOf(req.Msg.GetPagination(), "", req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_CREATED_AT: "created_at",
		ingestionv1.SortBy_SORT_BY_UPDATED_AT: "updated_at",
	}, "version", true)
	if err != nil {
		return nil, err
	}
	versions, total, err := a.store.ListPipelineVersions(ctx, filament.PipelineVersionFilter{Tenant: tenant, PipelineID: req.Msg.GetPipelineId(), ListOptions: options})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&ingestionv1.ListPipelineVersionsResponse{Versions: versions, Pagination: paginationOf(req.Msg.GetPagination(), options, total)}), nil
}

// GetPipeline returns a pipeline with the requested related resources.
func (a *Server) GetPipeline(ctx context.Context, req *connect.Request[ingestionv1.GetPipelineRequest]) (*connect.Response[ingestionv1.GetPipelineResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	pipeline, err := a.store.LoadPipeline(ctx, tenant, req.Msg.GetId())
	if errors.Is(err, filament.ErrNotFound) {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := a.expandPipeline(ctx, tenant, pipeline, req.Msg.GetIncludeVersions(), req.Msg.GetIncludeLastRun(), req.Msg.GetIncludeSchedule()); err != nil {
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
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	options, err := listOptionsOf(req.Msg.GetPagination(), req.Msg.GetSearch(), req.Msg.GetSorting(), map[ingestionv1.SortBy]string{
		ingestionv1.SortBy_SORT_BY_NAME:       "name",
		ingestionv1.SortBy_SORT_BY_CREATED_AT: "created_at",
		ingestionv1.SortBy_SORT_BY_UPDATED_AT: "updated_at",
	}, "id", false)
	if err != nil {
		return nil, err
	}
	pipelines, total, err := a.store.ListPipelines(ctx, filament.PipelineFilter{
		Tenant: string(tenant), IncludeDeleted: req.Msg.GetIncludeDeleted(), ListOptions: options,
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	for _, pipeline := range pipelines {
		if err := a.expandPipeline(ctx, tenant, pipeline, req.Msg.GetIncludeVersions(), req.Msg.GetIncludeLastRun(), req.Msg.GetIncludeSchedule()); err != nil {
			return nil, err
		}
	}
	return connect.NewResponse(&ingestionv1.ListPipelinesResponse{Pipelines: pipelines, Pagination: paginationOf(req.Msg.GetPagination(), options, total)}), nil
}

func (a *Server) expandPipeline(ctx context.Context, tenant filament.TenantID, pipeline *ingestionv1.Pipeline, includeVersions, includeLastRun, includeSchedule bool) error {
	if includeVersions {
		versions, _, err := a.store.ListPipelineVersions(ctx, filament.PipelineVersionFilter{Tenant: tenant, PipelineID: pipeline.GetId(), ListOptions: filament.ListOptions{SortBy: "version", SortDescending: true}})
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		pipeline.Versions = versions
	}
	if includeLastRun {
		states, _, err := a.store.ListRuns(ctx, filament.RunFilter{
			Tenant:     tenant,
			PipelineID: pipeline.GetId(),
			Status: []filament.RunStatus{
				filament.RunRequested, filament.RunRunning, filament.RunCompleted,
				filament.RunFailed, filament.RunCanceled, filament.RunPaused, filament.RunPartial,
			},
			SortDescending: true,
			Limit:          1,
		})
		if err != nil {
			return connect.NewError(connect.CodeInternal, err)
		}
		if len(states) > 0 {
			pipeline.LastRun = runInfoToProto(states[0])
		}
	}
	if includeSchedule && a.schedules != nil {
		schedule, err := a.schedules.LoadPipelineSchedule(ctx, tenant, pipeline.GetId())
		if err == nil {
			pipeline.Schedule = pipelineScheduleToProto(schedule)
		} else if !errors.Is(err, filament.ErrNotFound) {
			return connect.NewError(connect.CodeInternal, err)
		}
	}
	return nil
}

// DeletePipeline removes the pipeline by id. The store's delete transaction
// also removes the pipeline's schedules and pending scheduled runs.
func (a *Server) DeletePipeline(ctx context.Context, req *connect.Request[ingestionv1.DeletePipelineRequest]) (*connect.Response[ingestionv1.DeletePipelineResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	if err := a.store.DeletePipeline(ctx, tenant, req.Msg.GetId()); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	a.deletePipelineNotifierSecrets(ctx, tenant, req.Msg.GetId())
	return connect.NewResponse(&ingestionv1.DeletePipelineResponse{}), nil
}

// RunPipeline groups the pipeline's edges into per-route runs and submits each
// to the orchestrator.
func (a *Server) RunPipeline(ctx context.Context, req *connect.Request[ingestionv1.RunPipelineRequest]) (*connect.Response[ingestionv1.RunPipelineResponse], error) {
	tenant, err := tenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	edgeRuns, err := a.submitPipeline(ctx, tenant, req.Msg)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&ingestionv1.RunPipelineResponse{EdgeRuns: edgeRuns}), nil
}

func (a *Server) submitPipeline(ctx context.Context, tenant filament.TenantID, req *ingestionv1.RunPipelineRequest) ([]*ingestionv1.PipelineEdgeRun, error) {
	token := req.GetClientToken()
	if token == "" {
		token = uuid.NewString()
	}
	workerCfg := compile.WorkerConfigurationFromProto(req.GetWorkerConfiguration())
	if err := compile.ValidateWorkerConfiguration(workerCfg); err != nil {
		return nil, compileError(err)
	}
	compiled, err := a.compiler.Compile(ctx, tenant, req.GetPipelineId(), token, runOptionsFromProto(req.GetOptions()), "", workerCfg)
	if err != nil {
		return nil, compileError(err)
	}
	var edgeRuns []*ingestionv1.PipelineEdgeRun
	for _, c := range compiled {
		run, err := a.orch.Submit(ctx, c.Submission)
		if err != nil {
			if errors.Is(err, filament.ErrRunOverlap) {
				return nil, connect.NewError(connect.CodeFailedPrecondition, err)
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if a.log != nil {
			a.log.Debug("pipeline route submitted",
				filament.Field{Key: "event.name", Value: "pipeline.route.submitted"},
				filament.Field{Key: "pipeline_edge", Value: c.Edge},
				filament.Field{Key: "source_connector", Value: c.Submission.Request.Source.Connector},
				filament.Field{Key: "sink_connector", Value: c.Submission.Request.Sink.Connector},
				filament.Field{Key: "resource_count", Value: len(c.Submission.Request.Resources)},
				filament.Field{Key: "run_id", Value: string(run)})
		}
		runRequest := c.Submission.Request
		state := filament.RunState{Run: run, Tenant: runRequest.Tenant, Request: runRequest, ScheduleID: runRequest.ScheduleID, Status: filament.RunRequested}
		edgeRuns = append(edgeRuns, &ingestionv1.PipelineEdgeRun{PipelineEdgeKey: c.Edge, Run: runInfoToProto(state)})
	}
	if a.log != nil {
		a.log.Debug("pipeline submitted",
			filament.Field{Key: "event.name", Value: "pipeline.submitted"},
			filament.Field{Key: "pipeline_id", Value: req.GetPipelineId()},
			filament.Field{Key: "run_count", Value: len(edgeRuns)})
	}
	return edgeRuns, nil
}

// reconcileScheduledRunsBestEffort refreshes the schedule's pre-created
// RunScheduled rows without failing the caller: the rows are a visibility
// artifact, and a schedule on a pipeline with no compilable version yet is
// expected to fail here until the first version lands.
func (a *Server) reconcileScheduledRunsBestEffort(ctx context.Context, st filament.ScheduleState) {
	if err := runs.ReconcileScheduled(ctx, a.store, a.compiler, st); err != nil {
		if a.log != nil {
			a.log.Warn("scheduled runs not reconciled",
				filament.Field{Key: "event.name", Value: "pipeline_schedule.reconcile_deferred"},
				filament.Field{Key: "schedule_id", Value: string(st.ID)},
				filament.Field{Key: "error", Value: err.Error()})
		}
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
