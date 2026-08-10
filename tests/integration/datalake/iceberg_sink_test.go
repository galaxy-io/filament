//go:build integration

// Package datalake holds heavy end-to-end tests that wire a real Postgres source
// through the ingestion pipeline into a real Iceberg sink (REST catalog + MinIO),
// then verify the landed tables with an independent Trino reader.
package datalake

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	"github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/runner"

	// Registered via init() so the registries can resolve them by name.
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/postgres"
)

// resources is the subset of TPC-H tables this test moves end-to-end. It mixes
// pure int/text tables (region, nation) with typed tables carrying numeric and
// date columns (customer, orders) so the sink's type mapping is exercised, not
// just the plumbing.
var resources = []string{"region", "nation", "customer", "orders"}

const namespace = "ingest"

// keepStack, when set via `-keep`, leaves the MinIO/REST/Trino stack running
// after the assertions pass so you can browse the landed Iceberg tables. The
// test blocks (skipping t.Cleanup) until interrupted with Ctrl-C.
var keepStack = flag.Bool("keep", false, "keep the data-lake containers running after the test for inspection (Ctrl-C to tear down)")

// TestPostgresToIceberg seeds TPC-H into a Postgres source, runs it through the
// pipeline into an Iceberg sink backed by a REST catalog + MinIO, and asserts the
// Iceberg row counts (read back via Trino) match the source.
func TestPostgresToIceberg(t *testing.T) {
	if _, err := exec.LookPath("duckdb"); err != nil {
		t.Skip("duckdb not on PATH — install with `brew install duckdb` to run TPC-H seed")
	}
	ctx := context.Background()

	// 1. Source: Postgres seeded with a small TPC-H dataset.
	pg := testcontainers.Postgres(t)
	scenario := tpchScenario(t)
	if _, err := scenario.Seeders["postgres"](ctx, pg.DSN()); err != nil {
		t.Fatalf("seed tpch: %v", err)
	}

	// 2. Sink target: MinIO + Iceberg REST catalog + Trino (verifier).
	dl := testcontainers.TrinoDataLake(t)

	// 3. Run the pipeline: Postgres source → Iceberg sink.
	runPipeline(t, ctx, pg, dl)

	// 4. Verify: every resource's Iceberg row count (via Trino) matches Postgres.
	waitTrinoReady(t, ctx, dl)
	for _, res := range resources {
		var pgCount int64
		if err := pg.Pool().QueryRow(ctx, "SELECT count(*) FROM "+res).Scan(&pgCount); err != nil {
			t.Fatalf("count postgres %s: %v", res, err)
		}
		q := fmt.Sprintf("SELECT count(*) FROM iceberg.%s.%s", namespace, res)
		iceCount := trinoCount(t, ctx, dl, q)
		if pgCount != iceCount {
			t.Errorf("%s: iceberg has %d rows, postgres has %d", res, iceCount, pgCount)
			continue
		}
		t.Logf("%-10s %d rows match", res, iceCount)
	}

	if *keepStack {
		holdForInspection(t, ctx, dl)
	}
}

// holdForInspection prints the stack's host-reachable endpoints and blocks until
// the process receives SIGINT/SIGTERM. Because it never returns normally, the
// helpers' t.Cleanup teardown does not run, so the containers stay up for
// browsing. Ctrl-C ends the process and the testcontainers reaper removes them.
func holdForInspection(t *testing.T, ctx context.Context, dl *testcontainers.DataLake) {
	t.Helper()
	apiEndpoint, _ := dl.MinIO.ConnectionString(ctx)
	host, _ := dl.MinIO.Host(ctx)
	consolePort, _ := dl.MinIO.MappedPort(ctx, "9001/tcp")

	fmt.Fprintf(os.Stderr, "\n──────── data lake held open (-keep) ────────\n")
	fmt.Fprintf(os.Stderr, "MinIO S3 API : http://%s\n", apiEndpoint)
	if consolePort.Port() != "" {
		fmt.Fprintf(os.Stderr, "MinIO console: http://%s:%s  (minioadmin / minioadmin)\n", host, consolePort.Port())
	}
	fmt.Fprintf(os.Stderr, "Trino DSN    : %s\n", dl.TrinoDSN)
	fmt.Fprintf(os.Stderr, "Tables       : iceberg.%s.{%s}\n", namespace, joinComma(resources))
	fmt.Fprintf(os.Stderr, "Ctrl-C to tear everything down.\n")
	fmt.Fprintf(os.Stderr, "─────────────────────────────────────────────\n\n")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
}

func joinComma(s []string) string {
	out := ""
	for i, v := range s {
		if i > 0 {
			out += ", "
		}
		out += v
	}
	return out
}

