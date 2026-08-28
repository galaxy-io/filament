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

const (
	tenantA          = "11111111-1111-4111-8111-111111111111"
	runOne           = "20000000-0000-4000-8000-000000000001"
	runHighWater     = "20000000-0000-4000-8000-000000000002"
	missingRun       = "20000000-0000-4000-8000-000000000099"
	runReaped        = "20000000-0000-4000-8000-000000000003"
	scheduleOne      = "30000000-0000-4000-8000-000000000001"
	scheduleDeleted  = "30000000-0000-4000-8000-000000000002"
	scheduleReaped   = "30000000-0000-4000-8000-000000000003"
	pipelineSchedule = "40000000-0000-4000-8000-000000000001"
	pipelineDeleted  = "40000000-0000-4000-8000-000000000002"
	pipelineOne      = "40000000-0000-4000-8000-000000000003"
	pipelineReaped   = "40000000-0000-4000-8000-000000000004"
	missingPipeline  = "40000000-0000-4000-8000-000000000099"
	connectionOne    = "50000000-0000-4000-8000-000000000001"
	connectionTwo    = "50000000-0000-4000-8000-000000000002"
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
	for _, table := range []string{"secrets", "run_dedup_seen", "pipeline_resource_checkpoints", "run_resource_checkpoints", "run_resource_states", "runs", "schedules", "pipelines", "connections", "users", "tenants"} {
		if _, err := pool.Exec(ctx, "DELETE FROM "+table); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}

	store := postgres.New(pool)
	if err := store.EnsureTenant(ctx, tenantA, "Tenant A"); err != nil {
		t.Fatalf("EnsureTenant: %v", err)
	}
	return store
}

func TestStore_RunLifecycle(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	run := filament.RunState{
		Run:    runOne,
		Tenant: tenantA,
		Status: filament.RunRequested,
		Request: filament.RunRequest{
			Tenant:         tenantA,
			Source:         filament.Ref{Connector: "postgres", Config: map[string]any{"dsn": "ref:pg-dsn"}},
			Sink:           filament.Ref{Connector: "stdout"},
			IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullReplace},
		},
		Resources: []filament.ResourceState{
			{Resource: "orders", Enabled: true, Status: filament.RunRunning, Records: 10},
		},
		StartedAt: time.Now().Truncate(time.Microsecond),
	}

	if err := store.SaveRun(ctx, run); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	got, err := store.LoadRun(ctx, runOne)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if got.Tenant != tenantA || got.Request.Source.Connector != "postgres" {
		t.Fatalf("unexpected run: %+v", got)
	}
	if len(got.Resources) != 1 || got.Resources[0].Resource != "orders" || got.Resources[0].Records != 10 {
		t.Fatalf("unexpected resources: %+v", got.Resources)
	}

	if _, err := store.LoadRun(ctx, missingRun); err == nil {
		t.Fatal("expected ErrNotFound for missing run")
	}

	runs, _, err := store.ListRuns(ctx, filament.RunFilter{Tenant: tenantA})
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}

	cp := filament.NewCheckpoint("orders").Set("page", 3)
	if err := store.SaveCheckpoint(ctx, runOne, cp); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}
	loaded, err := store.LoadCheckpoint(ctx, runOne, "orders")
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if loaded.Int("page") != 3 {
		t.Fatalf("expected page=3, got %d", loaded.Int("page"))
	}

	seenBefore, err := store.DedupSeen(ctx, tenantA, runOne, 42)
	if err != nil {
		t.Fatalf("DedupSeen: %v", err)
	}
	if seenBefore {
		t.Fatal("expected first DedupSeen call to report unseen")
	}
	seenAfter, err := store.DedupSeen(ctx, tenantA, runOne, 42)
	if err != nil {
		t.Fatalf("DedupSeen: %v", err)
	}
	if !seenAfter {
		t.Fatal("expected second DedupSeen call to report already seen")
	}
}

