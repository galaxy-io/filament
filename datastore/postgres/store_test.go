package postgres_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
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
	for _, table := range []string{"dedup_seen", "checkpoints", "resource_states", "runs", "schedules", "pipelines", "connections", "secrets"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}

	return postgres.New(pool)
}

func TestStore_RunLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	run := filament.RunState{
		Run:    "run-1",
		Tenant: "tenant-a",
		Status: filament.RunRequested,
		Request: filament.RunRequest{
			Tenant:        "tenant-a",
			Source:        filament.Ref{Provider: "postgres", Config: map[string]any{"dsn": "ref:pg-dsn"}},
			Sink:          filament.Ref{Provider: "stdout"},
			IngestionType: filament.IngestionSnapshotReplace,
		},
		Resources: []filament.ResourceState{
			{Resource: "orders", Enabled: true, Status: filament.RunRunning, Records: 10},
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

	runs, err := store.ListRuns(ctx, filament.RunFilter{Tenant: "tenant-a"})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	cp := filament.NewCheckpoint("orders").Set("page", 3)
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
	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{
		Id: "schedule-pipeline", TenantId: "tenant-a", Name: "scheduled",
	}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}

	past := time.Now().Add(-time.Minute)
	sched := filament.ScheduleState{
		ID: "sched-1",
		Spec: filament.ScheduleSpec{
			Tenant:     "tenant-a",
			PipelineID: "schedule-pipeline",
			Cron:       "* * * * *",
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

	if err := store.ReleaseScheduleClaim(ctx, "sched-1"); err != nil {
		t.Fatalf("ReleaseScheduleClaim: %v", err)
	}
	due3, err := store.ClaimDue(ctx, time.Now(), 0)
	if err != nil {
		t.Fatalf("ClaimDue (after release): %v", err)
	}
	if len(due3) != 1 {
		t.Fatalf("expected released schedule to be claimable, got %+v", due3)
	}

	if err := store.DeleteSchedule(ctx, "sched-1"); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if _, err := store.LoadSchedule(ctx, "sched-1"); err == nil {
		t.Fatal("expected ErrNotFound after DeleteSchedule")
	}
}

// TestStore_ConnectionSoftDelete verifies a deleted connection disappears from
// reads and frees its (tenant, kind, name) for a new connection.
func TestStore_ConnectionSoftDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.EnsureTenant(ctx, "tenant-a", "Tenant A"); err != nil {
		t.Fatalf("EnsureTenant: %v", err)
	}

	conn := filament.Connection{
		ID: "conn-1", Tenant: "tenant-a", Kind: filament.ConnectorKindSource,
		Name: "pg-main", Connector: "postgres",
	}
	if _, err := store.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	if err := store.DeleteConnection(ctx, "conn-1"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	if _, err := store.LoadConnection(ctx, "conn-1"); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: "tenant-a"})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected deleted connection excluded from list, got %+v", listed)
	}

	withDeleted, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: "tenant-a", IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListConnections with IncludeDeleted: %v", err)
	}
	if len(withDeleted) != 1 {
		t.Fatalf("expected deleted connection included, got %+v", withDeleted)
	}

	// The partial unique index only covers live rows, so the name is reusable.
	conn.ID = "conn-2"
	if _, err := store.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("CreateConnection with reused name: %v", err)
	}
}

