package local

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/app"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	remotetarget "github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
	"github.com/galaxy-io/filament/cmd/internal/cli/target/targettest"

	_ "github.com/galaxy-io/filament/connectors/sample"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)

// newTestTarget boots the embedded stack on a loopback listener, the same way
// the CLI's local mode does, and fronts it with a YAML-authoring target.
func newTestTarget(t *testing.T) *Target {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = app.Serve(ctx, listener)
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})
	api := remotetarget.NewTarget(remotetarget.Options{Endpoint: "http://" + listener.Addr().String()})
	return NewTarget(Store{Path: filepath.Join(t.TempDir(), "filament.yaml")}, api)
}

func TestTargetEntityContract(t *testing.T) {
	t.Parallel()
	targettest.EntityLifecycle(t, newTestTarget(t), "sample", "stdout")
}

func TestTargetConfigurationLifecycle(t *testing.T) {
	t.Parallel()
	target := newTestTarget(t)
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
	if len(connections.Items) != 1 || connections.Items[0].Name != "demo" || connections.Items[0].Description == "" {
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

// TestTargetApplySeedsDeployment covers local-mode boot: the YAML document is
// the durable state and Apply recreates it on the fresh embedded deployment.
func TestTargetApplySeedsDeployment(t *testing.T) {
	t.Parallel()
	target := newTestTarget(t)
	ctx := context.Background()

	if _, err := target.CreateConnection(ctx, "source", "demo", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreateConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	if _, err := target.CreatePipeline(ctx, "copy", model.Pipeline{
		Source: model.PipelineNode{Ref: "demo"}, Sink: model.PipelineNode{Ref: "out"},
		SyncMode: "full", WriteMode: "replace",
	}); err != nil {
		t.Fatal(err)
	}

	// A second embedded deployment starts empty, as every CLI invocation does;
	// applying the same YAML rebuilds it.
	restarted := newTestTarget(t)
	restarted.store = target.store
	if err := restarted.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	pipeline, err := restarted.GetPipeline(ctx, "copy")
	if err != nil {
		t.Fatal(err)
	}
	if pipeline.Source.Ref != "demo" || pipeline.Sink.Ref != "out" {
		t.Fatalf("applied pipeline = %#v", pipeline)
	}
}

// TestSyncerConvergesDeploymentAndDocument covers filament up's reconcile
// loop: UI-style edits made directly on the deployment land in the YAML, and
// YAML edits land on the deployment.
func TestSyncerConvergesDeploymentAndDocument(t *testing.T) {
	t.Parallel()
	target := newTestTarget(t)
	ctx := context.Background()

	if _, err := target.CreateConnection(ctx, "sink", "out", model.Connection{Type: "stdout"}); err != nil {
		t.Fatal(err)
	}
	syncer, err := NewSyncer(target, memorySecrets{})
	if err != nil {
		t.Fatal(err)
	}

	// A UI edit: created straight on the deployment, bypassing the wrapper.
	if _, err := target.Target.CreateConnection(ctx, "source", "ui-made", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if err := syncer.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	doc, _, _, err := target.store.LoadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Sources["ui-made"]; !ok {
		t.Fatalf("UI-created source did not sync to YAML: %#v", doc.Sources)
	}

	// A YAML edit: written to the file behind the deployment's back.
	if err := target.store.Create("sources", "yaml-made", model.Connection{Type: "sample"}); err != nil {
		t.Fatal(err)
	}
	if err := syncer.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := target.Target.GetConnection(ctx, "source", "yaml-made"); err != nil {
		t.Fatalf("YAML-created source did not sync to deployment: %v", err)
	}

	// A UI delete converges the document.
	deployed, err := target.Target.GetConnection(ctx, "source", "ui-made")
	if err != nil {
		t.Fatal(err)
	}
	if err := target.Target.DeleteConnection(ctx, "source", "ui-made", deployed.Metadata); err != nil {
		t.Fatal(err)
	}
	if err := syncer.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	doc, _, _, err = target.store.LoadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Sources["ui-made"]; ok {
		t.Fatal("UI-deleted source is still in the YAML")
	}
	if _, ok := doc.Sources["yaml-made"]; !ok {
		t.Fatal("unrelated source was dropped from the YAML")
	}
}

// memorySecrets satisfies reads the syncer may issue for stored values.
type memorySecrets struct{}

func (memorySecrets) Name() string { return "test" }
func (memorySecrets) Read(context.Context, string) (filament.Secret, error) {
	return filament.Secret{}, filament.ErrNotFound
}
func (memorySecrets) Write(context.Context, string, filament.Secret) error { return nil }
func (memorySecrets) Delete(context.Context, string) error                 { return nil }
