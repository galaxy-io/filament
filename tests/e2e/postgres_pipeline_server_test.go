//go:build integration

package e2e

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
	gxtc "github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	"github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"
)

// TestPostgresPipelineThroughServer drives the full SaaS control-plane flow the
// connections refactor was built for: two Postgres containers (a TPC-H-seeded
// source and an empty sink), then — entirely through the connect Server API —
// create a source Connection, create a sink Connection, create a Pipeline that
// references them, and run it. It asserts the run completes and every routed
// table lands in the sink with a matching row count.
//
// It also exercises the connection/pipeline scope split: dsn lives on the
// Connection (CONNECTION-scoped); the sink node carries a PIPELINE overlay
// (schema, mode) that merges over the connection config at run time.
func TestPostgresPipelineThroughServer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tpch.RegisterSF(0.01)
	src := gxtc.Postgres(t)
	dst := gxtc.Postgres(t)

	sc, ok := seed.Get("tpch-sf0.01")
	if !ok {
		t.Fatal("tpch-sf0.01 seed scenario was not registered")
	}
	if _, err := sc.Seeders["postgres"](ctx, src.DSN()); err != nil {
		t.Fatalf("seed postgres tpch: %v", err)
	}

	// Engine wiring: in-process bus + memory store + orchestrator/engine/tracker.
	sources := registry.NewSources()
	sources.Register("postgres", func() filament.Source { return pgsource.New() })
	sinks := registry.NewSinks()
	sinks.Register("postgres", func() filament.Sink { return pgsink.New() })

	bus := inproc.New()
	store := memory.New()
	orch := orchestrator.New()
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Sources: sources, Sinks: sinks},
		tracker.New(), engine.New(), orch,
	)
	if err != nil {
		t.Fatalf("mount modules: %v", err)
	}
	h := host.New(bus)
	if err := h.Run(ctx, mods...); err != nil {
		t.Fatalf("run host: %v", err)
	}
	defer func() { _ = h.Close() }()

	api := server.New(sources, sinks, store, orch, bus)

	// 1. Create the source and sink Connections through the API. dsn is
	//    CONNECTION-scoped, so it belongs in the connection config.
	srcConn := mustCreateConnection(t, ctx, api, &ingestionv1.CreateConnectionRequest{
		Tenant:    "t1",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Name:      "tpch-source",
		Connector: "postgres",
		Config:    mustStruct(t, map[string]any{"dsn": src.DSN()}),
	})
	sinkConn := mustCreateConnection(t, ctx, api, &ingestionv1.CreateConnectionRequest{
		Tenant:    "t1",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
		Name:      "warehouse",
		Connector: "postgres",
		Config:    mustStruct(t, map[string]any{"dsn": dst.DSN()}),
	})

	// 2. Create a Pipeline referencing the connections. The sink node carries a
	//    PIPELINE overlay (schema, mode) — connection-scoped keys here would be
	//    rejected by the server.
	resources := []string{"region", "nation", "supplier"}
	nodes := []*ingestionv1.PipelineNode{
		{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: srcConn.GetId()},
		{
			Id: "dst", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: sinkConn.GetId(),
			Config: mustStruct(t, map[string]any{"schema": "public", "mode": "typed"}),
		},
	}
	var edges []*ingestionv1.PipelineEdge
	for _, r := range resources {
		edges = append(edges, &ingestionv1.PipelineEdge{
			FromNode:      "src",
			ToNode:        "dst",
			Resource:      r,
			IngestionType: ingestionv1.IngestionType_INGESTION_TYPE_SNAPSHOT_REPLACE,
		})
	}
	created, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{
		Tenant: "t1",
		Name:   "tpch-sync",
		Nodes:  nodes,
		Edges:  edges,
	}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	pipelineID := created.Msg.GetPipeline().GetId()

	// 3. Run the pipeline.
	runResp, err := api.RunPipeline(ctx, connect.NewRequest(&ingestionv1.RunPipelineRequest{
		PipelineId:  pipelineID,
		ClientToken: "e2e",
	}))
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if len(runResp.Msg.GetRuns()) == 0 {
		t.Fatal("RunPipeline produced no runs")
	}

	for _, b := range runResp.Msg.GetRuns() {
		final := waitRunStatus(t, ctx, store, filament.RunID(b.GetRunId()),
			filament.RunCompleted, filament.RunFailed, filament.RunPartial)
		if final.Status != filament.RunCompleted {
			t.Fatalf("run %s status = %v, error = %q", b.GetRunId(), final.Status, final.Error)
		}
	}

	// 4. Every routed table must have moved source -> sink with matching counts.
	for _, table := range resources {
		want := countPostgres(t, ctx, src.Pool(), table)
		got := countPostgres(t, ctx, dst.Pool(), table)
		if got != want {
			t.Fatalf("%s: sink has %d rows, want %d from source", table, got, want)
		}
		if want == 0 {
			t.Fatalf("%s: source seeded 0 rows — seed did not populate the table", table)
		}
	}
}

func mustCreateConnection(t *testing.T, ctx context.Context, api *server.Server, req *ingestionv1.CreateConnectionRequest) *ingestionv1.Connection {
	t.Helper()
	resp, err := api.CreateConnection(ctx, connect.NewRequest(req))
	if err != nil {
		t.Fatalf("CreateConnection(%s): %v", req.GetName(), err)
	}
	return resp.Msg.GetConnection()
}

func mustStruct(t *testing.T, m map[string]any) *structpb.Struct {
	t.Helper()
	s, err := structpb.NewStruct(m)
	if err != nil {
		t.Fatalf("NewStruct: %v", err)
	}
	return s
}