// TestStore_DedupSeenHighWaterMark verifies the high-water-mark semantics:
// run_dedup_seen holds one row per (tenant, run), not one per fact, so it must
// treat any seq at or below the highest one already applied as "seen" — not
// just an exact repeat of the same seq — and correctly advance past it for a
// genuinely new, higher seq.
func TestStore_DedupSeenHighWaterMark(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.SaveRun(ctx, filament.RunState{Run: runHighWater, Tenant: tenantA, Request: filament.RunRequest{Tenant: tenantA}}); err != nil {
		t.Fatalf("SaveRun: %v", err)
	}

	seen, err := store.DedupSeen(ctx, tenantA, runHighWater, 10)
	if err != nil {
		t.Fatalf("DedupSeen(10): %v", err)
	}
	if seen {
		t.Fatal("expected seq 10 to be unseen (first fact for this run)")
	}

	seen, err = store.DedupSeen(ctx, tenantA, runHighWater, 20)
	if err != nil {
		t.Fatalf("DedupSeen(20): %v", err)
	}
	if seen {
		t.Fatal("expected seq 20 to be unseen (advances the mark past 10)")
	}

	// A redelivered lower seq (not just an exact repeat) must be treated as
	// already applied — this is the point of the high-water mark.
	seen, err = store.DedupSeen(ctx, tenantA, runHighWater, 15)
	if err != nil {
		t.Fatalf("DedupSeen(15): %v", err)
	}
	if !seen {
		t.Fatal("expected seq 15 to be reported already-seen (below the mark of 20)")
	}

	seen, err = store.DedupSeen(ctx, tenantA, runHighWater, 25)
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
		Id: pipelineSchedule, TenantId: tenantA, Name: "scheduled",
	}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}

	past := time.Now().Add(-time.Minute)
	sched := filament.ScheduleState{
		ID: scheduleOne,
		Spec: filament.ScheduleSpec{
			Tenant:     tenantA,
			PipelineID: pipelineSchedule,
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
	if len(due) != 1 || due[0].ID != scheduleOne {
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

	if err := store.ReleaseScheduleClaim(ctx, scheduleOne); err != nil {
		t.Fatalf("ReleaseScheduleClaim: %v", err)
	}
	due3, err := store.ClaimDue(ctx, time.Now(), 0)
	if err != nil {
		t.Fatalf("ClaimDue (after release): %v", err)
	}
	if len(due3) != 1 {
		t.Fatalf("expected released schedule to be claimable, got %+v", due3)
	}

	if err := store.DeleteSchedule(ctx, scheduleOne); err != nil {
		t.Fatalf("DeleteSchedule: %v", err)
	}
	if _, err := store.LoadSchedule(ctx, scheduleOne); err == nil {
		t.Fatal("expected ErrNotFound after DeleteSchedule")
	}
}

// TestStore_ConnectionSoftDelete verifies a deleted connection disappears from
// lists, stays loadable by id with its delete stamp, and frees its
// (tenant, kind, name) for a new connection.
func TestStore_ConnectionSoftDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.EnsureTenant(ctx, tenantA, "Tenant A"); err != nil {
		t.Fatalf("EnsureTenant: %v", err)
	}

	conn := filament.Connection{
		ID: connectionOne, Tenant: tenantA, Kind: filament.ConnectorKindSource,
		Name: "pg-main", Connector: "postgres",
	}
	if _, err := store.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	if err := store.DeleteConnection(ctx, connectionOne); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	loaded, err := store.LoadConnection(ctx, connectionOne)
	if err != nil {
		t.Fatalf("expected deleted connection to stay loadable, got %v", err)
	}
	if loaded.DeletedAt == 0 {
		t.Fatalf("expected deleted_at set on the loaded connection, got %+v", loaded)
	}
	stamp, ok := strings.CutPrefix(loaded.Name, "pg-main__deleted__")
	if !ok {
		t.Fatalf("expected delete stamp on the name, got %q", loaded.Name)
	}
	stampedAt, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		t.Fatalf("delete stamp %q is not RFC3339: %v", stamp, err)
	}
	if stampedAt.UnixMilli() != loaded.DeletedAt {
		t.Fatalf("delete stamp %d disagrees with deleted_at %d", stampedAt.UnixMilli(), loaded.DeletedAt)
	}
	listed, _, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: tenantA})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected deleted connection excluded from list, got %+v", listed)
	}

	withDeleted, _, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: tenantA, IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListConnections with IncludeDeleted: %v", err)
	}
	if len(withDeleted) != 1 {
		t.Fatalf("expected deleted connection included, got %+v", withDeleted)
	}

	// The partial unique index only covers live rows, so the name is reusable.
	conn.ID = connectionTwo
	if _, err := store.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("CreateConnection with reused name: %v", err)
	}
}

