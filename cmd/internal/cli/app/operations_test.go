package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// memoryTarget models an in-process target like the local adapter.
func (t *memoryTarget) ExecutesInProcess() {}

type memoryTarget struct {
	document  model.Document
	catalog   model.Catalog
	runSpec   filament.RunSpec
	runGroup  model.RunGroup
	submitted model.RunSubmission
}

func (target *memoryTarget) Catalog(context.Context) (model.Catalog, error) {
	return target.catalog, nil
}

func (target *memoryTarget) ListConnections(_ context.Context, kind string) ([]model.NamedConnection, error) {
	connections, err := connectionMap(kind, target.document)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(connections))
	for name := range connections {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]model.NamedConnection, 0, len(names))
	for _, name := range names {
		result = append(result, model.NamedConnection{Kind: kind, Name: name, Connection: connections[name]})
	}
	return result, nil
}

func (target *memoryTarget) GetConnection(_ context.Context, kind, name string) (model.Connection, error) {
	connections, err := connectionMap(kind, target.document)
	if err != nil {
		return model.Connection{}, err
	}
	connection, ok := connections[name]
	if !ok {
		return model.Connection{}, fmt.Errorf("%s %q does not exist", kind, name)
	}
	return connection, nil
}

func (target *memoryTarget) CreateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	connections, err := connectionMap(kind, target.document)
	if err != nil {
		return model.Connection{}, err
	}
	if _, exists := connections[name]; exists {
		return model.Connection{}, fmt.Errorf("%s %q already exists", kind, name)
	}
	connection.Metadata = model.EntityMetadata{ID: kind + "/" + name, Revision: "1"}
	connections[name] = connection
	return target.GetConnection(ctx, kind, name)
}

func (target *memoryTarget) UpdateConnection(ctx context.Context, kind, name string, connection model.Connection) (model.Connection, error) {
	connections, err := connectionMap(kind, target.document)
	if err != nil {
		return model.Connection{}, err
	}
	if _, exists := connections[name]; !exists {
		return model.Connection{}, fmt.Errorf("%s %q does not exist", kind, name)
	}
	if kind == "source" {
		target.document.Sources[name] = connection
	} else {
		target.document.Sinks[name] = connection
	}
	return target.GetConnection(ctx, kind, name)
}

func (target *memoryTarget) DeleteConnection(_ context.Context, kind, name string, _ model.EntityMetadata) error {
	if kind == "source" {
		delete(target.document.Sources, name)
	} else {
		delete(target.document.Sinks, name)
	}
	return nil
}

func (target *memoryTarget) ListPipelines(context.Context) ([]model.NamedPipeline, error) {
	names := make([]string, 0, len(target.document.Pipelines))
	for name := range target.document.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]model.NamedPipeline, 0, len(names))
	for _, name := range names {
		result = append(result, model.NamedPipeline{Name: name, Pipeline: target.document.Pipelines[name]})
	}
	return result, nil
}

func (target *memoryTarget) GetPipeline(_ context.Context, name string) (model.Pipeline, error) {
	pipeline, ok := target.document.Pipelines[name]
	if !ok {
		return model.Pipeline{}, fmt.Errorf("pipeline %q does not exist", name)
	}
	return pipeline, nil
}

func (target *memoryTarget) CreatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	if _, exists := target.document.Pipelines[name]; exists {
		return model.Pipeline{}, fmt.Errorf("pipeline %q already exists", name)
	}
	pipeline.Metadata = model.EntityMetadata{ID: "pipeline/" + name, Revision: "1"}
	target.document.Pipelines[name] = pipeline
	return target.GetPipeline(ctx, name)
}

func (target *memoryTarget) UpdatePipeline(ctx context.Context, name string, pipeline model.Pipeline) (model.Pipeline, error) {
	if _, exists := target.document.Pipelines[name]; !exists {
		return model.Pipeline{}, fmt.Errorf("pipeline %q does not exist", name)
	}
	target.document.Pipelines[name] = pipeline
	return target.GetPipeline(ctx, name)
}

func (target *memoryTarget) DeletePipeline(_ context.Context, name string, _ model.EntityMetadata) error {
	delete(target.document.Pipelines, name)
	return nil
}

func (target *memoryTarget) ValidateConfiguration(_ context.Context, document model.Document) error {
	return ValidateDocument(document, target.catalog)
}

func (target *memoryTarget) Discover(_ context.Context, request model.DiscoverRequest) (model.ResourceList, error) {
	return model.ResourceList{Source: request.Source}, nil
}

func (target *memoryTarget) SubmitRun(_ context.Context, submission model.RunSubmission) (model.RunGroup, error) {
	target.submitted = submission
	target.runSpec = submission.Spec
	target.runGroup = model.RunGroup{Runs: []model.RunRef{{ID: "target-run-1", Route: "copy/edge-1"}}}
	return target.runGroup, nil
}

func (target *memoryTarget) TailRun(_ context.Context, group model.RunGroup, _ func(model.RunEvent)) (model.RunResult, error) {
	return model.RunResult{Runs: group.Runs, Status: "complete", Records: 12, Bytes: 34}, nil
}

func (target *memoryTarget) SignalRun(context.Context, model.RunRef, filament.Signal) error {
	return nil
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
	if len(result.Runs) != 1 || result.Runs[0].ID != "target-run-1" || result.Records != 12 || spec.Source.Connector != "sample" || spec.Sink.Connector != "stdout" {
		t.Fatalf("spec = %#v; result = %#v", spec, result)
	}
	if target.runSpec.PipelineID != "copy" || target.runSpec.Source.Config["rows"] != 2 {
		t.Fatalf("target run spec = %#v", target.runSpec)
	}
	if target.submitted.Pipeline == nil || target.submitted.Pipeline.Name != "copy" || target.submitted.Pipeline.Metadata.ID == "" {
		t.Fatalf("target submission = %#v", target.submitted)
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
