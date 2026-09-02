// Package targettest contains reusable behavioral contracts for CLI targets.
package targettest

import (
	"context"
	"testing"

	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// EntityLifecycle verifies the query, mutation, identity, and optimistic
// concurrency behavior every target adapter must provide.
func EntityLifecycle(t *testing.T, target cliapp.Target, sourceConnector, sinkConnector string) {
	t.Helper()
	ctx := context.Background()

	created, err := target.CreateConnection(ctx, "source", "contract-source", model.Connection{Type: sourceConnector})
	if err != nil {
		t.Fatal(err)
	}
	if created.Metadata.ID == "" || created.Metadata.Revision == "" {
		t.Fatalf("create metadata = %#v", created.Metadata)
	}
	if _, err := target.CreateConnection(ctx, "source", "contract-source", model.Connection{Type: sourceConnector}); err == nil {
		t.Fatal("duplicate connection create succeeded")
	}

	stale := created
	created.Config = map[string]any{"contract": true}
	updated, err := target.UpdateConnection(ctx, "source", "contract-source", created)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Metadata.ID != created.Metadata.ID || updated.Metadata.Revision == created.Metadata.Revision {
		t.Fatalf("update metadata = %#v, create metadata = %#v", updated.Metadata, created.Metadata)
	}
	if _, err := target.UpdateConnection(ctx, "source", "contract-source", stale); err == nil {
		t.Fatal("stale connection update succeeded")
	}

	if _, err := target.CreateConnection(ctx, "sink", "contract-sink", model.Connection{Type: sinkConnector}); err != nil {
		t.Fatal(err)
	}
	pipeline, err := target.CreatePipeline(ctx, "contract-pipeline", model.Pipeline{
		Source: model.PipelineNode{Ref: "contract-source"},
		Sink:   model.PipelineNode{Ref: "contract-sink"}, SyncMode: "full", WriteMode: "replace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pipeline.Metadata.ID == "" || pipeline.Metadata.Revision == "" {
		t.Fatalf("pipeline metadata = %#v", pipeline.Metadata)
	}
	pipeline.Resources = []string{"users"}
	// Flat fields are authoritative only when the graph is unset; edit flows
	// rebuild the graph the same way.
	pipeline.Graph = nil
	pipeline, err = target.UpdatePipeline(ctx, "contract-pipeline", pipeline)
	if err != nil {
		t.Fatal(err)
	}
	if len(pipeline.Resources) != 1 {
		t.Fatalf("updated pipeline = %#v", pipeline)
	}

	if err := target.DeletePipeline(ctx, "contract-pipeline", pipeline.Metadata); err != nil {
		t.Fatal(err)
	}
	connection, err := target.GetConnection(ctx, "source", "contract-source")
	if err != nil {
		t.Fatal(err)
	}
	if err := target.DeleteConnection(ctx, "source", "contract-source", connection.Metadata); err != nil {
		t.Fatal(err)
	}
	if _, err := target.GetConnection(ctx, "source", "contract-source"); err == nil {
		t.Fatal("deleted connection is still queryable")
	}
}
