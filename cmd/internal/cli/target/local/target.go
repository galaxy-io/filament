// Package local implements CLI target operations backed by a local YAML
// document and the in-process Filament runtime.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/registry"
)

var _ cliapp.Target = (*Target)(nil)

// Target implements CLI operations over a local YAML store and the in-process
// Filament runtime.
type Target struct {
	store   Store
	catalog model.Catalog
	runMu   sync.Mutex
	runs    map[string]*runSession
}

// NewTarget constructs a local target.
func NewTarget(store Store, catalog model.Catalog) *Target {
	return &Target{store: store, catalog: catalog, runs: make(map[string]*runSession)}
}

// Catalog returns connector metadata compiled into the local CLI.
func (t *Target) Catalog(_ context.Context) (model.Catalog, error) {
	return t.catalog, nil
}

// ListConnections returns named connections with target-owned metadata.
func (t *Target) ListConnections(_ context.Context, kind string) ([]model.NamedConnection, error) {
	if err := validateConnectionKind(kind); err != nil {
		return nil, err
	}
	doc, _, revision, err := t.store.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	connections := doc.Sources
	if kind == "sink" {
		connections = doc.Sinks
	}
	names := make([]string, 0, len(connections))
	for name := range connections {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]model.NamedConnection, 0, len(names))
	for _, name := range names {
		connection := connections[name]
		connection.Metadata = entityMetadata(kind, name, revision)
		result = append(result, model.NamedConnection{Kind: kind, Name: name, Connection: connection})
	}
	return result, nil
}

// GetConnection returns one named connection with target-owned metadata.
func (t *Target) GetConnection(ctx context.Context, kind, name string) (model.Connection, error) {
	connections, err := t.ListConnections(ctx, kind)
	if err != nil {
		return model.Connection{}, err
	}
	for _, item := range connections {
		if item.Name == name {
			return item.Connection, nil
		}
	}
	return model.Connection{}, fmt.Errorf("%s %q does not exist", kind, name)
}

// CreateConnection creates a connection and rejects duplicate names.
func (t *Target) CreateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	if err := validateConnectionKind(kind); err != nil {
		return model.Connection{}, err
	}
	if err := t.store.Create(kind+"s", name, connection); err != nil {
		return model.Connection{}, err
	}
	return t.GetConnection(ctx, kind, name)
}

// UpdateConnection replaces a connection using optimistic concurrency.
func (t *Target) UpdateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	if err := validateConnectionKind(kind); err != nil {
		return model.Connection{}, err
	}
	if err := t.store.Update(kind+"s", name, connection, connection.Metadata.Revision); err != nil {
		return model.Connection{}, err
	}
	return t.GetConnection(ctx, kind, name)
}

// DeleteConnection removes a connection using optimistic concurrency.
func (t *Target) DeleteConnection(_ context.Context, kind, name string, metadata model.EntityMetadata) error {
	if err := validateConnectionKind(kind); err != nil {
		return err
	}
	return t.store.Delete(kind+"s", name, metadata.Revision)
}

// ListPipelines returns named pipelines with target-owned metadata.
func (t *Target) ListPipelines(_ context.Context) ([]model.NamedPipeline, error) {
	doc, _, revision, err := t.store.LoadSnapshot()
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(doc.Pipelines))
	for name := range doc.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]model.NamedPipeline, 0, len(names))
	for _, name := range names {
		pipeline := doc.Pipelines[name]
		pipeline.Metadata = entityMetadata("pipeline", name, revision)
		result = append(result, model.NamedPipeline{Name: name, Pipeline: pipeline})
	}
	return result, nil
}

// GetPipeline returns one named pipeline with target-owned metadata.
func (t *Target) GetPipeline(ctx context.Context, name string) (model.Pipeline, error) {
	pipelines, err := t.ListPipelines(ctx)
	if err != nil {
		return model.Pipeline{}, err
	}
	for _, item := range pipelines {
		if item.Name == name {
			return item.Pipeline, nil
		}
	}
	return model.Pipeline{}, fmt.Errorf("pipeline %q does not exist", name)
}

