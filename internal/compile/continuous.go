package compile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"slices"
)

func (c *Compiler) compileContinuous(ctx context.Context, tenant filament.TenantID, p *ingestionv1.Pipeline, v *ingestionv1.PipelineVersion, token string, options filament.RunOptions, worker filament.WorkerConfiguration) ([]CompiledRun, error) {
	if v.GetGraph() == nil {
		return nil, fmt.Errorf("%w: continuous pipeline needs a graph", ErrInvalid)
	}
	graph := proto.Clone(v.GetGraph()).(*ingestionv1.PipelineGraph)
	nodes := map[string]*ingestionv1.PipelineNode{}
	connections := map[string]filament.Connection{}
	for _, n := range graph.Nodes {
		if n == nil || n.Id == "" || nodes[n.Id] != nil {
			return nil, fmt.Errorf("%w: invalid or duplicate graph node", ErrInvalid)
		}
		nodes[n.Id] = n
		conn, err := c.Store.LoadConnection(ctx, tenant, n.ConnectionId)
		if err != nil {
			return nil, err
		}
		if conn.DeletedAt != 0 {
			return nil, fmt.Errorf("%w: deleted connection", ErrPrecondition)
		}
		connections[n.ConnectionId] = conn
	}
	for _, e := range graph.Edges {
		if e == nil {
			return nil, fmt.Errorf("%w: nil graph edge", ErrInvalid)
		}
		if e.Selector != "" && e.Selector != e.Resource {
			return nil, fmt.Errorf("%w: continuous execution requires fixed resources, not selectors", ErrInvalid)
		}
		if e.ReadMode != ingestionv1.ReadMode_READ_MODE_UNSPECIFIED || len(e.Cursors) > 0 {
			return nil, fmt.Errorf("%w: continuous execution does not accept bounded read modes or cursors", ErrInvalid)
		}
		if e.WriteMode == ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED {
			e.WriteMode = ingestionv1.WriteMode_WRITE_MODE_APPEND
		}
		if e.WriteMode != ingestionv1.WriteMode_WRITE_MODE_APPEND {
			return nil, fmt.Errorf("%w: continuous execution requires append", ErrInvalid)
		}
	}
	groups, err := groupEdges(graph.Edges, nodes)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return nil, fmt.Errorf("%w: continuous pipeline needs a route", ErrInvalid)
	}
	var out []CompiledRun
	for _, group := range groups {
		if connections[group.source.ConnectionId].Kind != filament.ConnectorKindSource || connections[group.sink.ConnectionId].Kind != filament.ConnectorKindSink {
			return nil, fmt.Errorf("%w: route must connect a source to a sink", ErrInvalid)
		}
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
		sink, err := c.Sinks.Resolve(sinkRef.Connector)
		if err != nil {
			return nil, err
		}
		if err := filament.ValidateContinuousConnectors(source, sink); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrPrecondition, err)
		}
		resources, _ := routeResources(group)
		streamID := uuid.NewString()
		plan, err := planContinuousSource(source, sourceRef, resources, streamID)
		if err != nil {
			return nil, err
		}
		resources = plan.Resources
		// Conservative v2 fingerprint: no guessing that an edited endpoint/config is
		// compatible. A changed fingerprint requires a new generation after stop.
		raw, err := json.Marshal(struct {
			Source, Sink filament.Ref
			Resources    []string
			Mode         string
		}{sourceRef, sinkRef, resources, "continuous-append-v2"})
		if err != nil {
			return nil, fmt.Errorf("%w: fingerprint stream: %v", ErrInvalid, err)
		}
		hash := sha256.Sum256(raw)
		desired := &filament.ReplicationStream{ID: streamID, Tenant: tenant, PipelineID: p.Id, Route: group.key, SourceConnectionID: group.source.ConnectionId, SinkConnectionID: group.sink.ConnectionId, ConsumerName: plan.ConsumerName, ConsumerConfig: plan.ConsumerConfig, ContinuityFingerprint: hex.EncodeToString(hash[:]), CreatedFromPipelineVersionID: v.Id}
		out = append(out, CompiledRun{Edge: group.key, Submission: filament.RunSubmission{DesiredReplicationStream: desired, Request: filament.RunRequest{Tenant: tenant, PipelineID: p.Id, PipelineVersionID: v.Id, IdempotencyKey: fmt.Sprintf("%s:%s:%s", p.Id, token, group.key), Source: sourceRef, Sink: sinkRef, SourceConnectionID: group.source.ConnectionId, SinkConnectionID: group.sink.ConnectionId, Resources: resources, CheckpointRoute: group.key, Options: options, WorkerConfiguration: worker.Merge(WorkerConfigurationFromProto(p.WorkerConfiguration)), IngestionTypes: continuousIngestionTypes(resources)}}})
	}
	return out, nil
}

// PlanContinuousSource delegates provider config and fixed membership to the
// source planner. API validation and run compilation use this same pure path.
func PlanContinuousSource(source filament.Source, ref filament.Ref, resources []string) (filament.ReplicationStreamPlan, error) {
	return planContinuousSource(source, ref, resources, uuid.NewString())
}

func planContinuousSource(source filament.Source, ref filament.Ref, resources []string, streamID string) (filament.ReplicationStreamPlan, error) {
	planner, ok := source.(filament.ReplicationStreamPlanner)
	if !ok {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("%w: source cannot plan durable stream admission", ErrPrecondition)
	}
	plan, err := planner.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{ReplicationStreamID: streamID, Config: filament.NewConfig(ref.Config), Resources: resources})
	if err != nil {
		return plan, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if plan.ConsumerName == "" || len(plan.Resources) == 0 {
		return plan, fmt.Errorf("%w: source must resolve a consumer and fixed resources", ErrInvalid)
	}
	plan.Resources = slices.Clone(plan.Resources)
	slices.Sort(plan.Resources)
	for i, r := range plan.Resources {
		if r == "" || (i > 0 && plan.Resources[i-1] == r) {
			return plan, fmt.Errorf("%w: invalid planned resources", ErrInvalid)
		}
	}
	if len(resources) > 0 {
		requested := slices.Clone(resources)
		slices.Sort(requested)
		if !slices.Equal(requested, plan.Resources) {
			return plan, fmt.Errorf("%w: source changed requested membership", ErrInvalid)
		}
	}
	return plan, nil
}
func continuousIngestionTypes(resources []string) map[string]filament.IngestionType {
	out := make(map[string]filament.IngestionType, len(resources))
	for _, r := range resources {
		out[r] = filament.IngestionFullAppend
	}
	return out
}