// runPipeline drives the run through runner.RunOne, the same sequence the engine
// module and the worker binary both execute.
//
// It deliberately does not hand-roll the steps. An earlier version resolved the
// connectors itself, called EnsureSchema in a loop, and built the pipeline
// directly — and silently fell behind RunOne twice: once when the iceberg
// catalog key was renamed, and once when write policies became a required part
// of pipeline.Config. Going through the real entry point means the test cannot
// drift from what production does.
func runPipeline(t *testing.T, ctx context.Context, pg *testcontainers.PG, dl *testcontainers.DataLake) {
	t.Helper()

	bus := inproc.New()
	facts, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() { _ = facts.Close() }()

	// Collect concurrently: RunOne publishes as it goes, and the terminal fact is
	// the only report of success or failure.
	type outcome struct {
		name string
		err  string
	}
	done := make(chan outcome, 1)
	go func() {
		for msg := range facts.C() {
			f, decodeErr := events.Decode(msg)
			_ = msg.Ack()
			if decodeErr != nil {
				continue
			}
			switch d := f.Data.(type) {
			case events.RunCompletedEvent:
				done <- outcome{name: f.Name}
				return
			case events.RunFailedEvent:
				done <- outcome{name: f.Name, err: d.Error}
				return
			case events.RunPartialEvent:
				done <- outcome{name: f.Name, err: d.Error}
				return
			}
		}
	}()

	runner.RunOne(ctx, runner.Deps{
		Bus:       bus,
		DataStore: memory.New(),
		Sources:   registry.DefaultSources,
		Sinks:     registry.DefaultSinks,
	}, filament.RunSpec{
		Tenant:    "t0",
		Run:       "run1",
		Resources: resources,
		Source: filament.Ref{
			Provider: "postgres",
			Config:   map[string]any{"dsn": pg.DSN(), "schema": "public"},
		},
		Sink: filament.Ref{
			Provider: "iceberg",
			Config:   sinkConfig(t, ctx, dl),
		},
		Options: filament.RunOptions{SnapshotParallelism: 1},
	})

	select {
	case got := <-done:
		if got.err != "" {
			t.Fatalf("run ended %s: %s", got.name, got.err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("no terminal fact within 30s")
	}
}

// waitTrinoReady blocks until Trino reports at least one active node, so the
// verification queries don't race "No nodes available to run query" right after
// the coordinator's HTTP endpoint comes up.
func waitTrinoReady(t *testing.T, ctx context.Context, dl *testcontainers.DataLake) {
	t.Helper()
	deadline := time.Now().Add(90 * time.Second)
	for {
		var n int
		err := dl.DB.QueryRowContext(ctx, "SELECT count(*) FROM system.runtime.nodes WHERE state = 'active'").Scan(&n)
		if err == nil && n >= 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("trino not ready (nodes=%d): %v", n, err)
		}
		time.Sleep(time.Second)
	}
}

// trinoCount runs a count query, retrying transient cluster-readiness errors.
func trinoCount(t *testing.T, ctx context.Context, dl *testcontainers.DataLake, q string) int64 {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for {
		var n int64
		err := dl.DB.QueryRowContext(ctx, q).Scan(&n)
		if err == nil {
			return n
		}
		if time.Now().After(deadline) {
			t.Fatalf("query %q: %v", q, err)
		}
		time.Sleep(time.Second)
	}
}

// sinkConfig builds the iceberg sink config pointing at the test's REST catalog
// and MinIO, both reached from the host via their mapped ports. The s3.* keys
// flow through to iceberg-go's S3 FileIO so the sink writes Parquet to MinIO.
func sinkConfig(t *testing.T, ctx context.Context, dl *testcontainers.DataLake) map[string]any {
	t.Helper()

	restHost, err := dl.Catalog.Host(ctx)
	if err != nil {
		t.Fatalf("catalog host: %v", err)
	}
	restPort, err := dl.Catalog.MappedPort(ctx, "8181")
	if err != nil {
		t.Fatalf("catalog port: %v", err)
	}
	minioEndpoint, err := dl.MinIO.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("minio endpoint: %v", err)
	}

	return map[string]any{
		"warehouse": "s3://" + dl.Bucket + "/",
		"namespace": namespace,
		"catalog": map[string]any{
			"provider": "rest",
			"uri":      fmt.Sprintf("http://%s:%s", restHost, restPort.Port()),
			// Storage settings must be nested: buildRESTCatalog lifts only uri and
			// warehouse from the catalog object and copies everything else from
			// properties. Flat s3.* keys are silently dropped, which surfaces much
			// later as a 301 from MinIO on the first PutObject.
			"properties": map[string]any{
				"s3.endpoint":                 "http://" + minioEndpoint,
				"s3.access-key-id":            dl.MinIO.Username,
				"s3.secret-access-key":        dl.MinIO.Password,
				"s3.region":                   "us-east-1",
				"s3.force-virtual-addressing": "false", // MinIO needs path-style addressing
			},
		},
	}
}

// tpchScenario registers (once) and returns the small SF 0.01 TPC-H scenario —
// the default tpch init only registers 0.1/1/5, which are larger than this
// smoke test needs.
func tpchScenario(t *testing.T) seed.Scenario {
	t.Helper()
	const name = "tpch-sf0.01"
	if _, ok := seed.Get(name); !ok {
		tpch.RegisterSF(0.01)
	}
	sc, ok := seed.Get(name)
	if !ok {
		t.Fatalf("tpch scenario %q not registered", name)
	}
	return sc
}
