//go:build integration

package evolution_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	mysqlsink "github.com/galaxy-io/filament/connectors/mysql/sink"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func driftModel() rowmodel.Schema {
	return rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}, {Name: "name", Logical: rowmodel.LogicalString, Nullable: true}}, PrimaryKey: []string{"id"}}
}

// driftSink opens a sink on its own database and returns the qualified items
// table it will manage.
func driftSink(t *testing.T, db *testcontainers.MySQL, policy string, mode filament.IngestionType) (context.Context, *mysqlsink.Sink, filament.RunSpec, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	database := "drift_" + uuid.NewString()[:8]
	spec := filament.RunSpec{Run: "run", Resources: []string{"items"}, Sink: filament.Ref{Config: map[string]any{"dsn": db.RootDSN, "database": database, "schema_evolution": policy}}, WritePolicies: map[string]filament.WritePolicy{"items": {Capability: filament.WriteCapabilities(mode)[0]}}}
	s := mysqlsink.New()
	if err := s.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Commit(ctx) })
	return ctx, s, spec, "`" + database + "`.`items`"
}

// rowBatch builds a single-row batch for model from the given column writes.
func rowBatch(t *testing.T, model rowmodel.Schema, write func(filament.RowWriter)) *arrowbatch.Batch {
	t.Helper()
	col := &testutil.CollectSink{}
	t.Cleanup(col.Release)
	w, err := col.Builder("items", 0, model)
	if err != nil {
		t.Fatal(err)
	}
	write(w)
	if err := w.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	return col.BatchesFor("items")[0]
}

func TestMySQLReconcileBlocksBeforeReplace(t *testing.T) {
	db := testcontainers.MySQLContainer(t)
	for _, mode := range []string{"strict", "type", "required", "drop", "key"} {
		t.Run(mode, func(t *testing.T) {
			policy := "nullable_add"
			if mode == "strict" {
				policy = "strict"
			}
			ctx, s, _, qualified := driftSink(t, db, policy, filament.IngestionFullReplace)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			if _, err := db.RootDB.ExecContext(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
				t.Fatal(err)
			}
			model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalBool, Nullable: true})
			switch mode {
			case "type":
				model.Fields[1].Logical = rowmodel.LogicalBytes
			case "required":
				model.Fields[2].Nullable = false
			case "drop":
				model.Fields = model.Fields[:1]
			case "key":
				model.PrimaryKey = nil
			}
			observed := 0
			ctx = schema.WithObserver(ctx, func(string, schema.Detection) { observed++ })
			if err := s.EnsureSchema(ctx, "items", model); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("expected block: %v", err)
			}
			if observed != 1 {
				t.Fatalf("missing detection: %d", observed)
			}
			var count int
			if err := db.RootDB.QueryRowContext(ctx, "SELECT count(*) FROM "+qualified).Scan(&count); err != nil || count != 1 {
				t.Fatalf("replace removed rows: %d %v", count, err)
			}
			if err := db.RootDB.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.COLUMNS WHERE CONCAT('`',TABLE_SCHEMA,'`.`',TABLE_NAME,'`')=?", qualified).Scan(&count); err != nil || count != 2 {
				t.Fatalf("partial migration: %d %v", count, err)
			}
		})
	}
}

func TestMySQLNullableAddRestartAndWriteDrift(t *testing.T) {
	db := testcontainers.MySQLContainer(t)
	ctx, s, spec, qualified := driftSink(t, db, "nullable_add", filament.IngestionFullAppend)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RootDB.ExecContext(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
		t.Fatal(err)
	}
	model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalBool, Nullable: true}, rowmodel.Field{Name: "other", Logical: rowmodel.LogicalJSON, Nullable: true})
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	// DDL is durable without a run Commit, so a new sink must inspect and converge.
	retry := mysqlsink.New()
	if err := retry.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer retry.Commit(ctx)
	if err := retry.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.RootDB.QueryRowContext(ctx, "SELECT count(*) FROM "+qualified+" WHERE extra IS NULL AND other IS NULL").Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing rows changed: %d %v", count, err)
	}
	chunk := rowBatch(t, model, func(w filament.RowWriter) {
		w.Int64(2)
		w.String("new")
		w.Bool(true)
		w.String(`{"ok":true}`)
	})
	if _, err := retry.Apply(ctx, chunk, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RootDB.ExecContext(ctx, "ALTER TABLE "+qualified+" MODIFY COLUMN name LONGBLOB"); err != nil {
		t.Fatal(err)
	}
	if _, err := retry.Apply(ctx, chunk, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); !errors.Is(err, schema.ErrBlocked) {
		t.Fatalf("write drift not blocked: %v", err)
	}
	if err := db.RootDB.QueryRowContext(ctx, "SELECT count(*) FROM "+qualified).Scan(&count); err != nil || count != 2 {
		t.Fatalf("blocked write changed rows: %d %v", count, err)
	}
}

func TestMySQLUnsupportedTableFeatures(t *testing.T) {
	db := testcontainers.MySQLContainer(t)
	for _, change := range []string{"MODIFY COLUMN id BIGINT UNSIGNED NOT NULL", "ADD COLUMN derived BIGINT GENERATED ALWAYS AS (id+1)", "ADD UNIQUE KEY duplicate_key (id)", "MODIFY COLUMN name LONGTEXT COLLATE utf8mb4_0900_ai_ci"} {
		t.Run(change, func(t *testing.T) {
			ctx, s, _, qualified := driftSink(t, db, "nullable_add", filament.IngestionFullAppend)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			if _, err := db.RootDB.ExecContext(ctx, "ALTER TABLE "+qualified+" "+change); err != nil {
				t.Fatal(err)
			}
			if err := s.EnsureSchema(ctx, "items", model); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("unsupported feature admitted: %v", err)
			}
		})
	}
}