// TestStore_PipelineSoftDelete verifies a deleted pipeline disappears from
// lists, stays loadable by id, stops accepting versions, drops its schedule,
// and keeps version history for run views.
func TestStore_PipelineSoftDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.EnsureTenant(ctx, "tenant-a", "Tenant A"); err != nil {
		t.Fatalf("EnsureTenant: %v", err)
	}

	created, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-del", TenantId: "tenant-a", Name: "doomed"})
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if created.GetCreatedAt() == 0 {
		t.Fatalf("expected created_at on the create response, got %+v", created)
	}
	if _, err := store.CreatePipelineVersion(ctx, "pipe-del", &ingestionv1.PipelineVersion{}); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if err := store.SaveSchedule(ctx, filament.ScheduleState{
		ID:        "sched-del",
		Spec:      filament.ScheduleSpec{Tenant: "tenant-a", PipelineID: "pipe-del", Cron: "* * * * *"},
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Microsecond),
	}); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	if err := store.DeletePipeline(ctx, "pipe-del"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}
	loaded, err := store.LoadPipeline(ctx, "pipe-del")
	if err != nil {
		t.Fatalf("expected deleted pipeline to stay loadable, got %v", err)
	}
	if loaded.GetDeletedAt() == 0 {
		t.Fatalf("expected deleted_at set on the loaded pipeline, got %+v", loaded)
	}
	stamp, ok := strings.CutPrefix(loaded.GetName(), "doomed__deleted__")
	if !ok {
		t.Fatalf("expected delete stamp on the name, got %q", loaded.GetName())
	}
	stampedAt, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatalf("delete stamp %q is not RFC3339: %v", stamp, err)
	}
	if stampedAt.UnixMilli() != loaded.GetDeletedAt() {
		t.Fatalf("delete stamp %d disagrees with deleted_at %d", stampedAt.UnixMilli(), loaded.GetDeletedAt())
	}
	pipelines, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: "tenant-a"})
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(pipelines) != 0 {
		t.Fatalf("expected deleted pipeline excluded from list, got %+v", pipelines)
	}

	withDeleted, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: "tenant-a", IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListPipelines with IncludeDeleted: %v", err)
	}
	if len(withDeleted) != 1 {
		t.Fatalf("expected deleted pipeline included, got %+v", withDeleted)
	}
	if withDeleted[0].GetCreatedAt() == 0 || withDeleted[0].GetDeletedAt() == 0 {
		t.Fatalf("expected created_at and deleted_at set, got %+v", withDeleted[0])
	}
	if _, err := store.CreatePipelineVersion(ctx, "pipe-del", &ingestionv1.PipelineVersion{}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound creating version on deleted pipeline, got %v", err)
	}
	if _, err := store.LoadPipelineSchedule(ctx, "pipe-del"); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected schedule removed with pipeline, got %v", err)
	}

	// History survives for run views.
	versions, err := store.ListPipelineVersions(ctx, "pipe-del")
	if err != nil {
		t.Fatalf("ListPipelineVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected version history kept, got %d versions", len(versions))
	}
}

func TestStore_PipelineOptimisticLock(t *testing.T) {
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

	store := postgres.New(pool)
	created, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: "pipe-1", TenantId: "tenant-a", Name: "orders-sync"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	version, err := store.CreatePipelineVersion(ctx, created.Id, &ingestionv1.PipelineVersion{})
	if err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if version.Version != 1 {
		t.Fatalf("expected version 1, got %d", version.Version)
	}

	second, err := store.CreatePipelineVersion(ctx, created.Id, &ingestionv1.PipelineVersion{})
	if err != nil {
		t.Fatalf("CreatePipelineVersion second: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}

	versions, err := store.ListPipelineVersions(ctx, created.Id)
	if err != nil {
		t.Fatalf("ListPipelineVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	if versions[0].Version != 2 || versions[1].Version != 1 {
		t.Fatalf("expected versions newest first, got %d then %d", versions[0].Version, versions[1].Version)
	}
	if versions[0].CreatedAt == 0 {
		t.Fatalf("expected nonzero CreatedAt")
	}
	unknown, err := store.ListPipelineVersions(ctx, "no-such-pipeline")
	if err != nil {
		t.Fatalf("ListPipelineVersions unknown: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("expected empty versions for unknown pipeline, got %d", len(unknown))
	}

	created.Name = "orders-sync-v2"
	updated, err := store.UpdatePipeline(ctx, created)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "orders-sync-v2" {
		t.Fatalf("expected updated name, got %q", updated.Name)
	}

	fetched, err := store.LoadPipeline(ctx, "pipe-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Name != "orders-sync-v2" {
		t.Fatalf("got name %q", fetched.Name)
	}
	if fetched.CurrentVersionId != 2 {
		t.Fatalf("expected current version 2, got %d", fetched.CurrentVersionId)
	}
}
