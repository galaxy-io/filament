package postgres_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres"
)

// testDSN returns the DSN from FILAMENT_TEST_POSTGRES_DSN, or skips the test.
func testDSN(t *testing.T) string {
	t.Helper()
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FILAMENT_TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}
	return dsn
}

func newTestStore(t *testing.T) *postgres.Store {
	t.Helper()
	dsn := testDSN(t)

	sqlDB, err := postgres.NewSQLDB(dsn)
	if err != nil {
		t.Fatalf("NewSQLDB: %v", err)
	}
	defer sqlDB.Close()
	if err := postgres.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)

	// wipe between tests so each test starts from a clean slate against the
	// same long-lived container/schema.
	for _, table := range []string{"dedup_seen", "checkpoints", "resource_states", "runs", "schedules", "pipelines", "secrets"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}

	return postgres.New(pool)
}

func TestStore_RunLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	run := ingestion.RunState{
		Run:    "run-1",
		Tenant: "tenant-a",
		Status: ingestion.RunRequested,
		Request: ingestion.RunRequest{
			Tenant:        "tenant-a",
			Source:        ingestion.Ref{Provider: "postgres", Config: map[string]any{"dsn": "ref:pg-dsn"}},
			Sink:          ingestion.Ref{Provider: "stdout"},
			IngestionType: ingestion.IngestionSnapshotReplace,
		},
		Resources: []ingestion.ResourceState{
			{Resource: "orders", Enabled: true, Status: ingestion.RunRunning, Records: 10},
		},
		StartedAt: time.Now().Truncate(time.Microsecond),
	}

	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	got, err := store.LoadRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if got.Tenant != "tenant-a" || got.Request.Source.Provider != "postgres" {
		t.Fatalf("unexpected run: %+v", got)
	}
	if len(got.Resources) != 1 || got.Resources[0].Resource != "orders" || got.Resources[0].Records != 10 {
		t.Fatalf("unexpected resources: %+v", got.Resources)
	}

	if _, err := store.LoadRun(ctx, "does-not-exist"); err == nil {
		t.Fatal("expected ErrNotFound for missing run")
	}

	runs, err := store.ListRuns(ctx, ingestion.RunFilter{Tenant: "tenant-a"})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	cp := ingestion.NewCheckpoint("orders").Set("page", 3)
	if err := store.SaveCheckpoint(ctx, "run-1", cp); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	loaded, err := store.LoadCheckpoint(ctx, "run-1", "orders")
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if loaded.Int("page") != 3 {
		t.Fatalf("expected page=3, got %d", loaded.Int("page"))
	}

	seenBefore, err := store.DedupSeen(ctx, "tenant-a", "run-1", 42)
	if err != nil {
		t.Fatalf("DedupSeen: %v", err)
	}
	if seenBefore {
		t.Fatal("expected first DedupSeen call to report unseen")
	}
	seenAfter, err := store.DedupSeen(ctx, "tenant-a", "run-1", 42)
	if err != nil {
		t.Fatalf("DedupSeen: %v", err)
	}
	if !seenAfter {
		t.Fatal("expected second DedupSeen call to report already seen")
	}
}

// TestStore_DedupSeenHighWaterMark verifies the high-water-mark semantics:
// dedup_seen holds one row per (tenant, run), not one per fact, so it must
// treat any seq at or below the highest one already applied as "seen" — not
// just an exact repeat of the same seq — and correctly advance past it for a
// genuinely new, higher seq.
func TestStore_DedupSeenHighWaterMark(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	seen, err := store.DedupSeen(ctx, "tenant-a", "run-hwm", 10)
	if err != nil {
		t.Fatalf("DedupSeen(10): %v", err)
	}
	if seen {
		t.Fatal("expected seq 10 to be unseen (first fact for this run)")
	}

	seen, err = store.DedupSeen(ctx, "tenant-a", "run-hwm", 20)
	if err != nil {
		t.Fatalf("DedupSeen(20): %v", err)
	}
	if seen {
		t.Fatal("expected seq 20 to be unseen (advances the mark past 10)")
	}

	// A redelivered lower seq (not just an exact repeat) must be treated as
	// already applied — this is the point of the high-water mark.
	seen, err = store.DedupSeen(ctx, "tenant-a", "run-hwm", 15)
	if err != nil {
		t.Fatalf("DedupSeen(15): %v", err)
	}
	if !seen {
		t.Fatal("expected seq 15 to be reported already-seen (below the mark of 20)")
	}

	seen, err = store.DedupSeen(ctx, "tenant-a", "run-hwm", 25)
	if err != nil {
		t.Fatalf("DedupSeen(25): %v", err)
	}
	if seen {
		t.Fatal("expected seq 25 to be unseen (advances the mark past 20)")
	}
}

