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
			filament.IngestionFullReplace,
			filament.IngestionFullUpsert,
			filament.IngestionFullAppend,
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
				filament.IngestionFullReplace,
				filament.IngestionFullUpsert,
				filament.IngestionFullAppend,
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

func TestConnectionReportsEffectiveReplication(t *testing.T) {
	api, ids := leverAPI(t)
	for name, want := range map[string]ingestionv1.ReplicationMode{
		"standard": ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD,
		"cdc":      ingestionv1.ReplicationMode_REPLICATION_MODE_CDC,
		"sink":     ingestionv1.ReplicationMode_REPLICATION_MODE_UNSPECIFIED,
	} {
		resp, err := api.GetConnection(context.Background(), connect.NewRequest(&ingestionv1.GetConnectionRequest{Id: ids[name]}))
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Msg.GetConnection().GetReplication(); got != want {
			t.Fatalf("%s replication = %v, want %v", name, got, want)
		}
	}

	resp, err := api.GetConnection(context.Background(), connect.NewRequest(&ingestionv1.GetConnectionRequest{Id: ids["cdc"]}))
	if err != nil {
		t.Fatal(err)
	}
	resp.Msg.Connection.Config, err = structpb.NewStruct(map[string]any{"replication": "standard"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = api.UpdateConnection(context.Background(), connect.NewRequest(&ingestionv1.UpdateConnectionRequest{Connection: resp.Msg.Connection}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("changing replication mode: got %v, want invalid argument", err)
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

	t.Run("default edge is Replace with final per-table options", func(t *testing.T) {
		resp := validate(ids["standard"], &ingestionv1.PipelineEdge{})
		if !resp.GetValid() {
			t.Fatalf("valid = false: %v", resp)
		}
		ev := resp.GetEdges()[0]
		if ev.GetEffectiveMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE {
			t.Fatalf("effective mode = %v", ev.GetEffectiveMode())
		}
		if got := byResource(ev, "orders").GetSupportedModes(); len(got) != 3 {
			t.Fatalf("orders modes = %v", got)
		}
		if got := byResource(ev, "audit").GetSupportedModes(); len(got) != 2 || got[0] != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE {
			t.Fatalf("audit modes = %v, want Replace and Append", got)
		}
	})

	t.Run("Incremental blocks only where cursor and key do not qualify", func(t *testing.T) {
		resp := validate(ids["standard"], &ingestionv1.PipelineEdge{
			StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL,
		})
		if resp.GetValid() {
			t.Fatal("valid = true, want blocked by audit")
		}
		ev := resp.GetEdges()[0]
		if ev.GetEffectiveMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL {
			t.Fatalf("effective mode = %v", ev.GetEffectiveMode())
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

	t.Run("Incremental scoped to a qualifying table is valid", func(t *testing.T) {
		resp := validate(ids["standard"], &ingestionv1.PipelineEdge{
			Resource:         "orders",
			StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL,
		})
		if !resp.GetValid() {
			t.Fatalf("valid = false: %v", resp)
		}
	})

	t.Run("CDC connection carries no Standard mode", func(t *testing.T) {
		resp := validate(ids["cdc"], &ingestionv1.PipelineEdge{})
		ev := resp.GetEdges()[0]
		if ev.GetReplication() != ingestionv1.ReplicationMode_REPLICATION_MODE_CDC {
			t.Fatalf("replication = %v", ev.GetReplication())
		}
		if got := byResource(ev, "orders").GetSupportedModes(); len(got) != 0 {
			t.Fatalf("CDC Standard modes = %v, want none", got)
		}
		if resp.GetValid() {
			t.Fatal("valid = true, want blocked by audit's missing primary key")
		}
	})
}

func TestNormalizeEdgeModes(t *testing.T) {
	api, ids := leverAPI(t)
	nodes := func(sourceConn string) []*ingestionv1.PipelineNode {
		return []*ingestionv1.PipelineNode{
			{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: sourceConn},
			{Id: "snk", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: ids["sink"]},
		}
	}

	edge := &ingestionv1.PipelineEdge{FromNode: "src", ToNode: "snk"}
	if err := api.normalizeEdgeModes(context.Background(), nodes(ids["standard"]), []*ingestionv1.PipelineEdge{edge}); err != nil {
		t.Fatal(err)
	}
	if edge.GetStandardSyncMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE {
		t.Fatalf("default mode = %v", edge.GetStandardSyncMode())
	}

	edge = &ingestionv1.PipelineEdge{
		FromNode: "src", ToNode: "snk",
		StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL,
	}
	if err := api.normalizeEdgeModes(context.Background(), nodes(ids["standard"]), []*ingestionv1.PipelineEdge{edge}); err != nil {
		t.Fatal(err)
	}
	if edge.GetStandardSyncMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_INCREMENTAL {
		t.Fatalf("Incremental mode = %v", edge.GetStandardSyncMode())
	}

	edge = &ingestionv1.PipelineEdge{FromNode: "src", ToNode: "snk"}
	if err := api.normalizeEdgeModes(context.Background(), nodes(ids["cdc"]), []*ingestionv1.PipelineEdge{edge}); err != nil {
		t.Fatal(err)
	}
	if edge.GetStandardSyncMode() != ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_UNSPECIFIED {
		t.Fatalf("CDC mode = %v", edge.GetStandardSyncMode())
	}

	edge = &ingestionv1.PipelineEdge{
		FromNode: "src", ToNode: "snk",
		StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE,
	}
	if err := api.normalizeEdgeModes(context.Background(), nodes(ids["cdc"]), []*ingestionv1.PipelineEdge{edge}); err == nil {
		t.Fatal("CDC must reject a Standard mode")
	}

	edge = &ingestionv1.PipelineEdge{
		FromNode: "src", ToNode: "snk",
		StandardSyncMode: ingestionv1.StandardSyncMode(99),
	}
	if err := api.normalizeEdgeModes(context.Background(), nodes(ids["standard"]), []*ingestionv1.PipelineEdge{edge}); err == nil {
		t.Fatal("unknown Standard sync mode must be rejected")
	}
}
