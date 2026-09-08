//go:build e2e

package datalake

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	icebergsink "github.com/galaxy-io/filament/connectors/iceberg"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	gxtc "github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
)

func TestNATSPostgresTPCHToIceberg(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	testutil.RegisterTPCHSmokeScenario()
	pg := gxtc.Postgres(t)
	nats := gxtc.NATSContainer(t)
	lake := gxtc.TrinoDataLake(t)

	sc, ok := seed.Get("tpch-sf0.01")
	if !ok {
		t.Fatal("tpch-sf0.01 seed scenario was not registered")
	}
	manifest, err := sc.Seeders["postgres"](ctx, pg.DSN())
	if err != nil {
		t.Fatalf("seed postgres tpch: %v", err)
	}
	t.Logf("seeded %s with %d table(s)", manifest.Spec.Name, len(manifest.Tables))

	restURI := icebergRESTURI(t, ctx, lake)
	minioEndpoint, err := lake.MinIO.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("minio endpoint: %v", err)
	}
	t.Setenv("AWS_ACCESS_KEY_ID", "minioadmin")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "minioadmin")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("AWS_S3_ENDPOINT", "http://"+minioEndpoint)

	bus, err := natsbus.New(nats.URL, events.Codec,
		natsbus.WithStream("INGESTION_E2E"),
		natsbus.WithSubjects("ingestion.v1.>"),
	)
	if err != nil {
		t.Fatalf("nats bus: %v", err)
	}
	defer func() { _ = bus.Close() }()

	sources := registry.NewSources()
	sources.Register("postgres", func() filament.Source { return pgsource.New() })
	sinks := registry.NewSinks()
	sinks.Register("iceberg", func() filament.Sink { return icebergsink.New() })

	store := sqlite.NewMemory()
	orch := orchestrator.New()
	mods, err := module.MountAll(ctx,
		module.Deps{Bus: bus, DataStore: store, Sources: sources, Sinks: sinks},
		tracker.New(),
		engine.New(),
		orch,
	)
	if err != nil {
		t.Fatalf("mount modules: %v", err)
	}
	h := host.New(bus, host.WithLogf(t.Logf))
	if err := h.Run(ctx, mods...); err != nil {
		t.Fatalf("run host: %v", err)
	}
	defer func() { _ = h.Close() }()

	resources := []string{"region", "nation", "supplier"}
	runID, err := orch.Submit(ctx, filament.RunSubmission{Request: filament.RunRequest{
		Tenant: "t1",
		Source: filament.Ref{
			Connector: "postgres",
			Config:    map[string]any{"dsn": pg.DSN()},
		},
		Sink: filament.Ref{
			Connector: "iceberg",
			Config: map[string]any{
				"warehouse": "s3://" + lake.Bucket + "/",
				"namespace": "tpch_e2e",
				"catalog": map[string]any{
					"provider": "rest",
					"uri":      restURI,
				},
			},
		},
		Resources: resources,
		Options: filament.RunOptions{
			BatchMaxRows:        500,
			SnapshotParallelism: 2,
		},
	}})
	if err != nil {
		t.Fatalf("submit run: %v", err)
	}

	final := testutil.WaitForStatuses(t, ctx, store, "t1", runID, filament.RunCompleted, filament.RunFailed, filament.RunPartial)
	if final.Status != filament.RunCompleted {
		t.Fatalf("run %s status = %v, error = %q", runID, final.Status, final.Error)
	}

	for _, table := range resources {
		srcCount := countPostgres(t, ctx, pg.Pool(), table)
		dstCount := countTrino(t, ctx, lake.DB, "tpch_e2e", table)
		if dstCount != srcCount {
			t.Fatalf("%s row count = %d in iceberg, want %d from postgres", table, dstCount, srcCount)
		}
		sourceRows := canonicalPostgresTPCHRows(t, ctx, pg.Pool(), table)
		destinationRows := canonicalTrinoTPCHRows(t, ctx, lake.DB, "tpch_e2e", table)
		if !slices.Equal(destinationRows, sourceRows) {
			t.Fatalf("%s values differ between Postgres and Iceberg\npostgres=%v\niceberg=%v", table, sourceRows, destinationRows)
		}
	}
}

