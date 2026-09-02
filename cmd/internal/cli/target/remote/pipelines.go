package remote

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

var readModeFromString = map[string]ingestionv1.ReadMode{
	"":            ingestionv1.ReadMode_READ_MODE_UNSPECIFIED,
	"full":        ingestionv1.ReadMode_READ_MODE_FULL,
	"incremental": ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
	"cdc":         ingestionv1.ReadMode_READ_MODE_UNSPECIFIED,
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
	items, err := t.allPipelines(ctx)
	if err != nil {
		return nil, err
	}
	connections, err := t.connectionsByID(ctx)
	if err != nil {
		return nil, err
	}
	pipelines := make([]model.NamedPipeline, 0, len(items))
	for _, item := range items {
		pipelines = append(pipelines, model.NamedPipeline{
			Name: item.GetName(), Pipeline: pipelineFromProto(item, connections),
		})
	}
	return pipelines, nil
}

// ListPipelinesPage returns one deployment-native cursor page.
func (t *Target) ListPipelinesPage(ctx context.Context, request model.PageRequest) (model.Page[model.NamedPipeline], error) {
	items, pagination, err := t.pipelinePage(ctx, request)
	if err != nil {
		return model.Page[model.NamedPipeline]{}, err
	}
	connections, err := t.connectionsByID(ctx)
	if err != nil {
		return model.Page[model.NamedPipeline]{}, err
	}
	page := model.Page[model.NamedPipeline]{
		Items: make([]model.NamedPipeline, 0, len(items)), Total: int(pagination.GetTotal()),
		NextCursor: pagination.GetNextCursor(), PreviousCursor: pagination.GetPreviousCursor(),
	}
	for _, item := range items {
		page.Items = append(page.Items, model.NamedPipeline{
			Name: item.GetName(), Pipeline: pipelineFromProto(item, connections),
		})
	}
	return page, nil
}

