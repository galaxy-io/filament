package remote_test

import (
	"context"
	"net"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
	"github.com/galaxy-io/filament/cmd/internal/cli/target/targettest"
	_ "github.com/galaxy-io/filament/cmd/internal/connectors" // registers the connector catalog
	"github.com/galaxy-io/filament/datastore/sqlite"
	secretenv "github.com/galaxy-io/filament/secret/env"
)

// serve boots the real stack on a loopback listener and returns a target
// pointed at it. This is exactly how the CLI's local mode runs.
func serve(t *testing.T) *remote.Target {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- app.Serve(ctx, listener, app.WithDataStore(sqlite.NewMemory()), app.WithSecrets(secretenv.New()))
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("serve: %v", err)
		}
	})
	return remote.NewTarget(remote.Options{Endpoint: "http://" + listener.Addr().String()})
}

func TestEntityContract(t *testing.T) {
	targettest.EntityLifecycle(t, serve(t), "sample", "stdout")
}

func TestCatalogComesFromTheDeployment(t *testing.T) {
	catalog, err := serve(t).Catalog(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := catalog.Sources["sample"]; !ok {
		t.Fatalf("sources = %v", catalog.Sources)
	}
	if _, ok := catalog.Sinks["stdout"]; !ok {
		t.Fatalf("sinks = %v", catalog.Sinks)
	}
}

func TestPipelineRoundTripAndModes(t *testing.T) {
	ctx := context.Background()
	target := serve(t)
	if _, err := target.CreateConnection(ctx, "source", "src", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	proposed := model.Pipeline{Source: model.PipelineNode{Ref: "src"}, Sink: model.PipelineNode{Ref: "out"}, SyncMode: "full", WriteMode: "replace"}
	modes, err := target.PipelineModes(ctx, proposed)
	if err != nil {
		t.Fatal(err)
	}
	if modes.Replication != "standard" || len(modes.ReadModes) == 0 || len(modes.WriteModes) == 0 {
		t.Fatalf("modes = %+v", modes)
	}
	created, err := target.CreatePipeline(ctx, "copy", proposed)
	if err != nil {
		t.Fatal(err)
	}
	got, err := target.GetPipeline(ctx, "copy")
	if err != nil {
		t.Fatal(err)
	}
	if got.Source.Ref != "src" || got.Sink.Ref != "out" || got.SyncMode != "full" || got.WriteMode != "replace" || got.Metadata.Revision != created.Metadata.Revision {
		t.Fatalf("pipeline = %+v", got)
	}
	if got.Info.EditBlockedReason != "" || got.Graph == nil {
		t.Fatalf("info = %+v graph = %v", got.Info, got.Graph)
	}
	if _, err := target.GetPipeline(ctx, "missing"); err == nil {
		t.Fatal("missing pipeline resolved")
	}
	if _, err := target.CreatePipeline(ctx, "broken", model.Pipeline{Source: model.PipelineNode{Ref: "nope"}, Sink: model.PipelineNode{Ref: "out"}}); err == nil {
		t.Fatal("unknown connection accepted")
	}
}

func TestRunSavedPipelineToCompletion(t *testing.T) {
	ctx := context.Background()
	target := serve(t)
	if _, err := target.CreateConnection(ctx, "source", "src", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	pipeline, err := target.CreatePipeline(ctx, "copy", model.Pipeline{
		Source: model.PipelineNode{Ref: "src"}, Sink: model.PipelineNode{Ref: "out"}, SyncMode: "full", WriteMode: "replace",
	})
	if err != nil {
		t.Fatal(err)
	}
	group, err := target.SubmitRun(ctx, model.RunSubmission{Pipeline: &model.EntityReference{Name: "copy", Metadata: pipeline.Metadata}})
	if err != nil {
		t.Fatal(err)
	}
	if len(group.Runs) != 1 {
		t.Fatalf("runs = %+v", group.Runs)
	}
	// A sample run can finish before the tail subscribes; replay then carries
	// only the terminal event, so progress events are not asserted.
	result, err := target.TailRun(ctx, group, func(model.RunEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.Records == 0 {
		t.Fatalf("result = %+v", result)
	}
	if _, err := target.SubmitRun(ctx, model.RunSubmission{Spec: filament.RunSpec{}}); err == nil {
		t.Fatal("inline run accepted by the deployment target")
	}
}
