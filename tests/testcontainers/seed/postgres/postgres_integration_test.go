//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"testing"

	tc "github.com/galaxy-io/filament/tests/testcontainers"
	pgseeder "github.com/galaxy-io/filament/tests/testcontainers/seed/postgres"

	"github.com/galaxy-io/filament/tests/testcontainers/seed"
)

// TestSeedAndSnapshotRestore proves the seed + snapshot/restore machinery:
// generation lands the right rows, the fingerprint is deterministic, and
// Restore rewinds a mutated database to the seeded state without re-seeding.
func TestSeedAndSnapshotRestore(t *testing.T) {
	ctx := context.Background()
	pg := tc.Postgres(t)

	spec := seed.Default()
	first, err := pgseeder.Apply(ctx, pg.Pool(), spec)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(first.Tables) != spec.Tables {
		t.Fatalf("tables = %d, want %d", len(first.Tables), spec.Tables)
	}
	for _, tbl := range first.Tables {
		if tbl.Rows != int64(spec.RowsPerTable) {
			t.Errorf("%s rows = %d, want %d", tbl.Name, tbl.Rows, spec.RowsPerTable)
		}
	}

	// Determinism: re-applying the same spec reproduces identical fingerprints.
	second, err := pgseeder.Apply(ctx, pg.Pool(), spec)
	if err != nil {
		t.Fatalf("Apply (re-run): %v", err)
	}
	for i := range first.Tables {
		if first.Tables[i].Checksum != second.Tables[i].Checksum {
			t.Errorf("%s checksum not deterministic: %s vs %s",
				first.Tables[i].Name, first.Tables[i].Checksum, second.Tables[i].Checksum)
		}
	}

	// Snapshot the seeded state, then mutate.
	pg.Snapshot(t)
	t0 := seed.TableName(0)
	if _, err := pg.Pool().Exec(ctx, fmt.Sprintf("DELETE FROM %q", t0)); err != nil {
		t.Fatalf("mutate: %v", err)
	}
	if got := countRows(t, ctx, pg, t0); got != 0 {
		t.Fatalf("after delete %s rows = %d, want 0", t0, got)
	}

	// Restore rewinds to the seeded state — no re-seed.
	pg.Restore(t)
	if got := countRows(t, ctx, pg, t0); got != int64(spec.RowsPerTable) {
		t.Fatalf("after restore %s rows = %d, want %d", t0, got, spec.RowsPerTable)
	}
}

func countRows(t *testing.T, ctx context.Context, pg *tc.PG, table string) int64 {
	t.Helper()
	var n int64
	if err := pg.Pool().QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %q", table)).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}
