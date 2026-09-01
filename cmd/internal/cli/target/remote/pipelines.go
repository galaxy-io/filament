package remote

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// The CLI's flat pipeline maps onto a two-node graph: one source node, one
// sink node, and one edge per selected resource (or a single all-resource
// edge). Canvas pipelines with richer topologies still list and run; only
// editing them from the CLI is refused.
const (
	sourceNodeID = "source"
	sinkNodeID   = "sink"
)

var readModeFromString = map[string]ingestionv1.ReadMode{
	"":            ingestionv1.ReadMode_READ_MODE_UNSPECIFIED,
	"full":        ingestionv1.ReadMode_READ_MODE_FULL,
	"incremental": ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
}

var writeModeFromString = map[string]ingestionv1.WriteMode{
	"":        ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED,
	"append":  ingestionv1.WriteMode_WRITE_MODE_APPEND,
	"replace": ingestionv1.WriteMode_WRITE_MODE_REPLACE,
	"upsert":  ingestionv1.WriteMode_WRITE_MODE_UPSERT,
	"merge":   ingestionv1.WriteMode_WRITE_MODE_MERGE,
}

// ListPipelines returns every pipeline flattened to the CLI shape.
func (t *Target) ListPipelines(ctx context.Context) ([]model.NamedPipeline, error) {
	response, err := t.listPipelines(ctx)
	if err != nil {
		return nil, err
	}
	names, err := t.connectionNamesByID(ctx)
	if err != nil {
		return nil, err
	}
	pipelines := make([]model.NamedPipeline, 0, len(response.Msg.GetPipelines()))
	for _, item := range response.Msg.GetPipelines() {
		pipelines = append(pipelines, model.NamedPipeline{
			Name: item.GetName(), Pipeline: pipelineFromProto(item, names),
		})
	}
	return pipelines, nil
}

// GetPipeline returns one pipeline by name.
func (t *Target) GetPipeline(ctx context.Context, name string) (model.Pipeline, error) {
	item, err := t.findPipeline(ctx, name)
	if err != nil {
		return model.Pipeline{}, err
	}
	names, err := t.connectionNamesByID(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	return pipelineFromProto(item, names), nil
}

// CreatePipeline creates pipeline metadata and its first graph version.
func (t *Target) CreatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	graph, refs, err := t.buildGraph(ctx, pipeline)
	if err != nil {
		return model.Pipeline{}, err
	}
	created, err := t.client.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: name}))
	if err != nil {
		return model.Pipeline{}, t.rpcError(err)
	}
	pipelineID := created.Msg.GetPipeline().GetId()
	version, err := t.client.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: pipelineID, Graph: graph,
	}))
	if err != nil {
		_, _ = t.client.DeletePipeline(ctx, connect.NewRequest(&ingestionv1.DeletePipelineRequest{Id: pipelineID}))
		return model.Pipeline{}, t.rpcError(err)
	}
	assembled := created.Msg.GetPipeline()
	assembled.CurrentVersion = version.Msg.GetVersion()
	return pipelineFromProto(assembled, refs), nil
}

// UpdatePipeline replaces a flat pipeline's graph with a new version. The
// read revision must still be current.
func (t *Target) UpdatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	existing, err := t.findPipeline(ctx, name)
	if err != nil {
		return model.Pipeline{}, err
	}
	if len(existing.GetCurrentVersion().GetGraph().GetNodes()) > 2 {
		return model.Pipeline{}, fmt.Errorf("pipeline %q has a canvas topology; edit it in the UI", name)
	}
	if current := revisionOf(existing.GetCurrentVersion().GetVersion()); pipeline.Metadata.Revision != current {
		return model.Pipeline{}, fmt.Errorf("pipeline %q has changed since it was read; fetch it again", name)
	}
	graph, refs, err := t.buildGraph(ctx, pipeline)
	if err != nil {
		return model.Pipeline{}, err
	}
	version, err := t.client.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: existing.GetId(), Graph: graph,
	}))
	if err != nil {
		return model.Pipeline{}, t.rpcError(err)
	}
	existing.CurrentVersion = version.Msg.GetVersion()
	return pipelineFromProto(existing, refs), nil
}

// DeletePipeline removes a pipeline by name.
func (t *Target) DeletePipeline(ctx context.Context, name string, metadata model.EntityMetadata) error {
	id := metadata.ID
	if id == "" {
		existing, err := t.findPipeline(ctx, name)
		if err != nil {
			return err
		}
		id = existing.GetId()
	}
	_, err := t.client.DeletePipeline(ctx, connect.NewRequest(&ingestionv1.DeletePipelineRequest{Id: id}))
	return t.rpcError(err)
}

func (t *Target) listPipelines(ctx context.Context) (*connect.Response[ingestionv1.ListPipelinesResponse], error) {
	response, err := t.client.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{
		IncludeLastRun: true, IncludeSchedule: true,
	}))
	if err != nil {
		return nil, t.rpcError(err)
	}
	return response, nil
}

