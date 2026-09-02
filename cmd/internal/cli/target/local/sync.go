package local

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// Syncer converges the embedded deployment and the YAML document while a
// long-lived local deployment (filament up) runs. Each tick three-way diffs
// both sides against the last converged snapshot: a side that moved wins for
// that entity, and when both moved the deployment wins. UI edits therefore
// land in the YAML and YAML or CLI edits land in the deployment.
type Syncer struct {
	target  *Target
	secrets filament.Secrets
	base    syncSnapshot
}

type syncSnapshot struct {
	connections map[string]model.Connection // key kind+"\x00"+name
	pipelines   map[string]model.Pipeline
}

// NewSyncer starts from the just-applied document, when both sides are equal.
// secrets is the embedded deployment's provider, used to read stored secret
// values back into the YAML representation.
func NewSyncer(target *Target, secrets filament.Secrets) (*Syncer, error) {
	s := &Syncer{target: target, secrets: secrets}
	base, err := s.deploymentSnapshot(context.Background())
	if err != nil {
		return nil, err
	}
	s.base = base
	return s, nil
}

// Tick performs one reconcile pass.
func (s *Syncer) Tick(ctx context.Context) error {
	deployment, err := s.deploymentSnapshot(ctx)
	if err != nil {
		return err
	}
	document, err := s.documentSnapshot()
	if err != nil {
		return err
	}
	next := syncSnapshot{connections: map[string]model.Connection{}, pipelines: map[string]model.Pipeline{}}

	// Pipelines sync before connection deletes so a dropped pipeline releases
	// its connections, and connections sync before pipeline creates so new
	// pipelines can resolve their refs.
	if err := s.syncConnections(ctx, deployment, document, next); err != nil {
		return err
	}
	if err := s.syncPipelines(ctx, deployment, document, next); err != nil {
		return err
	}
	s.base = next
	return nil
}

func (s *Syncer) syncConnections(ctx context.Context, deployment, document, next syncSnapshot) error {
	for key := range union(deployment.connections, document.connections, s.base.connections) {
		kind, name := splitKey(key)
		deployed, onDeployment := deployment.connections[key]
		saved, inDocument := document.connections[key]
		base, inBase := s.base.connections[key]
		switch {
		case onDeployment && (!inBase || !connectionsEqual(deployed, base)) && (!inDocument || !connectionsEqual(deployed, saved)):
			// The deployment moved: write it to the document.
			if err := s.writeConnectionYAML(kind, name, deployed, inDocument); err != nil {
				return err
			}
			next.connections[key] = deployed
		case !onDeployment && inBase && inDocument:
			// Deleted on the deployment: drop it from the document.
			if err := s.target.deleteYAML(kind+"s", name); err != nil {
				return err
			}
		case inDocument && (!onDeployment || (!connectionsEqual(saved, deployed) && inBase && connectionsEqual(deployed, base))):
			// The document moved (or gained an entry): push it to the deployment.
			pushed, err := s.pushConnection(ctx, kind, name, saved, onDeployment, deployed)
			if err != nil {
				return err
			}
			next.connections[key] = pushed
		case onDeployment:
			next.connections[key] = deployed
		}
	}
	return nil
}

func (s *Syncer) syncPipelines(ctx context.Context, deployment, document, next syncSnapshot) error {
	for name := range union(deployment.pipelines, document.pipelines, s.base.pipelines) {
		deployed, onDeployment := deployment.pipelines[name]
		saved, inDocument := document.pipelines[name]
		base, inBase := s.base.pipelines[name]
		switch {
		case onDeployment && (!inBase || !pipelinesEqual(deployed, base)) && (!inDocument || !pipelinesEqual(deployed, saved)):
			if err := s.writePipelineYAML(name, deployed, inDocument); err != nil {
				return err
			}
			next.pipelines[name] = deployed
		case !onDeployment && inBase && inDocument:
			if err := s.target.deleteYAML("pipelines", name); err != nil {
				return err
			}
		case inDocument && (!onDeployment || (!pipelinesEqual(saved, deployed) && inBase && pipelinesEqual(deployed, base))):
			pushed, err := s.pushPipeline(ctx, name, saved, onDeployment, deployed)
			if err != nil {
				return err
			}
			next.pipelines[name] = pushed
		case onDeployment:
			next.pipelines[name] = deployed
		}
	}
	return nil
}

// deploymentSnapshot lists the embedded deployment flattened to the YAML
// shape, with secret values restored so the document representation is
// complete: env-style refs render as env:NAME and stored values are read back
// from the provider.
func (s *Syncer) deploymentSnapshot(ctx context.Context) (syncSnapshot, error) {
	snapshot := syncSnapshot{connections: map[string]model.Connection{}, pipelines: map[string]model.Pipeline{}}
	for _, kind := range []string{"source", "sink"} {
		connections, err := model.DrainPages(func(request model.PageRequest) (model.Page[model.NamedConnection], error) {
			return s.target.ListConnections(ctx, kind, request)
		})
		if err != nil {
			return syncSnapshot{}, err
		}
		for _, item := range connections {
			if strings.HasPrefix(item.Name, model.AdhocPrefix) {
				continue
			}
			connection := item.Connection
			if err := s.restoreSecrets(ctx, connection.Config, connection.SecretRefs); err != nil {
				return syncSnapshot{}, fmt.Errorf("%s %q: %w", kind, item.Name, err)
			}
			snapshot.connections[kind+"\x00"+item.Name] = connection
		}
	}
	pipelines, err := model.DrainPages(func(request model.PageRequest) (model.Page[model.NamedPipeline], error) {
		return s.target.ListPipelines(ctx, request)
	})
	if err != nil {
		return syncSnapshot{}, err
	}
	for _, item := range pipelines {
		pipeline := item.Pipeline
		// Adhoc entities and graphs the flat YAML cannot represent stay
		// deployment-only.
		if strings.HasPrefix(item.Name, model.AdhocPrefix) || pipeline.Info.EditBlockedReason != "" {
			continue
		}
		if err := s.restoreSecrets(ctx, pipeline.Source.Config, pipeline.Source.SecretRefs); err != nil {
			return syncSnapshot{}, fmt.Errorf("pipeline %q: %w", item.Name, err)
		}
		if err := s.restoreSecrets(ctx, pipeline.Sink.Config, pipeline.Sink.SecretRefs); err != nil {
			return syncSnapshot{}, fmt.Errorf("pipeline %q: %w", item.Name, err)
		}
		snapshot.pipelines[item.Name] = pipeline
	}
	return snapshot, nil
}

