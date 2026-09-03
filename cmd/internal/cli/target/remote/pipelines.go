package remote

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

var readModeFromString = map[string]ingestionv1.ReadMode{
	"":                ingestionv1.ReadMode_READ_MODE_UNSPECIFIED,
	"full":            ingestionv1.ReadMode_READ_MODE_FULL,
	"incremental":     ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
	model.SyncModeCDC: ingestionv1.ReadMode_READ_MODE_UNSPECIFIED,
}

var writeModeFromString = map[string]ingestionv1.WriteMode{
	"":        ingestionv1.WriteMode_WRITE_MODE_UNSPECIFIED,
	"append":  ingestionv1.WriteMode_WRITE_MODE_APPEND,
	"replace": ingestionv1.WriteMode_WRITE_MODE_REPLACE,
	"upsert":  ingestionv1.WriteMode_WRITE_MODE_UPSERT,
	"merge":   ingestionv1.WriteMode_WRITE_MODE_MERGE,
}

// ListPipelines returns every pipeline, projected to the simple shape where
// the graph allows it.
func (t *Target) ListPipelines(ctx context.Context) ([]model.NamedPipeline, error) {
	items, err := t.pipelines(ctx, "")
	if err != nil {
		return nil, err
	}
	connections, err := t.connectionIndex(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]model.NamedPipeline, 0, len(items))
	for _, item := range items {
		out = append(out, model.NamedPipeline{Name: item.GetName(), Pipeline: pipelineFromProto(item, connections)})
	}
	return out, nil
}

// GetPipeline returns one pipeline by name.
func (t *Target) GetPipeline(ctx context.Context, name string) (model.Pipeline, error) {
	item, err := t.findPipeline(ctx, name)
	if err != nil {
		return model.Pipeline{}, err
	}
	connections, err := t.connectionIndex(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	return pipelineFromProto(item, connections), nil
}

// CreatePipeline creates the pipeline and its first graph version. The API
// has no single call for both, so a failed version deletes the shell again.
func (t *Target) CreatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	graph, connections, err := t.graphToProto(ctx, pipeline)
	if err != nil {
		return model.Pipeline{}, err
	}
	created, err := t.client.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: name}))
	if err != nil {
		return model.Pipeline{}, t.rpcError(err)
	}
	shell := created.Msg.GetPipeline()
	version, err := t.client.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: shell.GetId(), Graph: graph,
	}))
	if err != nil {
		_, deleteErr := t.client.DeletePipeline(ctx, connect.NewRequest(&ingestionv1.DeletePipelineRequest{Id: shell.GetId()}))
		if deleteErr != nil {
			deleteErr = fmt.Errorf("delete pipeline %q by hand: %w", name, t.rpcError(deleteErr))
		}
		return model.Pipeline{}, errors.Join(t.rpcError(err), deleteErr)
	}
	shell.CurrentVersion = version.Msg.GetVersion()
	return pipelineFromProto(shell, connections), nil
}

// UpdatePipeline stores a new graph version. The read revision must still
// be current and the deployed graph must project to the simple shape.
func (t *Target) UpdatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	existing, err := t.findPipeline(ctx, name)
	if err != nil {
		return model.Pipeline{}, err
	}
	connections, err := t.connectionIndex(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	if reason := pipelineFromProto(existing, connections).Info.EditBlockedReason; reason != "" {
		return model.Pipeline{}, fmt.Errorf("pipeline %q cannot be edited here without losing part of its graph (%s); edit it in the UI", name, reason)
	}
	if current := revisionOf(existing.GetCurrentVersion().GetVersion()); pipeline.Metadata.Revision != current {
		return model.Pipeline{}, fmt.Errorf("pipeline %q has changed since it was read; fetch it again", name)
	}
	graph, connections, err := t.graphToProto(ctx, pipeline)
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
	return pipelineFromProto(existing, connections), nil
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

// findPipeline resolves a name, narrowed server-side by the substring search
// and matched exactly here.
func (t *Target) findPipeline(ctx context.Context, name string) (*ingestionv1.Pipeline, error) {
	items, err := t.pipelines(ctx, name)
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

func (t *Target) pipelines(ctx context.Context, search string) ([]*ingestionv1.Pipeline, error) {
	return drain(func(cursor string) ([]*ingestionv1.Pipeline, *ingestionv1.PaginationResponse, error) {
		response, err := t.client.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{
			IncludeLastRun: true, IncludeSchedule: true, Search: search, Pagination: pagination(cursor),
		}))
		if err != nil {
			return nil, nil, t.rpcError(err)
		}
		return response.Msg.GetPipelines(), response.Msg.GetPagination(), nil
	})
}

// connectionRef is what a graph needs to know about a saved connection.
type connectionRef struct {
	ID          string
	Name        string
	Kind        string
	Replication string
}

// connectionIndex lists every connection once, keyed by id and by kind+name,
// so graph mapping in either direction is a lookup.
type connectionIndex struct {
	byID   map[string]connectionRef
	byName map[string]connectionRef
}

func nameKey(kind, name string) string { return kind + "/" + name }

