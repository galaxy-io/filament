package local

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
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

	if err := target.PutConnection(ctx, "source", "demo", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if err := target.PutConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	if err := target.PutPipeline(ctx, "copy", model.Pipeline{
		Source: model.PipelineNode{Ref: "demo"}, Sink: model.PipelineNode{Ref: "out"},
		Resources: []string{"users"}, SyncMode: "full", WriteMode: "replace",
	}); err != nil {
		t.Fatal(err)
	}

	connections, err := target.ListConnections(ctx, "source")
	if err != nil {
		t.Fatal(err)
	}
	if len(connections.Items) != 1 || connections.Items[0].Name != "demo" || connections.Items[0].Description != "Synthetic source" {
		t.Fatalf("connections = %#v", connections)
	}
	pipelines, err := target.ListPipelines(ctx)
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

	if err := target.DeletePipeline(ctx, "copy"); err != nil {
		t.Fatal(err)
	}
	if err := target.DeleteConnection(ctx, "source", "demo"); err != nil {
		t.Fatal(err)
	}
	doc, err := target.Configuration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Pipelines) != 0 || len(doc.Sources) != 0 || len(doc.Sinks) != 1 {
		t.Fatalf("configuration = %#v", doc)
	}
}
