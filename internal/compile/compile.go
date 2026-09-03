// Package compile turns a stored pipeline into per-route run requests. It is
// the shared half of run intake: the server compiles for manual RunPipeline
// calls and the scheduler compiles for cron fires. Errors carry sentinels
// (filament.ErrNotFound, ErrPrecondition, ErrInvalid) instead of transport
// codes; callers map them to their own surface.
package compile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ErrPrecondition marks a pipeline that exists but cannot run: it or a
// connection it references is deleted, or it has no version yet.
var ErrPrecondition = errors.New("pipeline not runnable")

// ErrInvalid marks a pipeline definition the compiler rejects: edges with
// conflicting read modes, route write modes, or cursors; references to missing
// nodes or connections; or a node overlay that violates field scopes.
var ErrInvalid = errors.New("invalid pipeline")

// Compiler collapses a pipeline's current version into ready-to-submit run
// requests. DefaultTenant stands in for a pipeline row with no tenant.
type Compiler struct {
	Store         filament.DataStore
	Sources       filament.SourceRegistry
	Sinks         filament.SinkRegistry
	DefaultTenant string
}

// CompiledRun is one route's ready-to-submit request, keyed by its canvas edge.
type CompiledRun struct {
	Edge string
	Req  filament.RunRequest
}

type durableReplicationStreamStore interface {
	filament.ReplicationStreamStore
	filament.ReplicationStreamRunStore
}

// Compile loads the pipeline's current version and collapses its edges into
// per-route run requests. token salts each route's idempotency key. workerCfg
// overrides the pipeline's own worker configuration field by field for this
// call only; the resolved result is stamped onto every request, so a later edit
// to the pipeline cannot reshape a run already requested.
func (c *Compiler) Compile(ctx context.Context, pipelineID, token string, options filament.RunOptions, scheduleID filament.ScheduleID, workerCfg filament.WorkerConfiguration) ([]CompiledRun, error) {
	pipeline, err := c.Store.LoadPipeline(ctx, pipelineID)
	if err != nil {
		return nil, fmt.Errorf("load pipeline %q: %w", pipelineID, err)
	}
	if pipeline.GetDeletedAt() != 0 {
		return nil, fmt.Errorf("%w: pipeline %q is deleted", ErrPrecondition, pipeline.GetId())
	}
	version, err := c.Store.LoadPipelineVersion(ctx, pipeline.GetId(), 0)
	if errors.Is(err, filament.ErrNotFound) {
		return nil, fmt.Errorf("%w: pipeline %q has no version", ErrPrecondition, pipeline.GetId())
	}
	if err != nil {
		return nil, fmt.Errorf("load pipeline version: %w", err)
	}
	nodes := map[string]*ingestionv1.PipelineNode{}
	connections := map[string]filament.Connection{}
	for _, node := range version.GetGraph().GetNodes() {
		nodes[node.GetId()] = node
		if _, ok := connections[node.GetConnectionId()]; !ok {
			conn, err := c.Store.LoadConnection(ctx, node.GetConnectionId())
			if err != nil {
				return nil, fmt.Errorf("load connection %q: %w", node.GetConnectionId(), err)
			}
			if conn.DeletedAt != 0 {
				return nil, fmt.Errorf("%w: connection %q is deleted", ErrPrecondition, node.GetConnectionId())
			}
			connections[node.GetConnectionId()] = conn
		}
	}

	groups, err := groupEdges(version.GetGraph().GetEdges(), nodes)
	if err != nil {
		return nil, err
	}
	tenant := pipeline.GetTenantId()
	if tenant == "" {
		tenant = c.DefaultTenant
	}
	worker := workerCfg.Merge(WorkerConfigurationFromProto(pipeline.GetWorkerConfiguration()))
	compiled := make([]CompiledRun, 0, len(groups))
	for _, group := range groups {
		key := group.key
		resources, selectors := routeResources(group)
		sourceRef, err := c.resolveNodeRef(group.source, connections)
		if err != nil {
			return nil, err
		}
		sinkRef, err := c.resolveNodeRef(group.sink, connections)
		if err != nil {
			return nil, err
		}
		source, err := c.Sources.Resolve(sourceRef.Connector)
		if err != nil {
			return nil, err
		}
		cdc := filament.ReplicationOf(source, filament.NewConfig(sourceRef.Config)) == filament.ReplicationCDC
		ingestionTypes, err := compileIngestionTypes(group, cdc)
		if err != nil {
			return nil, err
		}
		replicationStream, err := c.planRouteReplicationStream(
			cdc, source, pipeline.GetId(), version.GetId(), filament.TenantID(tenant),
			key, group, connections, sourceRef, sinkRef,
		)
		if err != nil {
			return nil, err
		}
		replicationStreamID, replicationStreamGeneration := replicationStreamIdentity(replicationStream)
		compiled = append(compiled, CompiledRun{Edge: key, Req: filament.RunRequest{
			Tenant:                      filament.TenantID(tenant),
			PipelineID:                  pipeline.GetId(),
			PipelineVersionID:           version.GetId(),
			IdempotencyKey:              fmt.Sprintf("%s:%s:%s", pipeline.GetId(), token, key),
			Source:                      sourceRef,
			Sink:                        sinkRef,
			SourceConnectionID:          group.source.GetConnectionId(),
			SinkConnectionID:            group.sink.GetConnectionId(),
			Resources:                   resources,
			Selectors:                   selectors,
			IngestionTypes:              ingestionTypes,
			CheckpointRoute:             key,
			ReplicationStreamID:         replicationStreamID,
			ReplicationStreamGeneration: replicationStreamGeneration,
			ReplicationStream:           replicationStream,
			CursorConfigs:               group.cursorConfigs,
			Options:                     options,
			ScheduleID:                  scheduleID,
			WorkerConfiguration:         worker,
		}})
	}
	return compiled, nil
}

