//go:build integration

package transform_test

// Postgres → transform → Postgres over the seeded tables. Every table gets the
// same steps, so each step kind and both kernel families run over the full row
// count. SEED_SCALE=large pushes a million rows through. FILAMENT_PROFILE_DIR
// records a CPU profile and heap snapshot of exactly the run, seeding excluded.

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/profiling"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	seedpg "github.com/galaxy-io/filament/tests/testcontainers/seed/postgres"
)

// steps is applied to every seeded table: rename, drop, a conditional set, and
// two new columns from the date kernels.
const steps = `
    steps:
      - rename: {tenant: tenant_id}
      - drop: [payload]
      - set:
          tenant_id: "eu"
        where: {eq: [{col: tenant_id}, "t0"]}
      - set:
          updated_year: {year: {col: updated_at}}
          updated_day: {day: {col: updated_at}}
`

func definition(resources []string) string {
	var sb strings.Builder
	sb.WriteString("version: 1\nresources:\n")
	for _, r := range resources {
		fmt.Fprintf(&sb, "  %s:%s", r, steps)
	}
	return sb.String()
}

// stack is one in-process engine over a shared Postgres: the same modules the
// service mounts, with an in-memory bus and store.
type stack struct {
	orch  *orchestrator.Module
	store filament.DataStore
	close func()
}

func mount(tb testing.TB, ctx context.Context) stack {
	tb.Helper()
	sources := registry.NewSources()
	sources.Register("postgres", func() filament.Source { return pgsource.New() })
	sinks := registry.NewSinks()
	sinks.Register("postgres", func() filament.Sink { return pgsink.New() })

	bus := inproc.New()
	store := sqlite.NewMemory()
	orch := orchestrator.New()
	deps := module.Deps{Bus: bus, DataStore: store, Sources: sources, Sinks: sinks}
	mods, err := module.MountAll(ctx, deps, tracker.New(), engine.New(), orch)
	if err != nil {
		tb.Fatalf("mount: %v", err)
	}
	h := host.New(bus)
	if err := h.Run(ctx, mods...); err != nil {
		tb.Fatalf("run: %v", err)
	}
	return stack{orch: orch, store: store, close: func() { _ = h.Close(); _ = bus.Close() }}
}

// seedTables seeds the tier and returns its manifest and resource names.
func seedTables(tb testing.TB, ctx context.Context, pg *testcontainers.PG, spec seed.Spec) (seed.Manifest, []string) {
	tb.Helper()
	manifest, err := seedpg.Apply(ctx, pg.Pool(), spec)
	if err != nil {
		tb.Fatalf("seed: %v", err)
	}
	resources := make([]string, spec.Tables)
	for i := range resources {
		resources[i] = seed.TableName(i)
	}
	return manifest, resources
}

// runOnce drops the destination schema, submits one full-upsert run of every
// resource with the given definition (empty → no transform), and blocks for a
// terminal status. It returns the state and the submit → terminal wall time.
func runOnce(tb testing.TB, ctx context.Context, st stack, pg *testcontainers.PG, resources []string, def string) (filament.RunState, time.Duration) {
	tb.Helper()
	if _, err := pg.Pool().Exec(ctx, "DROP SCHEMA IF EXISTS dst CASCADE"); err != nil {
		tb.Fatalf("wipe dst schema: %v", err)
	}
	started := time.Now()
	id, err := st.orch.Submit(ctx, filament.RunSubmission{Request: filament.RunRequest{
		Tenant:         "t1",
		Source:         filament.Ref{Connector: "postgres", Config: map[string]any{"dsn": pg.DSN()}},
		Sink:           filament.Ref{Connector: "postgres", Config: map[string]any{"dsn": pg.DSN(), "schema": "dst"}},
		Resources:      resources,
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullUpsert},
		Options:        filament.RunOptions{SnapshotParallelism: 4, BatchMaxRows: 10_000},
		Transform:      def,
	}})
	if err != nil {
		tb.Fatalf("submit: %v", err)
	}
	final := testutil.WaitForStatuses(tb, ctx, st.store, "t1", id, filament.RunCompleted, filament.RunFailed)
	return final, time.Since(started)
}

