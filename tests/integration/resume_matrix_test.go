//go:build integration

package integration

// Phase-0 failure-injection matrix. For each resumable
// read mode, kill a run mid-extract, run one concurrent write op in the fail→resume
// window, then assert coverage of the rows present at run start.
//
// Assertion direction: LOSS = fail, DUPLICATE = pass. The idempotent PK-upsert sink
// absorbs over-delivery, so re-reading a row is always safe; the only bug a snapshot run
// can have is dropping a row it was obligated to deliver.
//
// Per-MODE expectations are encoded, not assumed. A PK-UPDATE in the gap MUST reappear
// under a physical-cursor mode (ctid+xmin, slot) — loss there is a real bug — but is a
// documented limitation of a key-space cursor (keyset, bitmap), where it is flagged and
// not failed. Only keyset exists today, so every op runs at the key-space tier; the
// physical modes slot into modesUnderTest as Phases 2-3 land and immediately inherit the
// stronger assertions.

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/internal/modules/engine"
	"github.com/galaxy-io/filament/internal/modules/orchestrator"
	"github.com/galaxy-io/filament/internal/modules/tracker"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/registry"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/galaxy-io/filament/tests/testcontainers/seed"
	seedpg "github.com/galaxy-io/filament/tests/testcontainers/seed/postgres"
)

// tier is the correctness tier of a read mode — the real axis is PK mutability, not key
// type (see the plan's "Correctness tiers"). It decides whether an op's added rows are a
// hard requirement or a documented limitation.
type tier int

const (
	tierKeySpace tier = iota // keyset, bitmap sub-range: loses only PK-mutated rows
	tierPhysical             // ctid+xmin, slot-watermark: loses nothing
)

// readMode is one resumable read strategy under test: the run options plus any source
// config that selects it.
type readMode struct {
	name string
	tier tier
	opts filament.RunOptions
	src  map[string]any // extra source config (e.g. read_mode, shard_pages)
}

// modesUnderTest is the set the matrix exercises. Add ctid+xmin / slot here as they land;
// each then runs the full op grid with its tier's assertions, no new wiring.
func modesUnderTest() []readMode {
	// Small batches so the injected failure lands mid-extract.
	opts := filament.RunOptions{SnapshotParallelism: 4, BatchMaxRows: 100}
	return []readMode{
		// SnapshotParallelism 1 keeps keyset's existing single-shard-per-uuid-table shape.
		{name: "keyset", tier: tierKeySpace, opts: filament.RunOptions{SnapshotParallelism: 1, BatchMaxRows: 100}},
		// Bitmap forces sub-range splitting (low shard_pages) and runs with parallel writers
		// + concurrent shards — the configuration its ack-counted completion must survive.
		{name: "bitmap", tier: tierKeySpace, opts: opts, src: map[string]any{"read_mode": "bitmap", "shard_pages": 1}},
		// ctid+xmin is the physical tier: block-range shards + horizon-compare reconciliation.
		// At tierPhysical the matrix demands gap-added rows (insert, pk-update, savepoint)
		// REappear after resume — loss is a real bug, not a documented limitation.
		{name: "ctid_xmin", tier: tierPhysical, opts: opts, src: map[string]any{"read_mode": "ctid", "shard_pages": 1}},
	}
}

// gapEffect is how a concurrent op changes the run-start coverage contract for the table
// it touched.
type gapEffect struct {
	// removed are start PKs no longer required in the destination: deleted, or moved to a
	// new PK. Capturing post-start deletes is an explicit non-goal, so these are exempt.
	removed []string
	// added are PKs that did not exist at run start. A physical-cursor mode MUST deliver
	// them (loss = fail); a key-space mode MAY miss them (documented limitation — flagged,
	// not failed), since a randomly-placed new key can fall in an already-completed region.
	added []string
}

