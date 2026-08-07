package server

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/registry"
)

// leverSource declares the full grid plus CDC and answers replication from
// the "replication" config value. "orders" has a primary key and an
// auto-detectable cursor; "audit" has neither.
type leverSource struct{}

func (leverSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name: "leversource",
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionSnapshotReplace,
			filament.IngestionSnapshotUpsert,
			filament.IngestionSnapshotAppend,
			filament.IngestionIncrementalAppend,
			filament.IngestionIncrementalUpsert,
			filament.IngestionCDC,
		),
		Resources: filament.ResourceCapabilities{Discoverable: true},
	}
}

func (leverSource) Validate(filament.Config) error                   { return nil }
func (leverSource) Configure(context.Context, filament.Config) error { return nil }
func (leverSource) Teardown(context.Context) error                   { return nil }

func (leverSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}

func (leverSource) Replication(cfg filament.Config) filament.ReplicationMode {
	if cfg.String("replication") == string(filament.ReplicationCDC) {
		return filament.ReplicationCDC
	}
	return filament.ReplicationStandard
}

func (leverSource) Discover(context.Context, filament.DiscoverOpts) (filament.DiscoverResult, error) {
	return filament.DiscoverResult{Resources: []filament.Resource{
		{Name: "orders", Selectable: true, PrimaryKey: []string{"id"}},
		{Name: "audit", Selectable: true},
	}}, nil
}

func (leverSource) Schema(_ context.Context, resource string) (filament.RecordSchema, error) {
	if resource == "orders" {
		return filament.RecordSchema{Resource: resource, PrimaryKey: []string{"id"}}, nil
	}
	return filament.RecordSchema{Resource: resource}, nil
}

func (leverSource) CursorColumns(_ context.Context, resource string) ([]filament.CursorColumn, error) {
	if resource != "orders" {
		return nil, nil
	}
	return []filament.CursorColumn{{
		SchemaField: filament.SchemaField{Name: "updated_at"},
		Eligible:    true, Recommended: true, Rank: 1,
	}}, nil
}

type leverSink struct{}

func (leverSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name: "leversink",
		Capabilities: filament.SinkCapabilities{
			Upsertable: true,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionSnapshotReplace,
				filament.IngestionSnapshotUpsert,
				filament.IngestionSnapshotAppend,
				filament.IngestionIncrementalAppend,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDC,
			),
		},
	}
}

func (leverSink) Open(context.Context, filament.RunSpec) error { return nil }
func (leverSink) Commit(context.Context) error                 { return nil }
func (leverSink) Abort(context.Context) error                  { return nil }
func (leverSink) Name() string                                 { return "leversink" }

func (leverSink) Apply(context.Context, filament.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, nil
}