func icebergRESTURI(t *testing.T, ctx context.Context, lake *gxtc.DataLake) string {
	t.Helper()
	host, err := lake.Catalog.Host(ctx)
	if err != nil {
		t.Fatalf("iceberg rest host: %v", err)
	}
	port, err := lake.Catalog.MappedPort(ctx, "8181/tcp")
	if err != nil {
		t.Fatalf("iceberg rest port: %v", err)
	}
	return fmt.Sprintf("http://%s:%s", host, port.Port())
}

func countPostgres(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count postgres %s: %v", table, err)
	}
	return n
}

var tpchComparisonColumns = map[string][]string{
	"region":   {"r_regionkey", "r_name", "r_comment"},
	"nation":   {"n_nationkey", "n_name", "n_regionkey", "n_comment"},
	"supplier": {"s_suppkey", "s_name", "s_address", "s_nationkey", "s_phone", "s_acctbal", "s_comment"},
}

var tpchOrderColumns = map[string]string{
	"region": "r_regionkey", "nation": "n_nationkey", "supplier": "s_suppkey",
}

func canonicalPostgresTPCHRows(t testing.TB, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	columns := tpchComparisonColumns[table]
	parts := make([]string, len(columns))
	for i, column := range columns {
		identifier := pgx.Identifier{column}.Sanitize()
		if table == "supplier" && column == "s_acctbal" {
			identifier = "(" + identifier + "::numeric(38,9))"
		}
		parts[i] = "COALESCE(" + identifier + "::text, '<NULL>')"
	}
	query := "SELECT concat_ws(E'\\x1f', " + strings.Join(parts, ", ") + ") FROM " +
		pgx.Identifier{table}.Sanitize() + " ORDER BY " + pgx.Identifier{tpchOrderColumns[table]}.Sanitize()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		t.Fatalf("read postgres %s: %v", table, err)
	}
	defer rows.Close()
	return scanCanonicalRows(t, rows)
}

type stringRows interface {
	Next() bool
	Scan(...any) error
	Err() error
}

func scanCanonicalRows(t testing.TB, rows stringRows) []string {
	t.Helper()
	var out []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatal(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func canonicalTrinoTPCHRows(t testing.TB, ctx context.Context, db *sql.DB, namespace, table string) []string {
	t.Helper()
	columns := tpchComparisonColumns[table]
	parts := make([]string, len(columns))
	for i, column := range columns {
		expression := column
		if table == "supplier" && column == "s_acctbal" {
			expression = "CAST(" + column + " AS DECIMAL(38,9))"
		}
		parts[i] = "COALESCE(CAST(" + expression + " AS VARCHAR), '<NULL>')"
	}
	query := fmt.Sprintf("SELECT concat(%s) FROM iceberg.%s.%s ORDER BY %s",
		strings.Join(interleave(parts, "chr(31)"), ", "), namespace, table, tpchOrderColumns[table])
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		t.Fatalf("read iceberg %s.%s: %v", namespace, table, err)
	}
	defer rows.Close()
	return scanCanonicalRows(t, rows)
}

func interleave(values []string, separator string) []string {
	if len(values) < 2 {
		return values
	}
	out := make([]string, 0, len(values)*2-1)
	for i, value := range values {
		if i > 0 {
			out = append(out, separator)
		}
		out = append(out, value)
	}
	return out
}

func countTrino(t *testing.T, ctx context.Context, db *sql.DB, namespace, table string) int64 {
	t.Helper()
	query := fmt.Sprintf("SELECT count(*) FROM iceberg.%s.%s", namespace, table)
	deadline := time.Now().Add(60 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		var n int64
		if err := db.QueryRowContext(ctx, query).Scan(&n); err == nil {
			return n
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			t.Fatalf("count iceberg %s.%s: %v", namespace, table, ctx.Err())
		case <-time.After(time.Second):
		}
	}
	t.Fatalf("count iceberg %s.%s: %v", namespace, table, lastErr)
	return 0
}