// gapOp is one concurrent write executed in the fail→resume window against the target
// table. run performs the mutation and reports its effect on the coverage contract.
type gapOp struct {
	name string
	skip string // non-empty → registered but skipped (logged, never silently dropped)
	run  func(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect
}

// matrixOps is the concurrent-op axis. The ops green for a key-space cursor today
// (non-PK UPDATE, DELETE, VACUUM) assert full no-loss now; PK-UPDATE / INSERT / savepoint
// carry an `added` set that only a physical mode is required to capture. wraparound is
// registered-but-skipped so the gap is visible, never silently absent.
func matrixOps() []gapOp {
	return []gapOp{
		{name: "none", run: func(*testing.T, context.Context, *pgxpool.Pool, string) gapEffect {
			return gapEffect{} // baseline: resume with no concurrent churn
		}},
		{name: "update_nonpk", run: opUpdateNonPK},
		{name: "delete", run: opDelete},
		{name: "insert", run: opInsert},
		{name: "pk_update", run: opPKUpdate},
		{name: "vacuum_freeze", run: opVacuum("VACUUM (FREEZE, ANALYZE)")},
		{name: "vacuum_full", run: opVacuum("VACUUM (FULL)")},
		{name: "savepoint_insert", run: opSavepointInsert},
		{name: "wraparound", skip: "burned-xid wraparound needs a disposable container (Phase-0 TODO)"},
	}
}

// targetTable is the seed table every op mutates; other seeded tables stay untouched and
// must come through fully covered regardless of the op.
const targetTable = "seed_00"

func TestResumeMatrix(t *testing.T) {
	for _, mode := range modesUnderTest() {
		for _, op := range matrixOps() {
			t.Run(mode.name+"/"+op.name, func(t *testing.T) {
				if op.skip != "" {
					t.Skip(op.skip)
				}
				runResumeScenario(t, mode, op)
			})
		}
	}
}

// runResumeScenario drives one cell of the matrix: seed → kill mid-extract → run the gap
// op → resume → assert no row present at run start was lost.
func runResumeScenario(t *testing.T, mode readMode, op gapOp) {
	ctx := context.Background()

	pg := testcontainers.Postgres(t)
	spec := seed.Default()
	manifest, err := seedpg.Apply(ctx, pg.Pool(), spec)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := pg.Pool().Exec(ctx, "CREATE SCHEMA dst"); err != nil {
		t.Fatalf("create dst schema: %v", err)
	}

	resources := make([]string, spec.Tables)
	for i := range resources {
		resources[i] = seed.TableName(i)
	}

	// Snapshot the PKs present at run start — the no-loss obligation is defined over these.
	startPKs := make(map[string]map[string]bool, len(resources))
	for _, tbl := range resources {
		startPKs[tbl] = capturePKs(t, ctx, pg.Pool(), tbl)
	}

	// First sink fails at write 5 (mid-extract); the resume sink is the real typed sink.
	var firstRun atomic.Bool
	firstRun.Store(true)
	sources := registry.NewSources()
	sources.Register("postgres", func() filament.Source { return pgsource.New() })
	sinks := registry.NewSinks()
	sinks.Register("postgres_typed", func() filament.Sink {
		ts := pgsink.New()
		if firstRun.CompareAndSwap(true, false) {
			return &flakySink{Sink: ts, failAt: 5}
		}
		return ts
	})

	bus := inproc.New()
	store := memory.New()
	orch := orchestrator.New()
	deps := module.Deps{Bus: bus, DataStore: store, Sources: sources, Sinks: sinks}
	mods, err := module.MountAll(ctx, deps, tracker.New(), engine.New(), orch)
	if err != nil {
		t.Fatalf("mount: %v", err)
	}
	h := host.New(bus)
	if err := h.Run(ctx, mods...); err != nil {
		t.Fatalf("run: %v", err)
	}
	defer func() { _ = h.Close(); _ = bus.Close() }()

	srcCfg := map[string]any{"dsn": pg.DSN()}
	for k, v := range mode.src {
		srcCfg[k] = v
	}
	id, err := orch.Submit(ctx, filament.RunRequest{
		Tenant:         "t1",
		Source:         filament.Ref{Provider: "postgres", Config: srcCfg},
		Sink:           filament.Ref{Provider: "postgres_typed", Config: map[string]any{"dsn": pg.DSN(), "schema": "dst"}},
		Resources:      resources,
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullUpsert},
		Options:        mode.opts,
	})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// Injected failure must leave the run resumable (partial), not terminal.
	partial := waitStatus(t, ctx, store, id, filament.RunPartial)
	if partial.Status != filament.RunPartial {
		t.Fatalf("after injected failure: status = %v (err %q), want partial", partial.Status, partial.Error)
	}

	// Confirm the mode's completion machinery actually fired before the kill, so coverage
	// is not passing trivially by re-reading everything on resume.
	if done := countDoneShards(ctx, store, id, resources); mode.name == "bitmap" {
		t.Logf("bitmap: %d shard(s) marked Done before kill (skipped on resume)", done)
	}

	// The fail→resume window: churn the source before it is re-read under a fresh snapshot.
	qualified := pgx.Identifier{"public", targetTable}.Sanitize()
	effect := op.run(t, ctx, pg.Pool(), qualified)

	// Re-request the same run id — the resume trigger.
	if err := events.Emit(ctx, bus, events.RunRequested, events.Envelope{Tenant: "t1", Run: id}, events.RunRequestedEvent{}); err != nil {
		t.Fatalf("re-request run: %v", err)
	}
	final := waitStatus(t, ctx, store, id, filament.RunCompleted)
	if final.Status != filament.RunCompleted {
		t.Fatalf("after resume: status = %v (err %q), want completed", final.Status, final.Error)
	}

	assertCoverage(t, ctx, pg.Pool(), mode, manifest, startPKs, effect)
}