func (t *Target) connectionIndex(ctx context.Context) (connectionIndex, error) {
	items, err := t.connections(ctx, ingestionv1.ConnectorKind_CONNECTOR_KIND_UNSPECIFIED, "")
	if err != nil {
		return connectionIndex{}, err
	}
	index := connectionIndex{byID: map[string]connectionRef{}, byName: map[string]connectionRef{}}
	for _, item := range items {
		ref := connectionRef{
			ID: item.GetId(), Name: item.GetName(), Kind: kindString(item.GetKind()),
			Replication: replicationString(item.GetReplication()),
		}
		index.byID[ref.ID] = ref
		index.byName[nameKey(ref.Kind, ref.Name)] = ref
	}
	return index, nil
}

func pipelineFromProto(item *ingestionv1.Pipeline, connections connectionIndex) model.Pipeline {
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
	graph, replication := graphFromProto(item.GetCurrentVersion().GetGraph(), connections)
	if err := model.ProjectSimpleGraph(&pipeline, graph, replication); err != nil {
		pipeline.Info.EditBlockedReason = err.Error()
	}
	return pipeline
}

// graphFromProto maps a deployed graph to the model, naming connections and
// reporting the source's replication so CDC projects correctly.
func graphFromProto(graph *ingestionv1.PipelineGraph, connections connectionIndex) (model.PipelineGraph, string) {
	out := model.PipelineGraph{}
	replication := ""
	for _, node := range graph.GetNodes() {
		ref := connections.byID[node.GetConnectionId()]
		name := ref.Name
		if name == "" {
			name = node.GetConnectionId()
		}
		if node.GetKind() == ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE && replication == "" {
			replication = ref.Replication
		}
		out.Nodes = append(out.Nodes, model.PipelineGraphNode{
			ID: node.GetId(), Kind: kindString(node.GetKind()), Connection: name,
			Config: configMap(node.GetConfig()), SecretRefs: maps.Clone(node.GetSecretRefs()),
		})
	}
	for _, edge := range graph.GetEdges() {
		mapped := model.PipelineGraphEdge{
			From: edge.GetFromNode(), To: edge.GetToNode(), Resource: edge.GetResource(), Selector: edge.GetSelector(),
			ReadMode: readModeString(edge.GetReadMode()), WriteMode: writeModeString(edge.GetWriteMode()),
		}
		for _, cursor := range edge.GetCursors() {
			mapped.Cursors = append(mapped.Cursors, model.ResourceCursor{
				Resource: cursor.GetResource(), Field: cursor.GetField(), LookbackSeconds: cursor.GetLookbackSeconds(),
			})
		}
		out.Edges = append(out.Edges, mapped)
	}
	return out, replication
}

// graphToProto builds the deployment graph for a pipeline, resolving
// connection names to ids. Simple pipelines are expanded first.
func (t *Target) graphToProto(ctx context.Context, pipeline model.Pipeline) (*ingestionv1.PipelineGraph, connectionIndex, error) {
	graph := model.SimplePipelineGraph(pipeline)
	connections, err := t.connectionIndex(ctx)
	if err != nil {
		return nil, connectionIndex{}, err
	}
	out := &ingestionv1.PipelineGraph{}
	for _, node := range graph.Nodes {
		kind, err := connectorKind(node.Kind)
		if err != nil {
			return nil, connectionIndex{}, err
		}
		ref, ok := connections.byName[nameKey(node.Kind, node.Connection)]
		if !ok {
			return nil, connectionIndex{}, fmt.Errorf("%s %q does not exist", node.Kind, node.Connection)
		}
		config, err := configStruct(cliapp.ConfigWithoutSecretValues(node.Config, node.SecretRefs))
		if err != nil {
			return nil, connectionIndex{}, fmt.Errorf("%s %q: %w", node.Kind, node.Connection, err)
		}
		out.Nodes = append(out.Nodes, &ingestionv1.PipelineNode{
			Id: node.ID, Kind: kind, ConnectionId: ref.ID, Config: config, SecretRefs: maps.Clone(node.SecretRefs),
		})
	}
	for _, edge := range graph.Edges {
		readMode, ok := readModeFromString[edge.ReadMode]
		if !ok {
			return nil, connectionIndex{}, fmt.Errorf("unknown sync mode %q", edge.ReadMode)
		}
		writeMode, ok := writeModeFromString[edge.WriteMode]
		if !ok {
			return nil, connectionIndex{}, fmt.Errorf("unknown write mode %q", edge.WriteMode)
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
	return out, connections, nil
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

func runStatusString(status ingestionv1.RunStatus) string {
	switch status {
	case ingestionv1.RunStatus_RUN_STATUS_REQUESTED:
		return "requested"
	case ingestionv1.RunStatus_RUN_STATUS_SCHEDULED:
		return "scheduled"
	case ingestionv1.RunStatus_RUN_STATUS_RUNNING:
		return "running"
	case ingestionv1.RunStatus_RUN_STATUS_COMPLETED:
		return "completed"
	case ingestionv1.RunStatus_RUN_STATUS_FAILED:
		return "failed"
	case ingestionv1.RunStatus_RUN_STATUS_CANCELED:
		return "canceled"
	case ingestionv1.RunStatus_RUN_STATUS_PAUSED:
		return "paused"
	case ingestionv1.RunStatus_RUN_STATUS_PARTIAL:
		return "partial"
	default:
		return ""
	}
}