// CreatePipeline creates a pipeline and rejects duplicate names.
func (t *Target) CreatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	if err := t.store.Create("pipelines", name, pipeline); err != nil {
		return model.Pipeline{}, err
	}
	return t.GetPipeline(ctx, name)
}

// UpdatePipeline replaces a pipeline using optimistic concurrency.
func (t *Target) UpdatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	if err := t.store.Update("pipelines", name, pipeline, pipeline.Metadata.Revision); err != nil {
		return model.Pipeline{}, err
	}
	return t.GetPipeline(ctx, name)
}

// DeletePipeline removes a pipeline using optimistic concurrency.
func (t *Target) DeletePipeline(_ context.Context, name string, metadata model.EntityMetadata) error {
	return t.store.Delete("pipelines", name, metadata.Revision)
}

// ValidateConfiguration applies the local runtime's authoritative validation.
func (t *Target) ValidateConfiguration(_ context.Context, document model.Document) error {
	return cliapp.ValidateDocument(document, t.catalog)
}

func validateConnectionKind(kind string) error {
	if kind != "source" && kind != "sink" {
		return fmt.Errorf("unknown connection kind %q", kind)
	}
	return nil
}

func entityMetadata(kind, name, revision string) model.EntityMetadata {
	return model.EntityMetadata{ID: "local/" + kind + "/" + name, Revision: revision}
}

// Discover executes resource discovery using the local connector registry.
func (t *Target) Discover(ctx context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	source, err := registry.DefaultSources.Resolve(request.Connector)
	if err != nil {
		return model.ResourceList{}, fmt.Errorf("resolve source connector %q: %w", request.Connector, err)
	}
	discoverable, ok := source.(filament.Discoverable)
	if !ok {
		return model.ResourceList{}, fmt.Errorf("source connector %q does not support resource discovery", request.Connector)
	}
	spec, ok := t.catalog.Sources[request.Connector]
	if !ok {
		return model.ResourceList{}, fmt.Errorf("unknown source connector %q", request.Connector)
	}
	config, err := cliapp.ResolveConfigSecrets(spec.Config, request.Config, os.LookupEnv)
	if err != nil {
		return model.ResourceList{}, fmt.Errorf("source %q: %w", request.Source, err)
	}
	if err := source.Configure(ctx, filament.NewConfig(config)); err != nil {
		return model.ResourceList{}, fmt.Errorf("configure source %q: %w", request.Source, err)
	}
	defer func() { _ = source.Teardown(ctx) }()

	result, err := discoverable.Discover(ctx, filament.DiscoverOpts{Refresh: request.Refresh})
	if err != nil {
		return model.ResourceList{}, fmt.Errorf("discover source %q: %w", request.Source, err)
	}
	sort.Slice(result.Resources, func(i, j int) bool {
		return result.Resources[i].Name < result.Resources[j].Name
	})
	resources := model.ResourceList{Source: request.Source, Items: make([]model.ResourceSummary, 0, len(result.Resources))}
	for _, resource := range result.Resources {
		resources.Items = append(resources.Items, model.ResourceSummary{
			Name: resource.Name, DisplayName: resource.DisplayName, Selectable: resource.Selectable,
			PrimaryKey: append([]string(nil), resource.PrimaryKey...), EstimatedRows: resource.Estimated,
		})
	}
	return resources, nil
}

// ConfigurationLocation returns the local YAML path.
func (t *Target) ConfigurationLocation() string { return t.store.Path }

// ReadConfiguration returns the exact local YAML bytes, or an initialized
// document when the file does not exist.
func (t *Target) ReadConfiguration(_ context.Context) ([]byte, error) {
	data, err := os.ReadFile(t.store.Path)
	if err == nil {
		return data, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	encoder.SetIndent(2)
	if err := encoder.Encode(model.NewDocument()); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// WriteConfiguration atomically writes validated YAML bytes.
func (t *Target) WriteConfiguration(_ context.Context, data []byte) error {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("parse YAML tree: %w", err)
	}
	return t.store.Write(&root)
}
