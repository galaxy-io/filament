package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// ApplyIfChanged applies the document when its bytes differ from the hash
// recorded at marker, so one-shot commands skip the round trips when nothing
// changed. Apply is idempotent; the marker only saves work.
func (t *Target) ApplyIfChanged(ctx context.Context, marker string) error {
	data, err := os.ReadFile(t.store.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if previous, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(previous)) == hash {
		return nil
	}
	if err := t.Apply(ctx); err != nil {
		return err
	}
	// #nosec G703 -- marker is derived from the state directory, never from input
	return os.WriteFile(marker, []byte(hash+"\n"), 0o600)
}

// Apply upserts the document's connections and pipelines into the deployment
// by name. Entities the deployment holds but the document does not are left
// alone: the document is an input, not the inventory.
func (t *Target) Apply(ctx context.Context) error {
	document, err := t.store.Load()
	if err != nil {
		return err
	}
	catalog, err := t.Catalog(ctx)
	if err != nil {
		return err
	}
	for _, kind := range []string{"source", "sink"} {
		if err := t.applyConnections(ctx, kind, document.Connections(kind), catalog); err != nil {
			return err
		}
	}
	return t.applyPipelines(ctx, document.Pipelines)
}

func (t *Target) applyConnections(ctx context.Context, kind string, wanted map[string]model.Connection, catalog model.Catalog) error {
	existing, err := model.DrainPages(func(request model.PageRequest) (model.Page[model.NamedConnection], error) {
		return t.ListConnections(ctx, kind, request)
	})
	if err != nil {
		return err
	}
	current := make(map[string]model.Connection, len(existing))
	for _, item := range existing {
		current[item.Name] = item.Connection
	}
	for _, name := range slices.Sorted(maps.Keys(wanted)) {
		desired := wanted[name]
		live, ok := current[name]
		// An unversioned document may have created a pinned HTTP connection.
		// Keep that pin on reapply, even if the catalog default has changed.
		if kind == "source" && ok {
			base, _, versioned := strings.Cut(live.Type, "@")
			_, registered := catalog.Sources[live.Type]
			alias := catalog.Sources[desired.Type].AliasTarget != ""
			if versioned && registered && alias && desired.Type == base {
				desired.Type = live.Type
			}
		}
		schema := catalog.Sources[desired.Type].Config
		if kind == "sink" {
			schema = catalog.Sinks[desired.Type].Config
		}
		refs, err := environmentRefs(schema.Fields, desired.Config, "")
		if err != nil {
			return fmt.Errorf("%s %q: %w", kind, name, err)
		}
		desired.SecretRefs = refs
		if !ok {
			if _, err := t.CreateConnection(ctx, kind, name, desired); err != nil {
				return err
			}
			continue
		}
		if connectionsEquivalent(live, desired) {
			continue
		}
		desired.Metadata = live.Metadata
		if _, err := t.UpdateConnection(ctx, kind, name, desired); err != nil {
			return err
		}
	}
	return nil
}

func (t *Target) applyPipelines(ctx context.Context, wanted map[string]model.Pipeline) error {
	existing, err := model.DrainPages(func(request model.PageRequest) (model.Page[model.NamedPipeline], error) {
		return t.ListPipelines(ctx, request)
	})
	if err != nil {
		return err
	}
	current := make(map[string]model.Pipeline, len(existing))
	for _, item := range existing {
		current[item.Name] = item.Pipeline
	}
	for _, name := range slices.Sorted(maps.Keys(wanted)) {
		desired := wanted[name]
		live, ok := current[name]
		if !ok {
			if _, err := t.CreatePipeline(ctx, name, desired); err != nil {
				return err
			}
			continue
		}
		if model.PipelinesEquivalent(live, desired) {
			continue
		}
		desired.Metadata, desired.Graph = live.Metadata, live.Graph
		if _, err := t.UpdatePipeline(ctx, name, desired); err != nil {
			return err
		}
	}
	return nil
}

// environmentRefs turns the document's env:NAME secret values into references
// the engine resolves from its environment, keyed by config path. Plaintext
// values stay in the config, and the engine stores them as it saves.
func environmentRefs(fields []filament.ConfigField, values map[string]any, parent string) (map[string]string, error) {
	refs := map[string]string{}
	for _, field := range fields {
		value, ok := values[field.Name]
		if !ok {
			continue
		}
		path := field.Name
		if parent != "" {
			path = parent + "." + field.Name
		}
		if nested, ok := value.(map[string]any); ok && len(field.Fields) > 0 {
			inner, err := environmentRefs(field.Fields, nested, path)
			if err != nil {
				return nil, err
			}
			maps.Copy(refs, inner)
			continue
		}
		if !cliapp.IsSecretField(field) {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("field %q must be a string", path)
		}
		if envName, referenced := cliapp.EnvironmentReferenceName(text); referenced {
			refs[path] = envName
		}
	}
	if len(refs) == 0 {
		return nil, nil
	}
	return refs, nil
}

// connectionsEquivalent compares the deployment's copy, which never carries
// secret values, with the document's copy stripped the same way.
func connectionsEquivalent(live, desired model.Connection) bool {
	return live.Type == desired.Type &&
		maps.Equal(live.SecretRefs, desired.SecretRefs) &&
		model.ConfigsEquivalent(live.Config, cliapp.ConfigWithoutSecretValues(desired.Config, desired.SecretRefs))
}
