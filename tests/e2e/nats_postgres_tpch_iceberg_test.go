//go:build integration

package e2e

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ingestion "github.com/galaxy-io/filament"
	icebergsink "github.com/galaxy-io/filament/connectors/iceberg"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus/host"
	natsbus "github.com/galaxy-io/filament/eventbus/nats"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	gxtc "github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	"github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"
)

func TestNATSPostgresTPCHToIceberg(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	tpch.RegisterSF(0.01)
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
	sources.Register("postgres", func() ingestion.Source { return pgsource.New() })
	sinks := registry.NewSinks()
	sinks.Register("iceberg", func() ingestion.Sink { return icebergsink.New() })

	store := memory.New()
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
	runID, err := orch.Submit(ctx, ingestion.RunRequest{
		Tenant: "t1",
		Source: ingestion.Ref{
			Provider: "postgres",
			Config:   map[string]any{"dsn": pg.DSN()},
		},
		Sink: ingestion.Ref{
			Provider: "iceberg",
			Config: map[string]any{
				"warehouse":  "s3://" + lake.Bucket + "/",
				"namespace":  "tpch_e2e",
				"write_mode": "replace",
				"catalog": map[string]any{
					"type": "rest",
					"uri":  restURI,
				},
			},
		},
		Resources: resources,
		Options: ingestion.RunOptions{
			BatchMaxRows:        500,
			SnapshotParallelism: 2,
		},
	})
	if err != nil {
		t.Fatalf("submit run: %v", err)
	}

	final := waitRunStatus(t, ctx, store, runID, ingestion.RunCompleted, ingestion.RunFailed, ingestion.RunPartial)
	if final.Status != ingestion.RunCompleted {
		t.Fatalf("run %s status = %v, error = %q", runID, final.Status, final.Error)
	}

	for _, table := range resources {
		srcCount := countPostgres(t, ctx, pg.Pool(), table)
		dstCount := countTrino(t, ctx, lake.DB, "tpch_e2e", table)
		if dstCount != srcCount {
			t.Fatalf("%s row count = %d in iceberg, want %d from postgres", table, dstCount, srcCount)
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

func waitRunStatus(t *testing.T, ctx context.Context, store ingestion.DataStore, id ingestion.RunID, statuses ...ingestion.RunStatus) ingestion.RunState {
	t.Helper()
	want := map[ingestion.RunStatus]bool{}
	for _, status := range statuses {
		want[status] = true
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, id)
		if err == nil && want[state.Status] {
			return state
		}
		select {
		case <-ctx.Done():
			if err == nil {
				t.Fatalf("timed out waiting for run %s status in %v; last status %v error %q", id, statuses, state.Status, state.Error)
			}
			t.Fatalf("timed out waiting for run %s status in %v: %v", id, statuses, err)
		case <-ticker.C:
		}
	}
}

func countPostgres(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) int64 {
	t.Helper()
	var n int64
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n); err != nil {
		t.Fatalf("count postgres %s: %v", table, err)
	}
	return n
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
