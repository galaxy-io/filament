//go:build integration

package e2e

import (
	"context"
	"fmt"
	"sync"
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

	registerTPCHSmokeScenario()
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
	secrets := newMemorySecrets()
	orch := orchestrator.New()
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Secrets: secrets, Sources: sources, Sinks: sinks},
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

	api := server.New(sources, sinks, store, orch, bus, server.WithSecrets(secrets))

	// 1. Create the source and sink Connections through the API. dsn is
	//    CONNECTION-scoped, so it belongs in the connection config.
	srcConn := mustCreateConnection(t, ctx, api, &ingestionv1.CreateConnectionRequest{
		TenantId:  "t1",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Name:      "tpch-source",
		Connector: "postgres",
		Config:    mustStruct(t, map[string]any{"dsn": src.DSN()}),
	})
	sinkConn := mustCreateConnection(t, ctx, api, &ingestionv1.CreateConnectionRequest{
		TenantId:  "t1",
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
			FromNode:         "src",
			ToNode:           "dst",
			Resource:         r,
			StandardSyncMode: ingestionv1.StandardSyncMode_STANDARD_SYNC_MODE_REPLACE,
		})
	}
	created, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{
		TenantId: "t1",
		Name:     "tpch-sync",
	}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	pipelineID := created.Msg.GetPipeline().GetId()
	if _, err := api.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
		PipelineId: pipelineID,
		Graph:      &ingestionv1.PipelineGraph{Nodes: nodes, Edges: edges},
	})); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}

	// 3. Run the pipeline.
	runResp, err := api.RunPipeline(ctx, connect.NewRequest(&ingestionv1.RunPipelineRequest{
		PipelineId:  pipelineID,
		ClientToken: "e2e",
	}))
	if err != nil {
		t.Fatalf("RunPipeline: %v", err)
	}
	if len(runResp.Msg.GetEdgeRuns()) == 0 {
		t.Fatal("RunPipeline produced no runs")
	}

	for _, er := range runResp.Msg.GetEdgeRuns() {
		run := er.GetRun().GetId()
		final := waitRunStatus(t, ctx, store, filament.RunID(run),
			filament.RunCompleted, filament.RunFailed, filament.RunPartial)
		if final.Status != filament.RunCompleted {
			t.Fatalf("run %s status = %v, error = %q", run, final.Status, final.Error)
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

func registerTPCHSmokeScenario() {
	if _, ok := seed.Get("tpch-sf0.01"); !ok {
		tpch.RegisterSF(0.01)
	}
}

type memorySecrets struct {
	mu     sync.RWMutex
	values map[string]filament.Secret
}

func newMemorySecrets() *memorySecrets {
	return &memorySecrets{values: make(map[string]filament.Secret)}
}

func (s *memorySecrets) Read(_ context.Context, ref string) (filament.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	secret, ok := s.values[ref]
	if !ok {
		return filament.Secret{}, fmt.Errorf("secret %q: %w", ref, filament.ErrNotFound)
	}
	return secret, nil
}

func (s *memorySecrets) Write(_ context.Context, ref string, secret filament.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[ref] = secret
	return nil
}

func (s *memorySecrets) Delete(_ context.Context, ref string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, ref)
	return nil
}

func (s *memorySecrets) Name() string { return "memory" }
