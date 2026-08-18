//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// TestSourceUUIDKeysetSharded verifies the percentile_disc sampler splits a randomly-keyed
// uuid table across shards (the case previously stuck on a single open shard) and reads it
// concurrently with no gaps or duplicates. shard_pages is forced low so a small table fans
// out, exercising the sampled non-integer boundaries + the per-shard leading-column bound.
func TestSourceUUIDKeysetSharded(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t)

	const want = 4000
	ddl := `
		CREATE TABLE uuid_keyed (
			id      uuid PRIMARY KEY,
			payload text NOT NULL
		);
		INSERT INTO uuid_keyed (id, payload)
		SELECT md5(g::text)::uuid, 'p-' || g
		FROM generate_series(1, ` + fmt.Sprint(want) + `) g;`
	if _, err := pg.Pool().Exec(ctx, ddl); err != nil {
		t.Fatalf("seed uuid_keyed: %v", err)
	}

	readSharded(t, ctx, pg, "uuid_keyed", want)
}

// TestSourceTextKeysetSharded does the same for a TEXT-leading composite key, exercising
// the text boundary casts and the full tuple cursor at sampled range boundaries.
func TestSourceTextKeysetSharded(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t)

	const nN, vN = 400, 10
	const want = nN * vN
	ddl := `
		CREATE TABLE text_keyed (
			name    text NOT NULL,
			version int  NOT NULL,
			payload text NOT NULL,
			PRIMARY KEY (name, version)
		);
		INSERT INTO text_keyed (name, version, payload)
		SELECT 'k' || lpad(gn::text, 6, '0'), gv, 'p-' || gn || '-' || gv
		FROM generate_series(1, ` + fmt.Sprint(nN) + `) gn,
		     generate_series(1, ` + fmt.Sprint(vN) + `) gv;`
	if _, err := pg.Pool().Exec(ctx, ddl); err != nil {
		t.Fatalf("seed text_keyed: %v", err)
	}

	readSharded(t, ctx, pg, "text_keyed", want)
}

// TestRouteProbe verifies the correlation probe routes a physically-ordered serial key to
// keyset (heap order tracks key order) and a randomly-distributed uuid key to bitmap (the
// default for random immutable keys) — the live half of the router whose decision table is
// unit-tested in TestChooseMode.
func TestRouteProbe(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t)

	// ordered: serial PK inserted in order → correlation ≈ 1. random: uuid PK → correlation ≈ 0.
	ddl := `
		CREATE TABLE ordered_keyed (id bigserial PRIMARY KEY, payload text NOT NULL);
		INSERT INTO ordered_keyed (payload) SELECT 'p-' || g FROM generate_series(1, 20000) g;
		CREATE TABLE random_keyed (id uuid PRIMARY KEY, payload text NOT NULL);
		INSERT INTO random_keyed (id, payload) SELECT gen_random_uuid(), 'p-' || g FROM generate_series(1, 20000) g;`
	if _, err := pg.Pool().Exec(ctx, ddl); err != nil {
		t.Fatalf("seed: %v", err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN()})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	for _, tc := range []struct {
		table string
		want  pgsource.ReadMode
	}{
		{"ordered_keyed", pgsource.ModeKeyset},
		{"random_keyed", pgsource.ModeBitmap},
	} {
		mode, corr, ok := src.Route(ctx, tc.table, pgsource.RouteSignals{})
		if mode != tc.want {
			t.Errorf("Route(%s) = %v (corr=%.3f, haveCorr=%v), want %v", tc.table, mode, corr, ok, tc.want)
		}
	}
}

// TestSourceBitmapSharded reads a uuid table in bitmap mode (unordered sub-range scans) and
// confirms exact coverage with no duplicates, ignoring the per-shard Drained sentinels the
// bitmap reader emits for completion accounting (the pipeline filters them; a raw sink sees
// them).
func TestSourceBitmapSharded(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t)

	const want = 4000
	ddl := `
		CREATE TABLE bm_keyed (id uuid PRIMARY KEY, payload text NOT NULL);
		INSERT INTO bm_keyed (id, payload)
		SELECT md5(g::text)::uuid, 'p-' || g FROM generate_series(1, ` + fmt.Sprint(want) + `) g;`
	if _, err := pg.Pool().Exec(ctx, ddl); err != nil {
		t.Fatalf("seed bm_keyed: %v", err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "scan_strategy": "bitmap", "shard_pages": 1})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	plan, err := src.PlanResume(ctx, []string{"bm_keyed"}, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["bm_keyed"])
	if !ok || ks.Mode != checkpoint.ModeBitmap || len(ks.Shards) < 2 {
		t.Fatalf("expected a multi-shard bitmap plan, got %+v (ok=%v)", plan["bm_keyed"], ok)
	}

	sink := &collectSink{}
	if err := src.ExtractFrom(ctx, sink, filament.ExtractOpts{Resources: []string{"bm_keyed"}, Parallelism: 4}, plan); err != nil {
		t.Fatalf("extract from: %v", err)
	}

	seen := make(map[string]bool, want)
	for _, r := range sink.recs {
		if r.Drained {
			continue // completion sentinel, not a data row
		}
		if seen[r.ID] {
			t.Fatalf("duplicate record id %q across bitmap shards", r.ID)
		}
		seen[r.ID] = true
	}
	if len(seen) != want {
		t.Fatalf("distinct ids = %d, want %d (gap or dropped row in a bitmap sub-range)", len(seen), want)
	}
}

// readSharded configures the source with a low shard_pages, asserts PlanResume produced a
// multi-shard plan (the sampler fired), reads it concurrently via ExtractFrom, and checks
// exact count with no duplicate record ids — a gap or double-read across a sampled boundary
// would fail one of these.
func readSharded(t *testing.T, ctx context.Context, pg *testcontainers.PG, table string, want int) {
	t.Helper()
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "shard_pages": 2})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	plan, err := src.PlanResume(ctx, []string{table}, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan[table])
	if !ok || len(ks.Shards) < 2 {
		t.Fatalf("expected a multi-shard sampled plan for %s, got %+v (ok=%v)", table, plan[table], ok)
	}

	sink := &collectSink{}
	if err := src.ExtractFrom(ctx, sink, filament.ExtractOpts{Resources: []string{table}, Parallelism: 4}, plan); err != nil {
		t.Fatalf("extract from: %v", err)
	}

	if len(sink.recs) != want {
		t.Fatalf("read %d records, want %d (gap or dropped row across a sampled boundary)", len(sink.recs), want)
	}
	seen := make(map[string]bool, want)
	for _, r := range sink.recs {
		if seen[r.ID] {
			t.Fatalf("duplicate record id %q across shards", r.ID)
		}
		seen[r.ID] = true
	}
	if len(seen) != want {
		t.Fatalf("distinct ids = %d, want %d", len(seen), want)
	}
}
