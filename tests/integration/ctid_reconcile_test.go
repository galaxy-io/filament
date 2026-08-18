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

// TestCtidReconcileHorizon isolates the horizon-compare reconciliation pass. It plans a ctid
// read (stamping the run-start xmin horizon H1), then marks EVERY shard Done — so on the next
// ExtractFrom no block range is read fresh; the only way a row reaches the sink is the
// reconcile/tail pass. It then churns a handful of rows (all committing after H1) and asserts
// exactly those churned rows are re-delivered and the thousands of untouched rows are not —
// proving the age(xmin) <= age(H1) filter selects post-horizon rows, not a full rescan.
func TestCtidReconcileHorizon(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t)

	const n = 5000
	ddl := `
		CREATE TABLE phys (id bigint PRIMARY KEY, v int NOT NULL, payload text NOT NULL);
		INSERT INTO phys (id, v, payload)
		SELECT g, 0, 'p-' || g FROM generate_series(1, ` + fmt.Sprint(n) + `) g;`
	if _, err := pg.Pool().Exec(ctx, ddl); err != nil {
		t.Fatalf("seed phys: %v", err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "scan_strategy": "ctid", "shard_pages": 1})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	// Plan stamps H1 (and the filenode/relpages). Everything seeded so far is pre-H1.
	plan, err := src.PlanResume(ctx, []string{"phys"}, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	ks, ok := checkpoint.ParseKeyset(plan["phys"])
	if !ok || ks.Mode != checkpoint.ModeCtid || len(ks.Shards) < 2 {
		t.Fatalf("expected a multi-shard ctid plan, got %+v (ok=%v)", plan["phys"], ok)
	}

	// Simulate a fully-read prior run: every block range complete.
	for i := range ks.Shards {
		ks.Shards[i].Done = true
	}
	donePlan := map[string]filament.Checkpoint{"phys": ks.ToCheckpoint("phys")}

	// Churn after H1: a non-PK update, a PK move, a fresh insert, and a delete.
	churn := []string{
		"UPDATE phys SET v = 1 WHERE id = 100",       // non-PK update → new tuple, post-H1 xmin
		"UPDATE phys SET id = 900001 WHERE id = 200", // PK move → new key 900001
		"INSERT INTO phys (id, v, payload) VALUES (900002, 0, 'new')",
		"DELETE FROM phys WHERE id = 300",
	}
	for _, q := range churn {
		if _, err := pg.Pool().Exec(ctx, q); err != nil {
			t.Fatalf("churn %q: %v", q, err)
		}
	}

	sink := &collectSink{}
	if err := src.ExtractFrom(ctx, sink, filament.ExtractOpts{Resources: []string{"phys"}, Parallelism: 4}, donePlan); err != nil {
		t.Fatalf("extract from (reconcile): %v", err)
	}

	got := make(map[string]bool)
	for _, r := range sink.recs {
		if r.Drained {
			continue
		}
		got[r.ID] = true
	}

	// The churned rows (by their CURRENT key) must be re-delivered.
	for _, id := range []string{"100", "900001", "900002"} {
		if !got[id] {
			t.Errorf("reconcile dropped churned row id=%s", id)
		}
	}
	// The deleted row's tuple is dead under the read snapshot → not delivered.
	if got["300"] {
		t.Errorf("reconcile delivered a deleted row id=300")
	}
	// The filter must not degrade into a full rescan: only post-horizon rows come back, so
	// the delivered set is a tiny fraction of the 5000 untouched rows.
	if len(got) > 50 {
		t.Errorf("reconcile delivered %d rows; expected only the post-horizon churn (filter not applied?)", len(got))
	}
}

// TestCtidRewriteGuard checks the resume rewrite guard: a VACUUM FULL between runs changes
// the heap filenode, so the persisted block-range cursor no longer maps to the same rows.
// PlanResume must detect the mismatch and return a FRESH plan (no Done shards) so the table
// is re-read whole, rather than trusting the stale completed ranges.
func TestCtidRewriteGuard(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t)

	const n = 3000
	ddl := `
		CREATE TABLE rw (id bigint PRIMARY KEY, payload text NOT NULL);
		INSERT INTO rw (id, payload) SELECT g, 'p-' || g FROM generate_series(1, ` + fmt.Sprint(n) + `) g;`
	if _, err := pg.Pool().Exec(ctx, ddl); err != nil {
		t.Fatalf("seed rw: %v", err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "scan_strategy": "ctid", "shard_pages": 1})); err != nil {
		t.Fatalf("configure: %v", err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	plan, err := src.PlanResume(ctx, []string{"rw"}, nil)
	if err != nil {
		t.Fatalf("plan resume: %v", err)
	}
	ks, _ := checkpoint.ParseKeyset(plan["rw"])
	for i := range ks.Shards {
		ks.Shards[i].Done = true
	}
	prev := map[string]filament.Checkpoint{"rw": ks.ToCheckpoint("rw")}

	// Rewrite the heap → new filenode → stale block ranges.
	if _, err := pg.Pool().Exec(ctx, "VACUUM (FULL) rw"); err != nil {
		t.Fatalf("vacuum full: %v", err)
	}

	replan, err := src.PlanResume(ctx, []string{"rw"}, prev)
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	got, ok := checkpoint.ParseKeyset(replan["rw"])
	if !ok || got.Mode != checkpoint.ModeCtid {
		t.Fatalf("re-plan not ctid: %+v", replan["rw"])
	}
	for i, sh := range got.Shards {
		if sh.Done {
			t.Errorf("rewrite guard did not fire: shard %d still Done after VACUUM FULL", i)
		}
	}
}