func (s *Syncer) documentSnapshot() (syncSnapshot, error) {
	doc, _, _, err := s.target.store.LoadSnapshot()
	if err != nil {
		return syncSnapshot{}, err
	}
	snapshot := syncSnapshot{connections: map[string]model.Connection{}, pipelines: map[string]model.Pipeline{}}
	for name, connection := range doc.Sources {
		snapshot.connections["source\x00"+name] = connection
	}
	for name, connection := range doc.Sinks {
		snapshot.connections["sink\x00"+name] = connection
	}
	for name, pipeline := range doc.Pipelines {
		snapshot.pipelines[name] = pipeline
	}
	return snapshot, nil
}

// restoreSecrets folds secret refs back into config values: an env-style ref
// becomes env:NAME and a deployment-minted ref is read from the provider.
func (s *Syncer) restoreSecrets(ctx context.Context, config map[string]any, refs map[string]string) error {
	for path, ref := range refs {
		if !strings.Contains(ref, "/") {
			setConfigValue(config, path, "env:"+ref)
			continue
		}
		secret, err := s.secrets.Read(ctx, ref)
		if err != nil {
			return fmt.Errorf("read secret %q: %w", ref, err)
		}
		setConfigValue(config, path, string(secret.Value))
	}
	return nil
}

func (s *Syncer) writeConnectionYAML(kind, name string, connection model.Connection, exists bool) error {
	if exists {
		return s.target.updateYAML(kind+"s", name, connection)
	}
	return s.target.store.Create(kind+"s", name, connection)
}

func (s *Syncer) writePipelineYAML(name string, pipeline model.Pipeline, exists bool) error {
	if exists {
		return s.target.updateYAML("pipelines", name, pipeline)
	}
	return s.target.store.Create("pipelines", name, pipeline)
}

func (s *Syncer) pushConnection(ctx context.Context, kind, name string, saved model.Connection, exists bool, deployed model.Connection) (model.Connection, error) {
	catalog, err := s.target.Catalog(ctx)
	if err != nil {
		return model.Connection{}, err
	}
	saved.SecretRefs, err = cliapp.UpdateSecretReferences(connectionSchema(catalog, kind, saved.Type), cliapp.ConfigPatch{Values: saved.Config}, nil)
	if err != nil {
		return model.Connection{}, fmt.Errorf("%s %q: %w", kind, name, err)
	}
	if !exists {
		if _, err := s.target.Target.CreateConnection(ctx, kind, name, saved); err != nil {
			return model.Connection{}, err
		}
		return saved, nil
	}
	saved.Metadata = deployed.Metadata
	if _, err := s.target.Target.UpdateConnection(ctx, kind, name, saved); err != nil {
		return model.Connection{}, err
	}
	return saved, nil
}

func (s *Syncer) pushPipeline(ctx context.Context, name string, saved model.Pipeline, exists bool, deployed model.Pipeline) (model.Pipeline, error) {
	if !exists {
		if _, err := s.target.Target.CreatePipeline(ctx, name, saved); err != nil {
			return model.Pipeline{}, err
		}
		return saved, nil
	}
	saved.Metadata = deployed.Metadata
	saved.Graph = nil // flat fields are authoritative for YAML-born edits
	if _, err := s.target.Target.UpdatePipeline(ctx, name, saved); err != nil {
		return model.Pipeline{}, err
	}
	return saved, nil
}

// connectionsEqual compares the YAML-visible projection.
func connectionsEqual(a, b model.Connection) bool {
	return a.Type == b.Type && configsEqual(a.Config, b.Config)
}

// pipelinesEqual compares the YAML-visible projection.
func pipelinesEqual(a, b model.Pipeline) bool {
	return a.Source.Ref == b.Source.Ref && a.Sink.Ref == b.Sink.Ref &&
		a.SyncMode == b.SyncMode && a.WriteMode == b.WriteMode &&
		reflect.DeepEqual(normalizeList(a.Resources), normalizeList(b.Resources)) &&
		configsEqual(a.Source.Config, b.Source.Config) && configsEqual(a.Sink.Config, b.Sink.Config)
}

func configsEqual(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}

func normalizeList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	return values
}

func setConfigValue(config map[string]any, path, value string) {
	parts := strings.Split(path, ".")
	for _, part := range parts[:len(parts)-1] {
		nested, ok := config[part].(map[string]any)
		if !ok {
			nested = map[string]any{}
			config[part] = nested
		}
		config = nested
	}
	config[parts[len(parts)-1]] = value
}

func splitKey(key string) (kind, name string) {
	kind, name, _ = strings.Cut(key, "\x00")
	return kind, name
}

func union[V any](maps ...map[string]V) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, m := range maps {
		for key := range m {
			keys[key] = struct{}{}
		}
	}
	return keys
}
