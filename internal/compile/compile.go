// Package compile turns a stored pipeline into per-route run requests. It is
// the shared half of run intake: the server compiles for manual RunPipeline
// calls and the scheduler compiles for cron fires. Errors carry sentinels
// (filament.ErrNotFound, ErrPrecondition, ErrInvalid) instead of transport
// codes; callers map them to their own surface.
package compile

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// ErrPrecondition marks a pipeline that exists but cannot run: it or a
// connection it references is deleted, or it has no version yet.
var ErrPrecondition = errors.New("pipeline not runnable")

// ErrInvalid marks a pipeline definition the compiler rejects: edges with
// conflicting ingestion types or cursors, references to missing nodes or
// connections, or a node overlay that violates field scopes.
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
		sourceRef, err := c.resolveNodeRef(group.source, connections)
		if err != nil {
			return nil, err
		}
		sinkRef, err := c.resolveNodeRef(group.sink, connections)
		if err != nil {
			return nil, err
		}
		source, err := c.Sources.Resolve(sourceRef.Provider)
		if err != nil {
			return nil, err
		}
		ingestionTypes := make(map[string]filament.IngestionType, len(group.syncModes))
		if filament.ReplicationOf(source, filament.NewConfig(sourceRef.Config)) == filament.ReplicationCDC {
			for resource := range group.syncModes {
				ingestionTypes[resource] = filament.IngestionCDC
			}
		} else {
			for resource, mode := range group.syncModes {
				ingestionTypes[resource] = mode.IngestionType()
			}
		}
		compiled = append(compiled, CompiledRun{Edge: key, Req: filament.RunRequest{
			Tenant:              filament.TenantID(tenant),
			PipelineID:          pipeline.GetId(),
			PipelineVersionID:   version.GetId(),
			IdempotencyKey:      fmt.Sprintf("%s:%s:%s", pipeline.GetId(), token, key),
			Source:              sourceRef,
			Sink:                sinkRef,
			SourceConnectionID:  group.source.GetConnectionId(),
			SinkConnectionID:    group.sink.GetConnectionId(),
			Resources:           resources,
			Selectors:           selectors,
			IngestionTypes:      ingestionTypes,
			CheckpointRoute:     key,
			CursorConfigs:       group.cursorConfigs,
			Options:             options,
			ScheduleID:          scheduleID,
			WorkerConfiguration: worker,
		}})
	}
	return compiled, nil
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
		Provider:   conn.Connector,
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
