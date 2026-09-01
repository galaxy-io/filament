package local

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

func TestTargetConfigurationLifecycle(t *testing.T) {
	t.Parallel()
	target := NewTarget(Store{Path: filepath.Join(t.TempDir(), "filament.yaml")}, model.Catalog{
		Sources: map[string]filament.ConnectorSpec{
			"sample": {Name: "sample", Description: "Synthetic source"},
		},
		Sinks: map[string]filament.SinkSpec{
			"stdout": {Name: "stdout", Description: "Terminal sink"},
		},
	})
	ctx := context.Background()
	service := cliapp.NewService(target)

	if _, err := target.CreateConnection(ctx, "source", "demo", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreatePipeline(ctx, "copy", model.Pipeline{
		Source: model.PipelineNode{Ref: "demo"}, Sink: model.PipelineNode{Ref: "out"},
		Resources: []string{"users"}, SyncMode: "full", WriteMode: "replace",
	}); err != nil {
		t.Fatal(err)
	}

	connections, err := service.Connections(ctx, "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections.Items) != 1 || connections.Items[0].Name != "demo" || connections.Items[0].Description != "Synthetic source" {
		t.Fatalf("connections = %#v", connections)
	}
	pipelines, err := service.Pipelines(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipelines.Items) != 1 || pipelines.Items[0].Name != "copy" || pipelines.Items[0].ResourceCount != 1 {
		t.Fatalf("pipelines = %#v", pipelines)
	}
	data, err := target.ReadConfiguration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "copy:") {
		t.Fatalf("configuration = %s", data)
	}

	pipeline, err := target.GetPipeline(ctx, "copy")
	if err != nil {
		t.Fatal(err)
	}
	if err := target.DeletePipeline(ctx, "copy", pipeline.Metadata); err != nil {
		t.Fatal(err)
	}
	connection, err := target.GetConnection(ctx, "source", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := target.DeleteConnection(ctx, "source", "demo", connection.Metadata); err != nil {
		t.Fatal(err)
	}
	doc, err := service.Configuration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Pipelines) != 0 || len(doc.Sources) != 0 || len(doc.Sinks) != 1 {
		t.Fatalf("configuration = %#v", doc)
	}
}

func TestTargetRejectsDuplicateCreatesAndStaleUpdates(t *testing.T) {
	t.Parallel()
	target := NewTarget(Store{Path: filepath.Join(t.TempDir(), "filament.yaml")}, model.Catalog{})
	ctx := context.Background()

	connection, err := target.CreateConnection(ctx, "source", "demo", model.Connection{Type: "sample"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateConnection(ctx, "source", "demo", model.Connection{Type: "sample"}); err == nil {
		t.Fatal("duplicate create succeeded")
	}
	if _, err := target.CreateConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	connection.Config = map[string]any{"rows": 10}
	if _, err := target.UpdateConnection(ctx, "source", "demo", connection); err == nil || !strings.Contains(err.Error(), "changed since it was read") {
		t.Fatalf("stale update error = %v", err)
	}

	connection, err = target.GetConnection(ctx, "source", "demo")
	if err != nil {
		t.Fatal(err)
	}
	connection.Config = map[string]any{"rows": 10}
	updated, err := target.UpdateConnection(ctx, "source", "demo", connection)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Metadata.ID == "" || updated.Metadata.Revision == connection.Metadata.Revision {
		t.Fatalf("updated metadata = %#v", updated.Metadata)
	}
}
