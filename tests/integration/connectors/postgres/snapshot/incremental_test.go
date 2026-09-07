//go:build integration

package snapshot_test

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestPostgresIncrementalShardedBackfillAndChanges(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgres(t)
	_, err := pg.Pool().Exec(ctx, `
		CREATE TABLE incremental_users (
			id bigint PRIMARY KEY,
			name text NOT NULL,
			updated_at timestamptz NOT NULL
		);
		INSERT INTO incremental_users (id, name, updated_at)
		SELECT g, 'user-' || g, timestamptz '2026-08-01 00:00:00+00' + g * interval '1 second'
		FROM generate_series(1, 10000) g;
	`)
	if err != nil {
		t.Fatal(err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "shard_pages": 1, "page_size": 250,
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	cursors := map[string]filament.ResourceCursorConfig{
		"incremental_users": {Field: "updated_at", LookbackSeconds: 1},
	}
	plan, err := src.PlanIncremental(ctx, []string{"incremental_users"}, nil, cursors)
	if err != nil {
		t.Fatal(err)
	}
	backfill, ok := checkpoint.ParseKeyset(plan["incremental_users"])
	if !ok || backfill.Mode != checkpoint.ModeIncrementalBackfill || len(backfill.Shards) < 2 {
		t.Fatalf("backfill plan = %#v", plan["incremental_users"].Raw())
	}
	initial := &testutil.CollectSink{}
	if err := src.ExtractFrom(ctx, initial, filament.ExtractOpts{
		Resources: []string{"incremental_users"}, Parallelism: 4,
	}, plan); err != nil {
		t.Fatal(err)
	}
	if len(initial.Records) != 10000 {
		t.Fatalf("backfill rows = %d, want 10000", len(initial.Records))
	}

	promoted, ok := checkpoint.PromoteIncrementalBackfill(plan["incremental_users"])
	if !ok {
		t.Fatal("backfill checkpoint did not promote")
	}
	_, err = pg.Pool().Exec(ctx, `
		UPDATE incremental_users SET name = 'changed', updated_at = '2026-08-03 00:00:00+00' WHERE id = 1;
		INSERT INTO incremental_users (id, name, updated_at) VALUES (10001, 'new', '2026-08-03 00:00:01+00');
	`)
	if err != nil {
		t.Fatal(err)
	}
	next, err := src.PlanIncremental(ctx, []string{"incremental_users"}, map[string]filament.Checkpoint{"incremental_users": promoted}, cursors)
	if err != nil {
		t.Fatal(err)
	}
	changes := &testutil.CollectSink{}
	if err := src.ExtractFrom(ctx, changes, filament.ExtractOpts{
		Resources: []string{"incremental_users"}, Parallelism: 4,
	}, next); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, record := range changes.Records {
		seen[record.ID] = true
	}
	if !seen["1"] || !seen["10001"] {
		t.Fatalf("incremental ids = %v, want updated 1 and inserted 10001", seen)
	}
}