func (t *Target) findPipeline(ctx context.Context, name string) (*ingestionv1.Pipeline, error) {
	response, err := t.listPipelines(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range response.Msg.GetPipelines() {
		if item.GetName() == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("pipeline %q does not exist", name)
}

// buildGraph compiles the flat pipeline into a two-node graph, resolving
// connection names to deployment ids. It also returns the id-to-name map for
// flattening the response.
func (t *Target) buildGraph(ctx context.Context, pipeline model.Pipeline) (*ingestionv1.PipelineGraph, map[string]string, error) {
	source, err := t.findConnection(ctx, "source", pipeline.Source.Ref)
	if err != nil {
		return nil, nil, err
	}
	sink, err := t.findConnection(ctx, "sink", pipeline.Sink.Ref)
	if err != nil {
		return nil, nil, err
	}
	sourceConfig, err := configStruct(pipeline.Source.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("source %q: %w", pipeline.Source.Ref, err)
	}
	sinkConfig, err := configStruct(pipeline.Sink.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("sink %q: %w", pipeline.Sink.Ref, err)
	}
	readMode, ok := readModeFromString[pipeline.SyncMode]
	if !ok {
		return nil, nil, fmt.Errorf("unknown sync mode %q", pipeline.SyncMode)
	}
	writeMode, ok := writeModeFromString[pipeline.WriteMode]
	if !ok {
		return nil, nil, fmt.Errorf("unknown write mode %q", pipeline.WriteMode)
	}
	edges := []*ingestionv1.PipelineEdge{{
		FromNode: sourceNodeID, ToNode: sinkNodeID, ReadMode: readMode, WriteMode: writeMode,
	}}
	if len(pipeline.Resources) > 0 {
		edges = make([]*ingestionv1.PipelineEdge, 0, len(pipeline.Resources))
		for _, resource := range pipeline.Resources {
			edges = append(edges, &ingestionv1.PipelineEdge{
				FromNode: sourceNodeID, ToNode: sinkNodeID, Resource: resource,
				ReadMode: readMode, WriteMode: writeMode,
			})
		}
	}
	graph := &ingestionv1.PipelineGraph{
		Nodes: []*ingestionv1.PipelineNode{
			{Id: sourceNodeID, Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: source.GetId(), Config: sourceConfig},
			{Id: sinkNodeID, Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: sink.GetId(), Config: sinkConfig},
		},
		Edges: edges,
	}
	refs := map[string]string{source.GetId(): pipeline.Source.Ref, sink.GetId(): pipeline.Sink.Ref}
	return graph, refs, nil
}

// pipelineFromProto flattens a graph pipeline leniently: the first source and
// sink nodes become the CLI refs, so richer canvas topologies still list.
func pipelineFromProto(item *ingestionv1.Pipeline, names map[string]string) model.Pipeline {
	pipeline := model.Pipeline{
		Metadata: model.EntityMetadata{
			ID:       item.GetId(),
			Revision: revisionOf(item.GetCurrentVersion().GetVersion()),
		},
		Info: model.PipelineInfo{
			Schedule:      item.GetSchedule().GetConfig().GetCron(),
			LastRunStatus: runStatusString(item.GetLastRun().GetStatus()),
			LastRunAt:     timeFromMillis(item.GetLastRun().GetStartedAt()),
			CreatedAt:     timeFromMillis(item.GetCreatedAt()),
			UpdatedAt:     timeFromMillis(item.GetUpdatedAt()),
		},
	}
	graph := item.GetCurrentVersion().GetGraph()
	for _, node := range graph.GetNodes() {
		ref := names[node.GetConnectionId()]
		if ref == "" {
			ref = node.GetConnectionId()
		}
		flat := model.PipelineNode{Ref: ref, Config: configMap(node.GetConfig())}
		switch node.GetKind() {
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
			if pipeline.Source.Ref == "" {
				pipeline.Source = flat
			}
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
			if pipeline.Sink.Ref == "" {
				pipeline.Sink = flat
			}
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		}
	}
	for _, edge := range graph.GetEdges() {
		if resource := edge.GetResource(); resource != "" {
			pipeline.Resources = append(pipeline.Resources, resource)
		}
		if pipeline.SyncMode == "" {
			pipeline.SyncMode = readModeString(edge.GetReadMode())
		}
		if pipeline.WriteMode == "" {
			pipeline.WriteMode = writeModeString(edge.GetWriteMode())
		}
	}
	return pipeline
}

func readModeString(mode ingestionv1.ReadMode) string {
	for name, value := range readModeFromString {
		if value == mode && name != "" {
			return name
		}
	}
	return ""
}

func writeModeString(mode ingestionv1.WriteMode) string {
	for name, value := range writeModeFromString {
		if value == mode && name != "" {
			return name
		}
	}
	return ""
}
