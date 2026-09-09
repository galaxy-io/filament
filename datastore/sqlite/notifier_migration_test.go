package sqlite

import (
	"context"
	"database/sql"
	"io/fs"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestNotifierMigration(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		name := "fresh"
		if upgrade {
			name = "upgrade"
		}
		t.Run(name, func(t *testing.T) {
			db, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			t.Cleanup(func() { _ = db.Close() })
			dir, err := fs.Sub(migrations, "migrations")
			if err != nil {
				t.Fatal(err)
			}
			provider, err := goose.NewProvider(goose.DialectSQLite3, db, dir)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if upgrade {
				if _, err := provider.UpTo(ctx, 1); err != nil {
					t.Fatal(err)
				}
				seedNotifierParents(t, db)
			}
			if err := Migrate(db); err != nil {
				t.Fatal(err)
			}
			if !upgrade {
				seedNotifierParents(t, db)
			}
			checkNotifierSchema(t, db)
			// Re-running migrations must preserve both rules and their parent.
			if err := Migrate(db); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.QueryRow("SELECT count(*) FROM notifier WHERE pipeline_id = 'pipeline'").Scan(&count); err != nil || count != 2 {
				t.Fatalf("notifier count = %d, %v", count, err)
			}
		})
	}
}

func seedNotifierParents(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, query := range []string{
		"INSERT INTO tenants (id, name, created_at, updated_at) VALUES ('tenant', 'Tenant', 1, 1), ('other', 'Other', 1, 1)",
		"INSERT INTO pipelines (id, tenant_id, name, created_at, updated_at) VALUES ('pipeline', 'tenant', 'Existing pipeline', 1, 1)",
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
}

func checkNotifierSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	const insert = `INSERT INTO notifier (id, tenant_id, pipeline_id, name, notification_type, events, created_at, updated_at)
		VALUES (?, ?, ?, 'Notify', ?, '["run.completed"]', 1, 1)`
	for _, id := range []string{"first", "second"} {
		if _, err := db.Exec(insert, id, "tenant", "pipeline", "webhook"); err != nil {
			t.Fatalf("insert multiple pipeline notifiers: %v", err)
		}
	}
	for _, tc := range []struct{ id, tenant, pipeline, kind string }{
		{"cross-tenant", "other", "pipeline", "webhook"},
		{"missing-parent", "tenant", "missing", "webhook"},
		{"unknown-kind", "tenant", "pipeline", "email"},
		{"unspecified", "tenant", "pipeline", ""},
	} {
		if _, err := db.Exec(insert, tc.id, tc.tenant, tc.pipeline, tc.kind); err == nil {
			t.Errorf("accepted invalid notifier %s", tc.id)
		}
	}
	if _, err := db.Exec(insert, "null-kind", "tenant", "pipeline", nil); err == nil {
		t.Error("accepted null notification type")
	}
	var enabled, deleted bool
	var version int64
	var deletedAt sql.NullInt64
	var resources, config, refs string
	if err := db.QueryRow(`SELECT enabled, is_deleted, version, deleted_at, resources, config, secret_refs
		FROM notifier WHERE id = 'first'`).Scan(&enabled, &deleted, &version, &deletedAt, &resources, &config, &refs); err != nil {
		t.Fatal(err)
	}
	if enabled || deleted || version != 1 || deletedAt.Valid || resources != "[]" || config != "{}" || refs != "{}" {
		t.Fatalf("unexpected defaults: enabled=%v deleted=%v version=%d deleted_at=%v resources=%s config=%s refs=%s",
			enabled, deleted, version, deletedAt, resources, config, refs)
	}
	var indexCount int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type = 'index' AND name = 'notifier_pipeline_idx'").Scan(&indexCount); err != nil || indexCount != 1 {
		t.Fatalf("notifier index count = %d, %v", indexCount, err)
	}
	var name string
	if err := db.QueryRow("SELECT name FROM pipelines WHERE id = 'pipeline'").Scan(&name); err != nil || name != "Existing pipeline" {
		t.Fatalf("existing pipeline = %q, %v", name, err)
	}
}
