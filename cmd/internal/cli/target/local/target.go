// Package local serves the CLI's local mode. The full filament stack runs
// embedded in the CLI process and the remote adapter fronts it; this target
// wraps that adapter to keep the YAML document as the durable authoring
// surface. Reads and runs go to the embedded deployment; mutations land in
// the YAML first and mirror to the deployment, whose state dies with the
// process.
package local

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	remotetarget "github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
)

// Target adapts the embedded deployment to local-mode semantics.
type Target struct {
	*remotetarget.Target
	store Store
}

// NewTarget wraps the embedded deployment's adapter with YAML authoring.
func NewTarget(store Store, api *remotetarget.Target) *Target {
	return &Target{Target: api, store: store}
}

// Apply pushes the YAML document into the embedded deployment: connections
// first, then the pipelines that reference them. The deployment starts empty
// every invocation, so apply is create-only. YAML keeps secrets as config
// values — env:NAME references or plaintext — so apply rebuilds the secret
// refs the deployment API expects before each create.
func (t *Target) Apply(ctx context.Context) error {
	doc, _, _, err := t.store.LoadSnapshot()
	if err != nil {
		return err
	}
	catalog, err := t.Catalog(ctx)
	if err != nil {
		return err
	}
	for kind, connections := range map[string]map[string]model.Connection{
		"source": doc.Sources, "sink": doc.Sinks,
	} {
		for name, connection := range connections {
			refs, err := cliapp.UpdateSecretReferences(connectionSchema(catalog, kind, connection.Type), cliapp.ConfigPatch{Values: connection.Config}, nil)
			if err != nil {
				return fmt.Errorf("apply %s %q: %w", kind, name, err)
			}
			connection.SecretRefs = refs
			if _, err := t.Target.CreateConnection(ctx, kind, name, connection); err != nil {
				return fmt.Errorf("apply %s %q: %w", kind, name, err)
			}
		}
	}
	for name, pipeline := range doc.Pipelines {
		source := doc.Sources[pipeline.Source.Ref]
		sink := doc.Sinks[pipeline.Sink.Ref]
		pipeline.Source.SecretRefs, err = cliapp.UpdateSecretReferences(connectionSchema(catalog, "source", source.Type), cliapp.ConfigPatch{Values: pipeline.Source.Config}, nil)
		if err == nil {
			pipeline.Sink.SecretRefs, err = cliapp.UpdateSecretReferences(connectionSchema(catalog, "sink", sink.Type), cliapp.ConfigPatch{Values: pipeline.Sink.Config}, nil)
		}
		if err != nil {
			return fmt.Errorf("apply pipeline %q: %w", name, err)
		}
		if _, err := t.Target.CreatePipeline(ctx, name, pipeline); err != nil {
			return fmt.Errorf("apply pipeline %q: %w", name, err)
		}
	}
	return nil
}

func connectionSchema(catalog model.Catalog, kind, connector string) filament.ConfigSchema {
	if kind == "sink" {
		return catalog.Sinks[connector].Config
	}
	return catalog.Sources[connector].Config
}

// CreateConnection writes the connection to the YAML document and mirrors it
// to the embedded deployment.
func (t *Target) CreateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	if err := validateConnectionKind(kind); err != nil {
		return model.Connection{}, err
	}
	if err := t.store.Create(kind+"s", name, connection); err != nil {
		return model.Connection{}, err
	}
	return t.Target.CreateConnection(ctx, kind, name, connection)
}

// UpdateConnection replaces the connection on the embedded deployment, whose
// optimistic lock is authoritative, then lands it in the YAML document.
func (t *Target) UpdateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	if err := validateConnectionKind(kind); err != nil {
		return model.Connection{}, err
	}
	updated, err := t.Target.UpdateConnection(ctx, kind, name, connection)
	if err != nil {
		return model.Connection{}, err
	}
	if err := t.updateYAML(kind+"s", name, connection); err != nil {
		return model.Connection{}, err
	}
	return updated, nil
}

// DeleteConnection removes the connection from the embedded deployment, then
// from the YAML document.
func (t *Target) DeleteConnection(ctx context.Context, kind, name string, metadata model.EntityMetadata) error {
	if err := validateConnectionKind(kind); err != nil {
		return err
	}
	if err := t.Target.DeleteConnection(ctx, kind, name, metadata); err != nil {
		return err
	}
	return t.deleteYAML(kind+"s", name)
}

// CreatePipeline writes the pipeline to the YAML document and mirrors it to
// the embedded deployment.
func (t *Target) CreatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	if err := t.store.Create("pipelines", name, pipeline); err != nil {
		return model.Pipeline{}, err
	}
	return t.Target.CreatePipeline(ctx, name, pipeline)
}

// UpdatePipeline replaces the pipeline on the embedded deployment, then lands
// it in the YAML document.
func (t *Target) UpdatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	updated, err := t.Target.UpdatePipeline(ctx, name, pipeline)
	if err != nil {
		return model.Pipeline{}, err
	}
	if err := t.updateYAML("pipelines", name, pipeline); err != nil {
		return model.Pipeline{}, err
	}
	return updated, nil
}

// DeletePipeline removes the pipeline from the embedded deployment, then from
// the YAML document.
func (t *Target) DeletePipeline(ctx context.Context, name string, metadata model.EntityMetadata) error {
	if err := t.Target.DeletePipeline(ctx, name, metadata); err != nil {
		return err
	}
	return t.deleteYAML("pipelines", name)
}

// PushConnection sends a connection to the embedded deployment without
// touching the YAML document. Deployment state dies with the process, so
// run-scoped entities need no cleanup.
func (t *Target) PushConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	return t.Target.CreateConnection(ctx, kind, name, connection)
}

// PushPipeline creates or replaces a pipeline on the embedded deployment
// without touching the YAML document.
func (t *Target) PushPipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	if _, err := t.GetPipeline(ctx, name); err != nil {
		return t.Target.CreatePipeline(ctx, name, pipeline)
	}
	return t.Target.UpdatePipeline(ctx, name, pipeline)
}

// updateYAML replaces a document entry under the store's own current
// revision. Callers hold deployment metadata, which never gates the file.
func (t *Target) updateYAML(section, name string, value any) error {
	_, _, revision, err := t.store.LoadSnapshot()
	if err != nil {
		return err
	}
	return t.store.Update(section, name, value, revision)
}

func (t *Target) deleteYAML(section, name string) error {
	_, _, revision, err := t.store.LoadSnapshot()
	if err != nil {
		return err
	}
	return t.store.Delete(section, name, revision)
}

func validateConnectionKind(kind string) error {
	if kind != "source" && kind != "sink" {
		return fmt.Errorf("unknown connection kind %q", kind)
	}
	return nil
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