func routeResources(group *routeGroup) (resources, selectors []string) {
	if group.all {
		return nil, nil
	}
	for resource := range group.resources {
		resources = append(resources, resource)
	}
	slices.Sort(resources)
	for selector := range group.selectors {
		selectors = append(selectors, selector)
	}
	slices.Sort(selectors)
	return resources, selectors
}

func (c *Compiler) planRouteReplicationStream(
	cdc bool,
	source filament.Source,
	pipelineID, pipelineVersionID string,
	tenant filament.TenantID,
	route string,
	group *routeGroup,
	connections map[string]filament.Connection,
	sourceRef, sinkRef filament.Ref,
) (*filament.ReplicationStream, error) {
	planner, plansStreams := source.(filament.ReplicationStreamPlanner)
	if !cdc || !plansStreams {
		return nil, nil
	}
	if _, ok := c.Store.(durableReplicationStreamStore); !ok {
		return nil, fmt.Errorf("%w: datastore %q cannot durably admit replication streams", ErrPrecondition, c.Store.Name())
	}
	planned, err := c.planReplicationStream(
		planner, pipelineID, pipelineVersionID, tenant, route, group, connections, sourceRef, sinkRef,
	)
	if err != nil {
		return nil, err
	}
	return &planned, nil
}

func replicationStreamIdentity(stream *filament.ReplicationStream) (string, int64) {
	if stream == nil {
		return "", 0
	}
	return stream.ID, stream.Generation
}

func (c *Compiler) planReplicationStream(
	planner filament.ReplicationStreamPlanner,
	pipelineID, pipelineVersionID string,
	tenant filament.TenantID,
	route string,
	group *routeGroup,
	connections map[string]filament.Connection,
	sourceRef, sinkRef filament.Ref,
) (filament.ReplicationStream, error) {
	id := uuid.NewString()
	plan, err := planner.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{
		ReplicationStreamID: id, Config: filament.NewConfig(sourceRef.Config),
	})
	if err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("plan replication stream %q: %w", route, err)
	}
	fingerprint, err := replicationStreamContinuityFingerprint(group, connections, sourceRef.Connector, plan.ContinuityConfig, sinkRef)
	if err != nil {
		return filament.ReplicationStream{}, fmt.Errorf("fingerprint replication stream %q: %w", route, err)
	}
	desired := filament.ReplicationStream{
		ID: id, Tenant: tenant, PipelineID: pipelineID, Route: route,
		SourceConnectionID: group.source.GetConnectionId(), SinkConnectionID: group.sink.GetConnectionId(),
		ConsumerName: plan.ConsumerName, ConsumerConfig: plan.ConsumerConfig,
		ContinuityFingerprint: fingerprint, CreatedFromPipelineVersionID: pipelineVersionID,
	}
	return desired, nil
}

