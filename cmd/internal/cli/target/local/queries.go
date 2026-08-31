package local

import (
	"context"
	"fmt"
	"sort"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Catalog supplies connector metadata compiled into the local CLI.
type Catalog interface {
	Description(kind, connector string) (string, bool)
}

// Queries implements target-neutral read operations over a local YAML store.
type Queries struct {
	store   Store
	catalog Catalog
}

// NewQueries constructs local read operations.
func NewQueries(store Store, catalog Catalog) *Queries {
	return &Queries{store: store, catalog: catalog}
}

// ListConnections lists locally saved source or sink connections.
func (q *Queries) ListConnections(_ context.Context, kind string) (model.ConnectionList, error) {
	doc, _, err := q.store.Load()
	if err != nil {
		return model.ConnectionList{}, err
	}
	connections, err := connectionsOfKind(doc, kind)
	if err != nil {
		return model.ConnectionList{}, err
	}
	result := model.ConnectionList{Kind: kind, Items: make([]model.ConnectionSummary, 0, len(connections))}
	for _, name := range sortedKeys(connections) {
		connection := connections[name]
		description, _ := q.catalog.Description(kind, connection.Type)
		result.Items = append(result.Items, model.ConnectionSummary{
			Name: name, Connector: connection.Type, Description: description,
		})
	}
	return result, nil
}

// ListPipelines lists locally saved pipelines.
func (q *Queries) ListPipelines(_ context.Context) (model.PipelineList, error) {
	doc, _, err := q.store.Load()
	if err != nil {
		return model.PipelineList{}, err
	}
	result := model.PipelineList{Items: make([]model.PipelineSummary, 0, len(doc.Pipelines))}
	for _, name := range sortedKeys(doc.Pipelines) {
		pipeline := doc.Pipelines[name]
		result.Items = append(result.Items, model.PipelineSummary{
			Name:          name,
			Source:        pipeline.Source.Ref,
			Sink:          pipeline.Sink.Ref,
			ResourceCount: len(pipeline.Resources),
			AllResources:  len(pipeline.Resources) == 0,
			SyncMode:      pipeline.SyncMode,
			WriteMode:     pipeline.WriteMode,
		})
	}
	return result, nil
}

func connectionsOfKind(doc Document, kind string) (map[string]Connection, error) {
	switch kind {
	case "source":
		return doc.Sources, nil
	case "sink":
		return doc.Sinks, nil
	default:
		return nil, fmt.Errorf("unknown connection kind %q", kind)
	}
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