// leverAPI builds a server with one standard source connection, one CDC
// source connection, and one sink connection.
func leverAPI(t *testing.T) (*Server, map[string]string) {
	t.Helper()
	sources := registry.NewSources()
	sources.Register("leversource", func() filament.Source { return leverSource{} })
	sinks := registry.NewSinks()
	sinks.Register("leversink", func() filament.Sink { return leverSink{} })
	api := New(sources, sinks, memory.New(), nil, nil)

	create := func(kind ingestionv1.ConnectorKind, name, connector string, config map[string]any) string {
		var cfg *structpb.Struct
		if config != nil {
			var err error
			cfg, err = structpb.NewStruct(config)
			if err != nil {
				t.Fatal(err)
			}
		}
		resp, err := api.CreateConnection(context.Background(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
			Kind: kind, Name: name, Connector: connector, Config: cfg,
		}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg.GetConnection().GetId()
	}

	return api, map[string]string{
		"standard": create(ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, "standard", "leversource", nil),
		"cdc":      create(ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, "cdc", "leversource", map[string]any{"replication": "cdc"}),
		"sink":     create(ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, "sink", "leversink", nil),
	}
}

func TestGetConnectionCapabilitiesLevers(t *testing.T) {
	api, ids := leverAPI(t)
	get := func(id string) *ingestionv1.GetConnectionCapabilitiesResponse {
		resp, err := api.GetConnectionCapabilities(context.Background(), connect.NewRequest(&ingestionv1.GetConnectionCapabilitiesRequest{Id: id}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg
	}

	standard := get(ids["standard"])
	if standard.GetReplication() != ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD {
		t.Fatalf("standard replication = %v", standard.GetReplication())
	}
	if got := standard.GetReadModes(); len(got) != 2 {
		t.Fatalf("standard read modes = %v", got)
	}
	if got := len(standard.GetCapabilities().GetSourcePolicies()); got != 5 {
		t.Fatalf("standard policies = %d, want 5 (cdc filtered)", got)
	}

	cdc := get(ids["cdc"])
	if cdc.GetReplication() != ingestionv1.ReplicationMode_REPLICATION_MODE_CDC {
		t.Fatalf("cdc replication = %v", cdc.GetReplication())
	}
	if got := cdc.GetReadModes(); len(got) != 0 {
		t.Fatalf("cdc read modes = %v, want none", got)
	}
	if got := len(cdc.GetCapabilities().GetSourcePolicies()); got != 1 {
		t.Fatalf("cdc policies = %d, want 1", got)
	}

	sink := get(ids["sink"])
	want := []ingestionv1.WriteMode{
		ingestionv1.WriteMode_WRITE_MODE_APPEND,
		ingestionv1.WriteMode_WRITE_MODE_REPLACE,
		ingestionv1.WriteMode_WRITE_MODE_UPSERT,
	}
	if got := sink.GetWriteModes(); len(got) != len(want) || got[0] != want[0] || got[2] != want[2] {
		t.Fatalf("sink write modes = %v", got)
	}
}

func TestValidatePipelineLevers(t *testing.T) {
	api, ids := leverAPI(t)
	validate := func(sourceConn string, edge *ingestionv1.PipelineEdge) *ingestionv1.ValidatePipelineResponse {
		edge.FromNode, edge.ToNode = "src", "snk"
		resp, err := api.ValidatePipeline(context.Background(), connect.NewRequest(&ingestionv1.ValidatePipelineRequest{
			Nodes: []*ingestionv1.PipelineNode{
				{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: sourceConn},
				{Id: "snk", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: ids["sink"]},
			},
			Edges: []*ingestionv1.PipelineEdge{edge},
		}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg
	}
	byResource := func(ev *ingestionv1.EdgeValidation, name string) *ingestionv1.ResourceValidation {
		for _, rv := range ev.GetResources() {
			if rv.GetResource() == name {
				return rv
			}
		}
		t.Fatalf("resource %q missing from breakdown", name)
		return nil
	}

	t.Run("default edge is a valid full refresh with per-table menus", func(t *testing.T) {
		resp := validate(ids["standard"], &ingestionv1.PipelineEdge{})
		if !resp.GetValid() {
			t.Fatalf("valid = false: %v", resp)
		}
		ev := resp.GetEdges()[0]
		if ev.GetIngestionType() != ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE {
			t.Fatalf("derived = %v", ev.GetIngestionType())
		}
		if got := byResource(ev, "orders").GetSupportedReadModes(); len(got) != 2 {
			t.Fatalf("orders read modes = %v", got)
		}
		if got := byResource(ev, "audit").GetSupportedReadModes(); len(got) != 1 || got[0] != ingestionv1.ReadMode_READ_MODE_FULL {
			t.Fatalf("audit read modes = %v, want full only", got)
		}
	})

	t.Run("incremental upsert blocks only where nothing qualifies", func(t *testing.T) {
		resp := validate(ids["standard"], &ingestionv1.PipelineEdge{
			ReadMode:  ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
			WriteMode: ingestionv1.WriteMode_WRITE_MODE_UPSERT,
		})
		if resp.GetValid() {
			t.Fatal("valid = true, want blocked by audit")
		}
		ev := resp.GetEdges()[0]
		if ev.GetIngestionType() != ingestionv1.IngestionType_INGESTION_TYPE_INCREMENTAL_UPSERT {
			t.Fatalf("derived = %v", ev.GetIngestionType())
		}
		orders := byResource(ev, "orders")
		for _, req := range orders.GetRequirements() {
			if req.GetBlocking() {
				t.Fatalf("orders should be advisory only: %v", req)
			}
		}
		blocking := 0
		for _, req := range byResource(ev, "audit").GetRequirements() {
			if req.GetBlocking() {
				blocking++
			}
		}
		if blocking != 2 {
			t.Fatalf("audit blocking requirements = %d, want cursor + primary key", blocking)
		}
	})

	t.Run("incremental upsert scoped to a qualifying table is valid", func(t *testing.T) {
		resp := validate(ids["standard"], &ingestionv1.PipelineEdge{
			Resource:  "orders",
			ReadMode:  ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
			WriteMode: ingestionv1.WriteMode_WRITE_MODE_UPSERT,
		})
		if !resp.GetValid() {
			t.Fatalf("valid = false: %v", resp)
		}
	})

	t.Run("cdc connection derives cdc and carries no levers", func(t *testing.T) {
		resp := validate(ids["cdc"], &ingestionv1.PipelineEdge{})
		ev := resp.GetEdges()[0]
		if ev.GetIngestionType() != ingestionv1.IngestionType_INGESTION_TYPE_CDC {
			t.Fatalf("derived = %v", ev.GetIngestionType())
		}
		if got := byResource(ev, "orders").GetSupportedReadModes(); len(got) != 0 {
			t.Fatalf("cdc per-table read modes = %v, want none", got)
		}
		if resp.GetValid() {
			t.Fatal("valid = true, want blocked by audit's missing primary key")
		}
	})
}

func TestDeriveEdgeTypes(t *testing.T) {
	api, ids := leverAPI(t)
	nodes := func(sourceConn string) []*ingestionv1.PipelineNode {
		return []*ingestionv1.PipelineNode{
			{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: sourceConn},
			{Id: "snk", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: ids["sink"]},
		}
	}

	edge := &ingestionv1.PipelineEdge{FromNode: "src", ToNode: "snk"}
	if err := api.deriveEdgeTypes(context.Background(), nodes(ids["standard"]), []*ingestionv1.PipelineEdge{edge}); err != nil {
		t.Fatal(err)
	}
	if edge.GetIngestionType() != ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE {
		t.Fatalf("default derived = %v", edge.GetIngestionType())
	}

	edge = &ingestionv1.PipelineEdge{
		FromNode: "src", ToNode: "snk",
		ReadMode:  ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
		WriteMode: ingestionv1.WriteMode_WRITE_MODE_APPEND,
	}
	if err := api.deriveEdgeTypes(context.Background(), nodes(ids["standard"]), []*ingestionv1.PipelineEdge{edge}); err != nil {
		t.Fatal(err)
	}
	if edge.GetIngestionType() != ingestionv1.IngestionType_INGESTION_TYPE_INCREMENTAL_APPEND {
		t.Fatalf("incremental append derived = %v", edge.GetIngestionType())
	}

	edge = &ingestionv1.PipelineEdge{FromNode: "src", ToNode: "snk"}
	if err := api.deriveEdgeTypes(context.Background(), nodes(ids["cdc"]), []*ingestionv1.PipelineEdge{edge}); err != nil {
		t.Fatal(err)
	}
	if edge.GetIngestionType() != ingestionv1.IngestionType_INGESTION_TYPE_CDC {
		t.Fatalf("cdc derived = %v", edge.GetIngestionType())
	}

	edge = &ingestionv1.PipelineEdge{
		FromNode: "src", ToNode: "snk",
		ReadMode:  ingestionv1.ReadMode_READ_MODE_INCREMENTAL,
		WriteMode: ingestionv1.WriteMode_WRITE_MODE_REPLACE,
	}
	if err := api.deriveEdgeTypes(context.Background(), nodes(ids["standard"]), []*ingestionv1.PipelineEdge{edge}); err == nil {
		t.Fatal("incremental replace must not compile")
	}
}