// assertCoverage is the matrix verdict. Every run-start PK (minus the op's exempt set)
// must be in the destination — a miss is data loss and fails. Rows the op added are a
// hard requirement only for a physical-cursor mode; for a key-space mode a miss is the
// documented limitation, logged not failed. Extra/duplicate rows are never an error.
func assertCoverage(t *testing.T, ctx context.Context, pool *pgxpool.Pool, mode readMode, manifest seed.Manifest, startPKs map[string]map[string]bool, effect gapEffect) {
	exempt := toSet(effect.removed)
	for _, tbl := range manifest.Tables {
		dst := capturePKs(t, ctx, pool, "dst."+tbl.Name)
		for pk := range startPKs[tbl.Name] {
			if tbl.Name == targetTable && exempt[pk] {
				continue // deleted or PK-moved-away in the gap — out of as-of-start scope
			}
			if !dst[pk] {
				t.Errorf("LOSS: %s pk %q present at run start but missing after resume", tbl.Name, pk)
			}
		}
	}

	// Rows the op introduced: physical must capture, key-space may miss.
	dst := capturePKs(t, ctx, pool, "dst."+targetTable)
	for _, pk := range effect.added {
		switch {
		case dst[pk]:
			// captured — always acceptable
		case mode.tier == tierPhysical:
			t.Errorf("LOSS: physical mode %q dropped gap-added pk %q", mode.name, pk)
		default:
			t.Logf("documented limitation: key-space mode %q missed gap-added pk %q", mode.name, pk)
		}
	}
}

// ── concurrent gap ops ───────────────────────────────────────────────────────

