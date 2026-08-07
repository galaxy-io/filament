package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/registry"
)

type stubSource struct{}

func (s *stubSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "stub-source"}
}
func (s *stubSource) Validate(filament.Config) error { return nil }
func (s *stubSource) Configure(context.Context, filament.Config) error {
	return nil
}

func (s *stubSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}
func (s *stubSource) Teardown(context.Context) error { return nil }

func newConnectionsTestServer() *Server {
	sources := registry.NewSources()
	sources.Register("stub-source", func() filament.Source { return &stubSource{} })
	return New(sources, registry.NewSinks(), memory.New(), nil, nil)
}

func createStubConnection(ctx context.Context, t *testing.T, api *Server, name string) string {
	t.Helper()
	created, err := api.CreateConnection(ctx, connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		TenantId:  "t1",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Name:      name,
		Connector: "stub-source",
	}))
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	return created.Msg.GetConnection().GetId()
}

func TestListConnectionsIncludeDeleted(t *testing.T) {
	ctx := context.Background()
	api := newConnectionsTestServer()
	id := createStubConnection(ctx, t, api, "pg-main")

	if _, err := api.DeleteConnection(ctx, connect.NewRequest(&ingestionv1.DeleteConnectionRequest{Id: id})); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	live, err := api.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{TenantId: "t1"}))
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if got := len(live.Msg.GetConnections()); got != 0 {
		t.Fatalf("ListConnections returned %d connections, want 0", got)
	}

	withDeleted, err := api.ListConnections(ctx, connect.NewRequest(&ingestionv1.ListConnectionsRequest{
		TenantId:       "t1",
		IncludeDeleted: true,
	}))
	if err != nil {
		t.Fatalf("ListConnections with IncludeDeleted: %v", err)
	}
	connections := withDeleted.Msg.GetConnections()
	if len(connections) != 1 {
		t.Fatalf("ListConnections with IncludeDeleted returned %d connections, want 1", len(connections))
	}
	if connections[0].GetKind() != ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE {
		t.Fatalf("expected the tombstone to keep its kind, got %v", connections[0].GetKind())
	}
}

func TestDeleteConnectionBlockedByLivePipeline(t *testing.T) {
	ctx := context.Background()
	api := newConnectionsTestServer()
	id := createStubConnection(ctx, t, api, "pg-main")

	if _, err := api.store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: "t1", Name: "uses-it"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if _, err := api.store.CreatePipelineVersion(ctx, "pipe-1", &ingestionv1.PipelineVersion{
		Nodes: []*ingestionv1.PipelineNode{{Id: "node-1", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: id}},
	}); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}

	_, err := api.DeleteConnection(ctx, connect.NewRequest(&ingestionv1.DeleteConnectionRequest{Id: id}))
	if err == nil {
		t.Fatal("expected DeleteConnection to be refused while a live pipeline references the connection")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("DeleteConnection error code = %v, want FailedPrecondition", got)
	}
}

func TestDeleteConnectionIgnoresDeletedPipelines(t *testing.T) {
	ctx := context.Background()
	api := newConnectionsTestServer()
	id := createStubConnection(ctx, t, api, "pg-main")

	if _, err := api.store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: "t1", Name: "used-it"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if _, err := api.store.CreatePipelineVersion(ctx, "pipe-1", &ingestionv1.PipelineVersion{
		Nodes: []*ingestionv1.PipelineNode{{Id: "node-1", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: id}},
	}); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if err := api.store.DeletePipeline(ctx, "pipe-1"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	if _, err := api.DeleteConnection(ctx, connect.NewRequest(&ingestionv1.DeleteConnectionRequest{Id: id})); err != nil {
		t.Fatalf("DeleteConnection after its pipeline was deleted: %v", err)
	}
}