// replicationStreamContinuityFingerprint excludes the selected resource set;
// the source planner decides which of its configuration fields affect
// continuity. Adding a table can therefore reuse the stream, while changing
// its source identity, sink target, or write semantics forks it.
func replicationStreamContinuityFingerprint(group *routeGroup, connections map[string]filament.Connection, sourceConnector string, sourceContinuity map[string]any, sinkRef filament.Ref) (string, error) {
	sourceConnection := connections[group.source.GetConnectionId()]
	sinkConnection := connections[group.sink.GetConnectionId()]
	identity := struct {
		SourceConnector         string         `json:"source_connector"`
		SourceConnectionID      string         `json:"source_connection_id"`
		SourceConnectionVersion int64          `json:"source_connection_version"`
		SourceContinuity        map[string]any `json:"source_continuity"`
		SinkConnector           string         `json:"sink_connector"`
		SinkConnectionID        string         `json:"sink_connection_id"`
		SinkConnectionVersion   int64          `json:"sink_connection_version"`
		SinkConfig              map[string]any `json:"sink_config"`
		WriteMode               string         `json:"write_mode"`
	}{
		SourceConnector: sourceConnector, SourceConnectionID: sourceConnection.ID,
		SourceConnectionVersion: sourceConnection.Version,
		SourceContinuity:        sourceContinuity,
		SinkConnector:           sinkRef.Connector, SinkConnectionID: sinkConnection.ID,
		SinkConnectionVersion: sinkConnection.Version, SinkConfig: sinkRef.Config,
		WriteMode: fmt.Sprint(group.writeMode),
	}
	raw, err := json.Marshal(identity)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func cdcIngestionFor(writeMode filament.WriteMode) filament.IngestionType {
	if writeMode == filament.WriteMerge {
		return filament.IngestionCDCMerge
	}
	return filament.IngestionCDCAppend
}

func compileIngestionTypes(group *routeGroup, cdc bool) (map[string]filament.IngestionType, error) {
	types := make(map[string]filament.IngestionType, len(group.readModes))
	for resource, readMode := range group.readModes {
		if cdc {
			types[resource] = cdcIngestionFor(group.writeMode)
			continue
		}
		ingestionType, err := filament.IngestionFor(readMode, group.writeMode)
		if err != nil {
			return nil, err
		}
		types[resource] = ingestionType
	}
	return types, nil
}

// resolveNodeRef builds the run-time Ref for a pipeline node from the reusable
// Connection it references, with the node's config/secret_refs shallow-merged
// on top as the PIPELINE overlay (node keys win). connection_id is required.
// It rejects an overlay that tries to set a CONNECTION-scoped field.
func (c *Compiler) resolveNodeRef(node *ingestionv1.PipelineNode, connections map[string]filament.Connection) (filament.Ref, error) {
	conn, ok := connections[node.GetConnectionId()]
	if !ok {
		return filament.Ref{}, fmt.Errorf("%w: node %q references missing connection %q", ErrInvalid, node.GetId(), node.GetConnectionId())
	}
	overlay := structMap(node.GetConfig())
	if schema, err := c.schemaFor(conn.Kind, conn.Connector); err == nil {
		if err := ValidateOverlayConfig(schema, overlay); err != nil {
			return filament.Ref{}, fmt.Errorf("%w: node %q: %v", ErrInvalid, node.GetId(), err)
		}
	}
	return filament.Ref{
		Connector:  conn.Connector,
		Config:     MergeConfig(conn.Config, overlay),
		SecretRefs: mergeStrings(conn.SecretRefs, node.GetSecretRefs()),
	}, nil
}

// schemaFor resolves a connector's config schema by kind + name from the
// source and sink registries.
func (c *Compiler) schemaFor(kind filament.ConnectorKind, connector string) (filament.ConfigSchema, error) {
	switch kind {
	case filament.ConnectorKindSource:
		src, err := c.Sources.Resolve(connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return src.Spec().Config, nil
	case filament.ConnectorKindSink:
		sink, err := c.Sinks.Resolve(connector)
		if err != nil {
			return filament.ConfigSchema{}, err
		}
		return sink.Spec().Config, nil
	default:
		return filament.ConfigSchema{}, fmt.Errorf("connector kind is required")
	}
}

// MergeConfig shallow-merges overlay over base; overlay keys win. base is not
// mutated.
func MergeConfig(base, overlay map[string]any) map[string]any {
	out := make(map[string]any, len(base)+len(overlay))
	maps.Copy(out, base)
	maps.Copy(out, overlay)
	return out
}

func mergeStrings(base, overlay map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(overlay))
	maps.Copy(out, base)
	maps.Copy(out, overlay)
	return out
}

func structMap(s *structpb.Struct) map[string]any {
	if s == nil {
		return map[string]any{}
	}
	return s.AsMap()
}