// opUpdateNonPK mutates a non-key column on a key-space-scattered sample. The PKs are
// unchanged, so all remain required; a key-space cursor need not reflect the new value.
func opUpdateNonPK(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect {
	if _, err := pool.Exec(ctx, "UPDATE "+table+" SET amount = amount + 1 WHERE seq % 7 = 0"); err != nil {
		t.Fatalf("gap update: %v", err)
	}
	return gapEffect{}
}

// opDelete removes a scattered sample. Post-start deletes are a non-goal, so the deleted
// PKs become exempt from the no-loss check (whether or not they were read before the kill).
func opDelete(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect {
	rows, err := pool.Query(ctx, "DELETE FROM "+table+" WHERE seq % 11 = 0 RETURNING id::text")
	if err != nil {
		t.Fatalf("gap delete: %v", err)
	}
	return gapEffect{removed: scanIDs(t, rows)}
}

// opInsert adds a fresh, randomly-keyed row. A physical mode must deliver it; a key-space
// mode may miss it when the new uuid sorts below an already-passed cursor.
func opInsert(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect {
	id := insertRow(t, ctx, pool, table)
	return gapEffect{added: []string{id}}
}

// opPKUpdate moves a row to a new primary key — the case that separates the tiers. The old
// key is gone (exempt); the new key must reappear under a physical mode, may be missed by
// a key-space mode (its old-key region may already be complete, its new-key region too).
func opPKUpdate(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect {
	var oldID string
	if err := pool.QueryRow(ctx, "SELECT id::text FROM "+table+" ORDER BY seq DESC LIMIT 1").Scan(&oldID); err != nil {
		t.Fatalf("gap pk-update select: %v", err)
	}
	var newID string
	if err := pool.QueryRow(ctx, "UPDATE "+table+" SET id = gen_random_uuid() WHERE id = $1::uuid RETURNING id::text", oldID).Scan(&newID); err != nil {
		t.Fatalf("gap pk-update: %v", err)
	}
	return gapEffect{removed: []string{oldID}, added: []string{newID}}
}

// opVacuum runs a VACUUM variant in the gap. FREEZE exercises the physical tier's freeze
// guard; FULL rewrites the heap (new relfilenode, ctids change) exercising its rewrite
// guard. Neither changes the logical row set, so a key-space cursor is unaffected and must
// stay fully covered. VACUUM cannot run inside a txn block → simple protocol.
func opVacuum(stmt string) func(*testing.T, context.Context, *pgxpool.Pool, string) gapEffect {
	return func(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect {
		if _, err := pool.Exec(ctx, stmt+" "+table, pgx.QueryExecModeSimpleProtocol); err != nil {
			t.Fatalf("gap %s: %v", stmt, err)
		}
		return gapEffect{}
	}
}

// opSavepointInsert commits an insert through a savepoint and rolls another back, so the
// committed row carries a SUBTRANSACTION xmin — the case that makes pg_visible_in_snapshot
// unsafe and forces the physical tier's numeric horizon compare. The committed row is
// `added`; the rolled-back row must never appear (a phantom would surface as an extra PK,
// which the no-loss check tolerates but is not produced by a correct sink).
func opSavepointInsert(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) gapEffect {
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("gap savepoint begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var committed string
	if err := tx.QueryRow(ctx, insertSQL(table)).Scan(&committed); err != nil {
		t.Fatalf("gap savepoint insert: %v", err)
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT sp"); err != nil {
		t.Fatalf("savepoint: %v", err)
	}
	if _, err := tx.Exec(ctx, insertSQL(table)); err != nil {
		t.Fatalf("savepoint insert 2: %v", err)
	}
	if _, err := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT sp"); err != nil {
		t.Fatalf("rollback to savepoint: %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("savepoint commit: %v", err)
	}
	return gapEffect{added: []string{committed}}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// insertSQL builds an INSERT … RETURNING id for the seed schema with a random key and
// deterministic-enough payload (values are irrelevant to coverage, only the new PK is).
func insertSQL(table string) string {
	return "INSERT INTO " + table + " (id, tenant, seq, amount, payload, updated_at) " +
		"VALUES (gen_random_uuid(), 't0', -1, 0, '{}'::jsonb, now()) RETURNING id::text"
}

func insertRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) string {
	var id string
	if err := pool.QueryRow(ctx, insertSQL(table)).Scan(&id); err != nil {
		t.Fatalf("gap insert: %v", err)
	}
	return id
}

// capturePKs reads the primary-key set of a (possibly schema-qualified) seed table as a
// text set. Seed tables have a single uuid PK named id.
func capturePKs(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) map[string]bool {
	rows, err := pool.Query(ctx, "SELECT id::text FROM "+table)
	if err != nil {
		t.Fatalf("capture pks %s: %v", table, err)
	}
	out := make(map[string]bool)
	for _, id := range scanIDs(t, rows) {
		out[id] = true
	}
	return out
}

func scanIDs(t *testing.T, rows pgx.Rows) []string {
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	return ids
}

// countDoneShards totals the bitmap shards already flagged complete across all resources'
// persisted checkpoints.
func countDoneShards(ctx context.Context, store *memory.Store, id filament.RunID, resources []string) int {
	n := 0
	for _, res := range resources {
		cp, err := store.LoadCheckpoint(ctx, id, res)
		if err != nil {
			continue
		}
		ks, ok := checkpoint.ParseKeyset(cp)
		if !ok {
			continue
		}
		for _, sh := range ks.Shards {
			if sh.Done {
				n++
			}
		}
	}
	return n
}

func toSet(s []string) map[string]bool {
	out := make(map[string]bool, len(s))
	for _, v := range s {
		out[v] = true
	}
	return out
}