func TestStore_ScheduleClaimDue(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	past := time.Now().Add(-time.Minute)
	sched := ingestion.ScheduleState{
		ID: "sched-1",
		Spec: ingestion.ScheduleSpec{
			Tenant: "tenant-a",
			Cron:   "* * * * *",
			Request: ingestion.RunRequest{
				Tenant: "tenant-a",
				Source: ingestion.Ref{Provider: "postgres"},
				Sink:   ingestion.Ref{Provider: "stdout"},
			},
		},
		Enabled:   true,
		NextFire:  &past,
		CreatedAt: time.Now().Truncate(time.Microsecond),
	}
	if err := store.SaveSchedule(ctx, sched); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	// limit=0 must mean "unlimited", matching how the scheduler module calls it.
	due, err := store.ClaimDue(ctx, time.Now(), 0)
	if err != nil {
		t.Fatalf("ClaimDue: %v", err)
	}
	if len(due) != 1 || due[0].ID != "sched-1" {
		t.Fatalf("expected to claim sched-1, got %+v", due)
	}

	// Immediately re-claiming must not return the same schedule again — it's
	// under an active lease.
	due2, err := store.ClaimDue(ctx, time.Now(), 0)
	if err != nil {
		t.Fatalf("ClaimDue (second): %v", err)
	}
	if len(due2) != 0 {
		t.Fatalf("expected leased schedule to be excluded from a second claim, got %+v", due2)
	}

	if err := store.MarkFired(ctx, "sched-1", time.Now()); err != nil {
		t.Fatalf("MarkFired: %v", err)
	}

	loaded, err := store.LoadSchedule(ctx, "sched-1")
	if err != nil {
		t.Fatalf("LoadSchedule: %v", err)
	}
	if loaded.LastFired == nil {
		t.Fatal("expected LastFired to be set after MarkFired")
	}

	if err := store.DeleteSchedule(ctx, "sched-1"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if _, err := store.LoadSchedule(ctx, "sched-1"); err == nil {
		t.Fatal("expected ErrNotFound after DeleteSchedule")
	}
}

func TestSecretsStore_RoundTrip(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN(t)

	sqlDB, err := postgres.NewSQLDB(dsn)
	if err != nil {
		t.Fatalf("NewSQLDB: %v", err)
	}
	defer sqlDB.Close()
	if err := postgres.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "DELETE FROM secrets"); err != nil {
		t.Fatalf("truncate secrets: %v", err)
	}

	key := make([]byte, 32)
	secrets, err := postgres.NewSecretsStore(pool, "test-key-v1", key)
	if err != nil {
		t.Fatalf("NewSecretsStore: %v", err)
	}

	ref := "tenant-a/pg-dsn"
	original := ingestion.Secret{Value: []byte("postgres://user:pw@host/db"), Meta: map[string]string{"rotated": "2026-01-01"}}
	if err := secrets.Write(ctx, ref, original); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := secrets.Read(ctx, ref)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got.Value) != string(original.Value) {
		t.Fatalf("expected decrypted value %q, got %q", original.Value, got.Value)
	}
	if got.Meta["rotated"] != "2026-01-01" {
		t.Fatalf("expected meta round-trip, got %+v", got.Meta)
	}

	if err := secrets.Delete(ctx, ref); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := secrets.Read(ctx, ref); err == nil {
		t.Fatal("expected ErrNotFound after Delete")
	}
}

func TestPipelineStore_OptimisticLock(t *testing.T) {
	ctx := context.Background()
	dsn := testDSN(t)

	sqlDB, err := postgres.NewSQLDB(dsn)
	if err != nil {
		t.Fatalf("NewSQLDB: %v", err)
	}
	defer sqlDB.Close()
	if err := postgres.Migrate(sqlDB); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	pool, err := postgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "DELETE FROM pipelines"); err != nil {
		t.Fatalf("truncate pipelines: %v", err)
	}

	pipelines := postgres.NewPipelineStore(pool)
	created, err := pipelines.Create(ctx, "pipe-1", "tenant-a", "orders-sync", nil, nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Version != 1 {
		t.Fatalf("expected version 1, got %d", created.Version)
	}

	created.Name = "orders-sync-v2"
	updated, err := pipelines.Update(ctx, created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("expected version 2, got %d", updated.Version)
	}

	// Reusing the stale (version=1) copy must be rejected, not silently applied.
	created.Name = "stale-write"
	if _, err := pipelines.Update(ctx, created); !errors.Is(err, postgres.ErrVersionConflict) {
		t.Fatalf("expected ErrVersionConflict for stale update, got %v", err)
	}

	fetched, err := pipelines.Get(ctx, "pipe-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Name != "orders-sync-v2" {
		t.Fatalf("stale update must not have applied, got name %q", fetched.Name)
	}
}