func TestStore_ListConnectionsSearchSortAndPage(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	for _, connection := range []filament.Connection{
		{ID: connectionOne, Tenant: tenantA, Kind: filament.ConnectorKindSource, Name: "Alpha Warehouse", Connector: "postgres"},
		{ID: connectionTwo, Tenant: tenantA, Kind: filament.ConnectorKindSource, Name: "Zulu Warehouse", Connector: "postgres"},
	} {
		if _, err := store.CreateConnection(ctx, connection); err != nil {
			t.Fatal(err)
		}
	}

	connections, total, err := store.ListConnections(ctx, filament.ConnectionFilter{
		Tenant: tenantA, Kind: filament.ConnectorKindSource,
		ListOptions: filament.ListOptions{Search: "ware", SortBy: "name", SortDescending: true, Limit: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(connections) != 1 || connections[0].Name != "Zulu Warehouse" {
		t.Fatalf("connections = %+v, total = %d", connections, total)
	}
}

// TestStore_PipelineSoftDelete verifies a deleted pipeline disappears from
// lists, stays loadable by id, stops accepting versions, drops its schedule,
// and keeps version history for run views.
func TestStore_PipelineSoftDelete(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	if err := store.EnsureTenant(ctx, tenantA, "Tenant A"); err != nil {
		t.Fatalf("EnsureTenant: %v", err)
	}

	created, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineDeleted, TenantId: tenantA, Name: "doomed"})
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if created.GetCreatedAt() == 0 {
		t.Fatalf("expected created_at on the create response, got %+v", created)
	}
	if _, err := store.CreatePipelineVersion(ctx, pipelineDeleted, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}}); err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if err := store.SaveSchedule(ctx, filament.ScheduleState{
		ID:        scheduleDeleted,
		Spec:      filament.ScheduleSpec{Tenant: tenantA, PipelineID: pipelineDeleted, Cron: "* * * * *"},
		Enabled:   true,
		CreatedAt: time.Now().Truncate(time.Microsecond),
	}); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}

	if err := store.DeletePipeline(ctx, pipelineDeleted); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}
	loaded, err := store.LoadPipeline(ctx, pipelineDeleted)
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
	pipelines, _, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: tenantA})
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(pipelines) != 0 {
		t.Fatalf("expected deleted pipeline excluded from list, got %+v", pipelines)
	}

	withDeleted, _, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: tenantA, IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListPipelines with IncludeDeleted: %v", err)
	}
	if len(withDeleted) != 1 {
		t.Fatalf("expected deleted pipeline included, got %+v", withDeleted)
	}
	if withDeleted[0].GetCreatedAt() == 0 || withDeleted[0].GetDeletedAt() == 0 {
		t.Fatalf("expected created_at and deleted_at set, got %+v", withDeleted[0])
	}
	if _, err := store.CreatePipelineVersion(ctx, pipelineDeleted, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound creating version on deleted pipeline, got %v", err)
	}
	if _, err := store.LoadPipelineSchedule(ctx, pipelineDeleted); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected schedule removed with pipeline, got %v", err)
	}

	// History survives for run views.
	versions, _, err := store.ListPipelineVersions(ctx, filament.PipelineVersionFilter{PipelineID: pipelineDeleted})
	if err != nil {
		t.Fatalf("ListPipelineVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected version history kept, got %d versions", len(versions))
	}
}

