package postgres

import (
	"context"
	"database/sql"
	"io/fs"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestNotifierMigration(t *testing.T) {
	for _, upgrade := range []bool{false, true} {
		name := "fresh"
		if upgrade {
			name = "upgrade"
		}
		t.Run(name, func(t *testing.T) {
			db := notifierMigrationDB(t)
			dir, err := fs.Sub(migrations, "migrations")
			if err != nil {
				t.Fatal(err)
			}
			provider, err := goose.NewProvider(goose.DialectPostgres, db, dir)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if upgrade {
				if _, err := provider.UpTo(ctx, 6); err != nil {
					t.Fatal(err)
				}
				seedNotifierParents(t, db)
			}
			if _, err := provider.Up(ctx); err != nil {
				t.Fatal(err)
			}
			if !upgrade {
				seedNotifierParents(t, db)
			}
			checkNotifierSchema(t, db)
			if _, err := provider.Up(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.QueryRow("SELECT count(*) FROM notifier").Scan(&count); err != nil || count != 2 {
				t.Fatalf("notifier count = %d, %v", count, err)
			}
		})
	}
}

// Each test owns an isolated schema, so upgrade testing never rolls back or
// drops the schema used by the other persistence contract tests.
func notifierMigrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FILAMENT_TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	admin := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = admin.Close() })
	schema := pgx.Identifier{"notifier_test_" + uuid.NewString()}.Sanitize()
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP SCHEMA " + schema + " CASCADE"); err != nil {
			t.Errorf("clean up migration test schema: %v", err)
		}
	})
	config.RuntimeParams["search_path"] = schema
	db := stdlib.OpenDB(*config)
	t.Cleanup(func() { _ = db.Close() })
	return db
}

const (
	notifierTenant   = "11111111-1111-4111-8111-111111111111"
	notifierOther    = "11111111-1111-4111-8111-111111111112"
	notifierPipeline = "22222222-2222-4222-8222-222222222222"
)

func seedNotifierParents(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO tenants (id, name) VALUES ($1, 'Tenant'), ($2, 'Other')", notifierTenant, notifierOther); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO pipelines (id, tenant_id, name) VALUES ($1, $2, 'Existing pipeline')", notifierPipeline, notifierTenant); err != nil {
		t.Fatal(err)
	}
}

func checkNotifierSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	const insert = `INSERT INTO notifier (tenant_id, pipeline_id, name, notification_type, events)
		VALUES ($1, $2, 'Notify', $3, '["run.completed"]')`
	for range 2 {
		if _, err := db.Exec(insert, notifierTenant, notifierPipeline, "webhook"); err != nil {
			t.Fatalf("insert multiple pipeline notifiers: %v", err)
		}
	}
	for _, tc := range []struct{ name, tenant, pipeline, kind string }{
		{"cross-tenant", notifierOther, notifierPipeline, "webhook"},
		{"missing-parent", notifierTenant, uuid.NewString(), "webhook"},
		{"unknown-kind", notifierTenant, notifierPipeline, "email"},
		{"unspecified", notifierTenant, notifierPipeline, ""},
	} {
		if _, err := db.Exec(insert, tc.tenant, tc.pipeline, tc.kind); err == nil {
			t.Errorf("accepted invalid notifier %s", tc.name)
		}
	}
	if _, err := db.Exec(insert, notifierTenant, notifierPipeline, nil); err == nil {
		t.Error("accepted null notification type")
	}
	var enabled, deleted bool
	var version int64
	var deletedAt sql.NullTime
	var resources, config, refs string
	if err := db.QueryRow(`SELECT enabled, is_deleted, version, deleted_at, resources, config, secret_refs
		FROM notifier LIMIT 1`).Scan(&enabled, &deleted, &version, &deletedAt, &resources, &config, &refs); err != nil {
		t.Fatal(err)
	}
	if enabled || deleted || version != 1 || deletedAt.Valid || resources != "[]" || config != "{}" || refs != "{}" {
		t.Fatalf("unexpected defaults: enabled=%v deleted=%v version=%d deleted_at=%v resources=%s config=%s refs=%s",
			enabled, deleted, version, deletedAt, resources, config, refs)
	}
	var indexCount int
	if err := db.QueryRow("SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'notifier_pipeline_idx'").Scan(&indexCount); err != nil || indexCount != 1 {
		t.Fatalf("notifier index count = %d, %v", indexCount, err)
	}
	var name string
	if err := db.QueryRow("SELECT name FROM pipelines WHERE id = $1", notifierPipeline).Scan(&name); err != nil || name != "Existing pipeline" {
		t.Fatalf("existing pipeline = %q, %v", name, err)
	}
}