// GetPipeline returns one pipeline by name.
func (t *Target) GetPipeline(ctx context.Context, name string) (model.Pipeline, error) {
	item, err := t.findPipeline(ctx, name)
	if err != nil {
		return model.Pipeline{}, err
	}
	connections, err := t.connectionsByID(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	return pipelineFromProto(item, connections), nil
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
		cleanupErr := t.deletePipelineAfterFailedCreate(ctx, pipelineID)
		return model.Pipeline{}, errors.Join(t.rpcError(err), cleanupErr)
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
	connections, err := t.connectionsByID(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	currentPipeline := pipelineFromProto(existing, connections)
	if reason := currentPipeline.Info.EditBlockedReason; reason != "" {
		return model.Pipeline{}, fmt.Errorf("pipeline %q cannot be edited by the CLI without losing data: %s; edit it in the UI", name, reason)
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

func (t *Target) deletePipelineAfterFailedCreate(ctx context.Context, id string) error {
	_, err := t.client.DeletePipeline(ctx, connect.NewRequest(&ingestionv1.DeletePipelineRequest{Id: id}))
	if err == nil {
		return nil
	}
	return fmt.Errorf("cleanup incomplete; delete pipeline %q manually: %w", id, t.rpcError(err))
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

func (t *Target) allPipelines(ctx context.Context) ([]*ingestionv1.Pipeline, error) {
	var items []*ingestionv1.Pipeline
	cursor := ""
	for {
		page, pagination, err := t.pipelinePage(ctx, model.PageRequest{PageSize: internalPageSize, Cursor: cursor})
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		cursor = pagination.GetNextCursor()
		if cursor == "" {
			return items, nil
		}
	}
}

func (t *Target) pipelinePage(
	ctx context.Context,
	request model.PageRequest,
) ([]*ingestionv1.Pipeline, *ingestionv1.PaginationResponse, error) {
	response, err := t.client.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{
		IncludeLastRun: true, IncludeSchedule: true,
		Pagination: paginationRequest(request.PageSize, request.Cursor),
	}))
	if err != nil {
		return nil, nil, t.rpcError(err)
	}
	return response.Msg.GetPipelines(), response.Msg.GetPagination(), nil
}

func (t *Target) findPipeline(ctx context.Context, name string) (*ingestionv1.Pipeline, error) {
	items, err := t.allPipelines(ctx)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.GetName() == name {
			return item, nil
		}
	}
	return nil, fmt.Errorf("pipeline %q does not exist", name)
}

// buildGraph compiles the flat pipeline into a two-node graph, resolving
// connection names to deployment ids. It also returns the id-to-name map for
// flattening the response.
func (t *Target) buildGraph(ctx context.Context, pipeline model.Pipeline) (*ingestionv1.PipelineGraph, map[string]connectionIdentity, error) {
	graph := pipeline.Graph
	if graph == nil {
		built := model.SimplePipelineGraph(pipeline)
		graph = &built
	}
	connections, err := t.connectionsByName(ctx)
	if err != nil {
		return nil, nil, err
	}
	return graphToProto(*graph, connections)
}

func pipelineFromProto(item *ingestionv1.Pipeline, connections map[string]connectionIdentity) model.Pipeline {
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
	graph := graphFromProto(item.GetCurrentVersion().GetGraph(), connections)
	replication := ""
	for _, node := range item.GetCurrentVersion().GetGraph().GetNodes() {
		if node.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE {
			replication = connections[node.GetConnectionId()].Replication
			break
		}
	}
	if err := model.ProjectSimpleGraph(&pipeline, graph, replication); err != nil {
		pipeline.Info.EditBlockedReason = err.Error()
	}
	return pipeline
}

func readModeString(mode ingestionv1.ReadMode) string {
	switch mode {
	case ingestionv1.ReadMode_READ_MODE_FULL:
		return "full"
	case ingestionv1.ReadMode_READ_MODE_INCREMENTAL:
		return "incremental"
	default:
		return ""
	}
}

func writeModeString(mode ingestionv1.WriteMode) string {
	switch mode {
	case ingestionv1.WriteMode_WRITE_MODE_APPEND:
		return "append"
	case ingestionv1.WriteMode_WRITE_MODE_REPLACE:
		return "replace"
	case ingestionv1.WriteMode_WRITE_MODE_UPSERT:
		return "upsert"
	case ingestionv1.WriteMode_WRITE_MODE_MERGE:
		return "merge"
	default:
		return ""
	}
}

type connectionIdentity struct {
	ID          string
	Name        string
	Kind        string
	Replication string
}

func (t *Target) connectionsByID(ctx context.Context) (map[string]connectionIdentity, error) {
	connections, err := t.allConnections(ctx, ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED)
	if err != nil {
		return nil, err
	}
	out := make(map[string]connectionIdentity, len(connections))
	for _, connection := range connections {
		kind := "source"
		if connection.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK {
			kind = "sink"
		}
		out[connection.GetId()] = connectionIdentity{
			ID: connection.GetId(), Name: connection.GetName(), Kind: kind,
			Replication: replicationString(connection.GetReplication()),
		}
	}
	return out, nil
}

func (t *Target) connectionsByName(ctx context.Context) (map[string]connectionIdentity, error) {
	byID, err := t.connectionsByID(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]connectionIdentity, len(byID))
	for _, connection := range byID {
		out[connection.Kind+"\x00"+connection.Name] = connection
	}
	return out, nil
}

func graphFromProto(graph *ingestionv1.PipelineGraph, connections map[string]connectionIdentity) model.PipelineGraph {
	out := model.PipelineGraph{}
	for _, node := range graph.GetNodes() {
		kind := ""
		switch node.GetKind() {
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE:
			kind = "source"
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK:
			kind = "sink"
		case ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED:
		}
		connection := connections[node.GetConnectionId()].Name
		if connection == "" {
			connection = node.GetConnectionId()
		}
		out.Nodes = append(out.Nodes, model.PipelineGraphNode{
			ID: node.GetId(), Kind: kind, Connection: connection, Config: configMap(node.GetConfig()),
			SecretRefs: cloneStrings(node.GetSecretRefs()),
		})
	}
	for _, edge := range graph.GetEdges() {
		mapped := model.PipelineGraphEdge{
			From: edge.GetFromNode(), To: edge.GetToNode(), Resource: edge.GetResource(),
			Selector: edge.GetSelector(), ReadMode: readModeString(edge.GetReadMode()), WriteMode: writeModeString(edge.GetWriteMode()),
		}
		for _, cursor := range edge.GetCursors() {
			mapped.Cursors = append(mapped.Cursors, model.ResourceCursor{
				Resource: cursor.GetResource(), Field: cursor.GetField(), LookbackSeconds: cursor.GetLookbackSeconds(),
			})
		}
		out.Edges = append(out.Edges, mapped)
	}
	return out
}

func graphToProto(graph model.PipelineGraph, connections map[string]connectionIdentity) (*ingestionv1.PipelineGraph, map[string]connectionIdentity, error) {
	out := &ingestionv1.PipelineGraph{}
	refs := map[string]connectionIdentity{}
	for _, node := range graph.Nodes {
		kind, err := connectorKind(node.Kind)
		if err != nil {
			return nil, nil, err
		}
		connection, ok := connections[node.Kind+"\x00"+node.Connection]
		if !ok {
			return nil, nil, fmt.Errorf("%s %q does not exist", node.Kind, node.Connection)
		}
		config, err := configStruct(cliapp.ConfigWithoutSecretValues(node.Config, node.SecretRefs))
		if err != nil {
			return nil, nil, fmt.Errorf("%s %q: %w", node.Kind, node.Connection, err)
		}
		out.Nodes = append(out.Nodes, &ingestionv1.PipelineNode{
			Id: node.ID, Kind: kind, ConnectionId: connection.ID, Config: config,
			SecretRefs: cloneStrings(node.SecretRefs),
		})
		refs[connection.ID] = connection
	}
	for _, edge := range graph.Edges {
		readMode, ok := readModeFromString[edge.ReadMode]
		if !ok {
			return nil, nil, fmt.Errorf("unknown sync mode %q", edge.ReadMode)
		}
		writeMode, ok := writeModeFromString[edge.WriteMode]
		if !ok {
			return nil, nil, fmt.Errorf("unknown write mode %q", edge.WriteMode)
		}
		mapped := &ingestionv1.PipelineEdge{
			FromNode: edge.From, ToNode: edge.To, Resource: edge.Resource, Selector: edge.Selector,
			ReadMode: readMode, WriteMode: writeMode,
		}
		for _, cursor := range edge.Cursors {
			mapped.Cursors = append(mapped.Cursors, &ingestionv1.ResourceCursorConfig{
				Resource: cursor.Resource, Field: cursor.Field, LookbackSeconds: cursor.LookbackSeconds,
			})
		}
		out.Edges = append(out.Edges, mapped)
	}
	return out, refs, nil
}
