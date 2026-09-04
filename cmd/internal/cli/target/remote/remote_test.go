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
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
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
	var events []model.RunEvent
	result, err := target.TailRun(ctx, group, func(event model.RunEvent) { events = append(events, event) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "complete" || result.Records == 0 {
		t.Fatalf("result = %+v", result)
	}
	if len(events) == 0 {
		t.Fatal("no progress events observed")
	}
	if _, err := target.SubmitRun(ctx, model.RunSubmission{Spec: filament.RunSpec{}}); err == nil {
		t.Fatal("inline run accepted by the deployment target")
	}
}

// serveWithSecrets is serve with a writable secret store, for paths that
// mint refs.
func serveWithSecrets(t *testing.T) (*remote.Target, *secretsqlite.Provider) {
	t.Helper()
	store := sqlite.NewMemory()
	secrets, err := secretsqlite.New(store.DB(), "test", []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- app.Serve(ctx, listener, app.WithDataStore(store), app.WithSecrets(secrets)) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Errorf("serve: %v", err)
		}
	})
	return remote.NewTarget(remote.Options{Endpoint: "http://" + listener.Addr().String()}), secrets
}

// A connection read back from the deployment carries refs and no values, so
// a value supplied on update is new input and must reach the deployment.
func TestUpdateReplacesSecret(t *testing.T) {
	ctx := context.Background()
	target, secrets := serveWithSecrets(t)
	created, err := target.CreateConnection(ctx, "source", "crm", model.Connection{Type: "attio", Config: map[string]any{"api_key": "first"}})
	if err != nil {
		t.Fatal(err)
	}
	created.Config = map[string]any{"api_key": "second"}
	updated, err := target.UpdateConnection(ctx, "source", "crm", created)
	if err != nil {
		t.Fatal(err)
	}
	if updated.SecretRefs["api_key"] == created.SecretRefs["api_key"] {
		t.Fatal("secret ref unchanged: new value was dropped")
	}
	secret, err := secrets.Read(ctx, updated.SecretRefs["api_key"])
	if err != nil || string(secret.Value) != "second" {
		t.Fatalf("stored value = %q, err = %v", secret.Value, err)
	}
}
