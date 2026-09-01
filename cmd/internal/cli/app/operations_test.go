package app

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

type memoryTarget struct {
	document model.Document
	catalog  model.Catalog
	runSpec  filament.RunSpec
}

func (target *memoryTarget) Catalog(context.Context) (model.Catalog, error) {
	return target.catalog, nil
}

func (target *memoryTarget) Configuration(context.Context) (model.Document, error) {
	return target.document, nil
}

func (target *memoryTarget) PutConnection(_ context.Context, kind, name string, connection model.Connection) error {
	if kind == "source" {
		target.document.Sources[name] = connection
	} else {
		target.document.Sinks[name] = connection
	}
	return nil
}

func (target *memoryTarget) DeleteConnection(_ context.Context, kind, name string) error {
	if kind == "source" {
		delete(target.document.Sources, name)
	} else {
		delete(target.document.Sinks, name)
	}
	return nil
}

func (target *memoryTarget) PutPipeline(_ context.Context, name string, pipeline model.Pipeline) error {
	target.document.Pipelines[name] = pipeline
	return nil
}

func (target *memoryTarget) DeletePipeline(_ context.Context, name string) error {
	delete(target.document.Pipelines, name)
	return nil
}

func (target *memoryTarget) Discover(_ context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	return model.ResourceList{Source: request.Source}, nil
}

func (target *memoryTarget) Run(_ context.Context, spec filament.RunSpec, _ func(model.RunEvent)) (model.RunResult, error) {
	target.runSpec = spec
	return model.RunResult{Run: "target-run-1", Records: 12, Bytes: 34}, nil
}

func TestTypedOperationsDriveTarget(t *testing.T) {
	t.Parallel()
	target := newMemoryTarget()
	service := NewService(target)
	ctx := context.Background()

	if _, err := service.SaveConnection(ctx, SaveConnectionRequest{
		Create: true, Kind: "source", Name: "input", Connector: "sample",
		Config: ConfigPatch{Values: map[string]any{"token": "env:REMOTE_TOKEN"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveConnection(ctx, SaveConnectionRequest{
		Create: true, Kind: "sink", Name: "output", Connector: "stdout",
	}); err != nil {
		t.Fatal(err)
	}
	resources := []string{"users"}
	if _, err := service.SavePipeline(ctx, SavePipelineRequest{
		Create: true, Name: "copy", Source: "input", Sink: "output",
		Resources: &resources, SyncMode: "full", WriteMode: "replace",
		SourceConfig: ConfigPatch{Values: map[string]any{"rows": 2}},
	}); err != nil {
		t.Fatal(err)
	}

	connections, err := service.Connections(ctx, "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections.Items) != 1 || connections.Items[0].Name != "input" {
		t.Fatalf("connections = %#v", connections)
	}
	pipelines, err := service.Pipelines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelines.Items) != 1 || pipelines.Items[0].Name != "copy" {
		t.Fatalf("pipelines = %#v", pipelines)
	}

	spec, result, err := service.ExecuteRun(ctx, RunRequest{Pipeline: "copy"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Run != "target-run-1" || result.Records != 12 || spec.Source.Connector != "sample" || spec.Sink.Connector != "stdout" {
		t.Fatalf("spec = %#v; result = %#v", spec, result)
	}
	if target.runSpec.PipelineID != "copy" || target.runSpec.Source.Config["rows"] != 2 {
		t.Fatalf("target run spec = %#v", target.runSpec)
	}
	if target.runSpec.Tenant != "" || target.runSpec.Run != "" {
		t.Fatalf("application leaked target identity into run spec: %#v", target.runSpec)
	}
	if target.runSpec.Source.Config["token"] != "env:REMOTE_TOKEN" {
		t.Fatalf("application resolved a target-owned secret: %#v", target.runSpec.Source.Config)
	}
	if err := service.DeleteSavedConnection(ctx, "source", "input"); err == nil {
		t.Fatal("referenced source deletion succeeded")
	}
}

func TestRawConfigurationIsOptional(t *testing.T) {
	t.Parallel()
	service := NewService(newMemoryTarget())
	if service.ConfigurationLocation() != "" {
		t.Fatalf("configuration location = %q", service.ConfigurationLocation())
	}
	if _, err := service.ReadConfiguration(context.Background()); !errors.Is(err, ErrRawConfigurationUnsupported) {
		t.Fatalf("read error = %v", err)
	}
	if err := service.WriteConfiguration(context.Background(), nil); !errors.Is(err, ErrRawConfigurationUnsupported) {
		t.Fatalf("write error = %v", err)
	}
}

func newMemoryTarget() *memoryTarget {
	return &memoryTarget{
		document: model.NewDocument(),
		catalog: model.Catalog{
			Sources: map[string]filament.ConnectorSpec{
				"sample": {
					Name: "sample",
					Config: filament.ConfigSchema{Fields: []filament.ConfigField{
						{Name: "token", Type: filament.FieldSecret, Scope: filament.ScopeConnection},
						{Name: "rows", Type: filament.FieldInt, Scope: filament.ScopePipeline},
					}},
				},
			},
			Sinks: map[string]filament.SinkSpec{
				"stdout": {
					Name: "stdout",
					Capabilities: filament.SinkCapabilities{
						WritePolicies: filament.WriteCapabilities(filament.IngestionFullReplace),
					},
				},
			},
		},
	}
}
