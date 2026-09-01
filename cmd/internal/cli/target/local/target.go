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

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/registry"
	"gopkg.in/yaml.v3"
)

var _ cliapp.Target = (*Target)(nil)

// Target implements CLI operations over a local YAML store and the in-process
// Filament runtime.
type Target struct {
	store   Store
	catalog model.Catalog
}

// NewTarget constructs a local target.
func NewTarget(store Store, catalog model.Catalog) *Target {
	return &Target{store: store, catalog: catalog}
}

// Catalog returns connector metadata compiled into the local CLI.
func (t *Target) Catalog(_ context.Context) (model.Catalog, error) {
	return t.catalog, nil
}

// Configuration loads the local configuration document.
func (t *Target) Configuration(_ context.Context) (model.Document, error) {
	doc, _, err := t.store.Load()
	return doc, err
}

// PutConnection adds or replaces a local connection.
func (t *Target) PutConnection(_ context.Context, kind, name string, connection model.Connection) error {
	if kind != "source" && kind != "sink" {
		return fmt.Errorf("unknown connection kind %q", kind)
	}
	return t.store.Put(kind+"s", name, connection)
}

// DeleteConnection removes a local connection.
func (t *Target) DeleteConnection(_ context.Context, kind, name string) error {
	if kind != "source" && kind != "sink" {
		return fmt.Errorf("unknown connection kind %q", kind)
	}
	return t.store.Delete(kind+"s", name)
}

// PutPipeline adds or replaces a local pipeline.
func (t *Target) PutPipeline(_ context.Context, name string, pipeline model.Pipeline) error {
	return t.store.Put("pipelines", name, pipeline)
}

// DeletePipeline removes a local pipeline.
func (t *Target) DeletePipeline(_ context.Context, name string) error {
	return t.store.Delete("pipelines", name)
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