// TestStore_DeletePipelineReapsScheduledRuns pins the delete cascade against
// the runs.schedule_id ON DELETE SET NULL foreign key: removing the schedules
// row nulls the pointer before any post-delete reap could use it, so the
// delete transaction itself must remove pending RunScheduled rows, and a
// stale SaveSchedule afterward must not resurrect the schedule.
func TestStore_DeletePipelineReapsScheduledRuns(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	if _, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineReaped, TenantId: tenantA, Name: "reaped"}); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	version, err := store.CreatePipelineVersion(ctx, pipelineReaped, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	fire := time.Now().Add(time.Hour).Truncate(time.Microsecond)
	schedule := filament.ScheduleState{
		ID:        scheduleReaped,
		Spec:      filament.ScheduleSpec{Tenant: tenantA, PipelineID: pipelineReaped, Cron: "0 * * * *", Timezone: "UTC", Enabled: true},
		Enabled:   true,
		NextFire:  &fire,
		CreatedAt: time.Now().Truncate(time.Microsecond),
	}
	if err := store.SaveSchedule(ctx, schedule); err != nil {
		t.Fatalf("SaveSchedule: %v", err)
	}
	if err := store.CreateRun(ctx, filament.RunState{
		Run:    runReaped,
		Tenant: tenantA,
		Status: filament.RunScheduled,
		Request: filament.RunRequest{
			Tenant:            tenantA,
			PipelineID:        pipelineReaped,
			PipelineVersionID: version.GetId(),
			ScheduleID:        scheduleReaped,
		},
		ScheduleID:  scheduleReaped,
		ScheduledAt: fire,
	}); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	pending, _, err := store.ListRuns(ctx, filament.RunFilter{PipelineID: pipelineReaped, Status: []filament.RunStatus{filament.RunScheduled}})
	if err != nil {
		t.Fatalf("ListRuns before delete: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending scheduled run before delete, got %d", len(pending))
	}

	if err := store.DeletePipeline(ctx, pipelineReaped); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	pending, _, err = store.ListRuns(ctx, filament.RunFilter{PipelineID: pipelineReaped, Status: []filament.RunStatus{filament.RunScheduled}})
	if err != nil {
		t.Fatalf("ListRuns after delete: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected pending scheduled runs reaped on pipeline delete, got %d", len(pending))
	}
	orphans, _, err := store.ListRuns(ctx, filament.RunFilter{Tenant: tenantA, Status: []filament.RunStatus{filament.RunScheduled}})
	if err != nil {
		t.Fatalf("ListRuns for orphans: %v", err)
	}
	if len(orphans) != 0 {
		t.Fatalf("expected no scheduled runs left for the tenant, got %+v", orphans)
	}
	if _, err := store.LoadPipelineSchedule(ctx, pipelineReaped); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected schedule removed with pipeline, got %v", err)
	}

	// A scheduler tick that claimed the schedule before the delete saves its
	// detached state afterward — that save must not re-insert the row.
	if err := store.SaveSchedule(ctx, schedule); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound saving a schedule for a deleted pipeline, got %v", err)
	}
	if _, err := store.LoadPipelineSchedule(ctx, pipelineReaped); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected schedule to stay deleted after stale save, got %v", err)
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
	if err := store.EnsureTenant(ctx, tenantA, "Tenant A"); err != nil {
		t.Fatalf("EnsureTenant: %v", err)
	}
	created, err := store.CreatePipeline(ctx, &ingestionv1.Pipeline{Id: pipelineOne, TenantId: tenantA, Name: "orders-sync"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	version, err := store.CreatePipelineVersion(ctx, created.Id, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatalf("CreatePipelineVersion: %v", err)
	}
	if version.Version != 1 {
		t.Fatalf("expected version 1, got %d", version.Version)
	}

	second, err := store.CreatePipelineVersion(ctx, created.Id, &ingestionv1.PipelineVersion{Graph: &ingestionv1.PipelineGraph{}})
	if err != nil {
		t.Fatalf("CreatePipelineVersion second: %v", err)
	}
	if second.Version != 2 {
		t.Fatalf("expected version 2, got %d", second.Version)
	}

	versions, _, err := store.ListPipelineVersions(ctx, filament.PipelineVersionFilter{PipelineID: created.Id})
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
	unknown, _, err := store.ListPipelineVersions(ctx, filament.PipelineVersionFilter{PipelineID: missingPipeline})
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

	fetched, err := store.LoadPipeline(ctx, pipelineOne)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if fetched.Name != "orders-sync-v2" {
		t.Fatalf("got name %q", fetched.Name)
	}
	if fetched.GetCurrentVersion().GetId() != second.Id {
		t.Fatalf("expected current version %q, got %q", second.Id, fetched.GetCurrentVersion().GetId())
	}
}

func TestStore_EnsureUserIdempotent(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	first, err := store.EnsureUser(ctx, tenantA, "zitadel-user-1")
	if err != nil {
		t.Fatalf("EnsureUser: %v", err)
	}
	if first == "" {
		t.Fatal("EnsureUser returned an empty id")
	}
	again, err := store.EnsureUser(ctx, tenantA, "zitadel-user-1")
	if err != nil {
		t.Fatalf("EnsureUser again: %v", err)
	}
	if again != first {
		t.Fatalf("EnsureUser minted a second id %q for the same subject, want %q", again, first)
	}
	other, err := store.EnsureUser(ctx, tenantA, "zitadel-user-2")
	if err != nil {
		t.Fatalf("EnsureUser other: %v", err)
	}
	if other == first {
		t.Fatalf("EnsureUser reused id %q across distinct subjects", first)
	}
	if _, err := store.EnsureUser(ctx, tenantA, ""); err == nil {
		t.Fatal("EnsureUser accepted an empty external id")
	}
}

func TestStore_ResolveTenantIdempotent(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	first, err := store.ResolveTenant(ctx, "zitadel-org-1", "Org One")
	if err != nil {
		t.Fatalf("ResolveTenant: %v", err)
	}
	if first == "" {
		t.Fatal("ResolveTenant returned an empty id")
	}
	again, err := store.ResolveTenant(ctx, "zitadel-org-1", "")
	if err != nil {
		t.Fatalf("ResolveTenant again: %v", err)
	}
	if again != first {
		t.Fatalf("ResolveTenant minted a second id %q for the same org, want %q", again, first)
	}
	loaded, err := store.LoadTenant(ctx, first)
	if err != nil {
		t.Fatalf("LoadTenant: %v", err)
	}
	if loaded.Name != "Org One" {
		t.Fatalf("empty name overwrote %q with %q", "Org One", loaded.Name)
	}
	other, err := store.ResolveTenant(ctx, "zitadel-org-2", "Org Two")
	if err != nil {
		t.Fatalf("ResolveTenant other: %v", err)
	}
	if other == first {
		t.Fatalf("ResolveTenant reused id %q across distinct orgs", first)
	}
	if _, err := store.ResolveTenant(ctx, "", "Nameless"); err == nil {
		t.Fatal("ResolveTenant accepted an empty external id")
	}
}