func TestPostgresTransform(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Minute)
	defer cancel()

	pg := testcontainers.SharedPostgres(t)
	manifest, resources := seedTables(t, ctx, pg, seed.Default())
	st := mount(t, ctx)
	defer st.close()

	// The profile brackets submit → completed only. Everything above is setup.
	var session *profiling.Session
	if dir := os.Getenv("FILAMENT_PROFILE_DIR"); dir != "" {
		var err error
		if session, err = profiling.Start(profiling.DefaultConfig(dir)); err != nil {
			t.Fatalf("profiling: %v", err)
		}
	}
	final, elapsed := runOnce(t, ctx, st, pg, resources, definition(resources))
	if session != nil {
		if err := session.Stop(); err != nil {
			t.Fatalf("profiling stop: %v", err)
		}
		for profile, path := range session.Files() {
			t.Logf("%s profile: %s", profile, path)
		}
	}
	if final.Status != filament.RunCompleted {
		t.Fatalf("status = %v (err %q), want completed", final.Status, final.Error)
	}

	var total int64
	for _, tbl := range manifest.Tables {
		total += tbl.Rows
		assertTransformed(t, ctx, pg.Pool(), tbl.Name)
	}
	t.Logf("%d rows across %d tables in %s (%.0f rows/s)", total, len(manifest.Tables), elapsed.Round(time.Millisecond), float64(total)/elapsed.Seconds())
}

// assertTransformed checks the destination's shape and aggregates against the
// same steps applied in SQL to the source, so it holds at any row count.
func assertTransformed(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) {
	t.Helper()
	var columns []string
	rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = 'dst' AND table_name = $1 ORDER BY ordinal_position`, table)
	if err != nil {
		t.Fatalf("%s: columns: %v", table, err)
	}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, c)
	}
	rows.Close()
	has := func(name string) bool {
		for _, c := range columns {
			if c == name {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"id", "tenant_id", "seq", "amount", "updated_at", "updated_year", "updated_day"} {
		if !has(want) {
			t.Errorf("%s: missing column %q in %v", table, want, columns)
		}
	}
	for _, gone := range []string{"tenant", "payload"} {
		if has(gone) {
			t.Errorf("%s: column %q should be gone, have %v", table, gone, columns)
		}
	}

	type agg struct {
		n, seq, amount, year, day, eu int64
	}
	read := func(sql string) agg {
		var a agg
		if err := pool.QueryRow(ctx, sql).Scan(&a.n, &a.seq, &a.amount, &a.year, &a.day, &a.eu); err != nil {
			t.Fatalf("%s: %v\n%s", table, err, sql)
		}
		return a
	}
	want := read(fmt.Sprintf(`SELECT count(*), coalesce(sum(seq),0), coalesce(sum(amount),0),
		coalesce(sum(extract(year from updated_at at time zone 'UTC')),0)::bigint,
		coalesce(sum(extract(day from updated_at at time zone 'UTC')),0)::bigint,
		count(*) filter (where tenant = 't0') FROM public.%q`, table))
	got := read(fmt.Sprintf(`SELECT count(*), coalesce(sum(seq),0), coalesce(sum(amount),0),
		coalesce(sum(updated_year),0), coalesce(sum(updated_day),0),
		count(*) filter (where tenant_id = 'eu') FROM dst.%q`, table))
	if got != want {
		t.Errorf("%s: dst %+v, want %+v", table, got, want)
	}
	var other int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM dst.%q WHERE tenant_id NOT IN ('eu','t1','t2','t3')`, table)).Scan(&other); err != nil {
		t.Fatal(err)
	}
	if other != 0 {
		t.Errorf("%s: %d rows with an unexpected tenant_id", table, other)
	}
}
