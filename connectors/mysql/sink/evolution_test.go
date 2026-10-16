package mysql

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

func driftModel() rowmodel.Schema {
	return rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}, {Name: "name", Logical: rowmodel.LogicalString, Nullable: true}}, PrimaryKey: []string{"id"}}
}

func driftSink(t *testing.T, policy string, mode filament.IngestionType) (context.Context, *Sink, filament.RunSpec) {
	t.Helper()
	dsn := os.Getenv("FILAMENT_TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set FILAMENT_TEST_MYSQL_DSN to an isolated MySQL server")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	spec := filament.RunSpec{Run: "run", Resources: []string{"items"}, Sink: filament.Ref{Config: map[string]any{"dsn": dsn, "database": "drift_" + uuid.NewString()[:8], "schema_evolution": policy}}, WritePolicies: map[string]filament.WritePolicy{"items": {Capability: filament.WriteCapabilities(mode)[0]}}}
	s := New()
	if err := s.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := s.db.ExecContext(ctx, "DROP DATABASE "+quoteIdent(s.database))
		if err != nil {
			t.Error(err)
		}
		s.release()
	})
	return ctx, s, spec
}

func TestCheckedPhysicalSchema(t *testing.T) {
	for _, f := range []rowmodel.Field{{Name: "bad", Logical: rowmodel.LogicalArray}, {Name: "bad", Logical: rowmodel.LogicalInt64, Native: "bigint unsigned"}, {Name: "bad", Logical: rowmodel.LogicalDecimal, Precision: 66}} {
		model := driftModel()
		model.Engine = engine
		model.Fields[1] = f
		if _, err := physicalSchema("items", model); !errors.Is(err, schema.ErrBlocked) {
			t.Fatalf("unsupported field admitted: %+v %v", f, err)
		}
	}
	if _, err := schemaPolicy("typo"); err == nil {
		t.Fatal("invalid policy accepted")
	}
}

func TestCheckedReconcileBlocksBeforeReplace(t *testing.T) {
	for _, mode := range []string{"strict", "type", "required", "drop", "key"} {
		t.Run(mode, func(t *testing.T) {
			policy := "nullable_add"
			if mode == "strict" {
				policy = "strict"
			}
			ctx, s, _ := driftSink(t, policy, filament.IngestionFullReplace)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			qualified := s.tables["items"].qualified
			if _, err := s.db.ExecContext(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
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
			if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+qualified).Scan(&count); err != nil || count != 1 {
				t.Fatalf("replace removed rows: %d %v", count, err)
			}
			if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA=? AND TABLE_NAME='items'", s.database).Scan(&count); err != nil || count != 2 {
				t.Fatalf("partial migration: %d %v", count, err)
			}
		})
	}
}

func TestCheckedNullableAddRestartAndWriteDrift(t *testing.T) {
	ctx, s, spec := driftSink(t, "nullable_add", filament.IngestionFullAppend)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	qualified := s.tables["items"].qualified
	if _, err := s.db.ExecContext(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
		t.Fatal(err)
	}
	model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalBool, Nullable: true}, rowmodel.Field{Name: "other", Logical: rowmodel.LogicalJSON, Nullable: true})
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	// DDL is durable without a run Commit, so a new sink must inspect and converge.
	retry := New()
	if err := retry.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer retry.release()
	if err := retry.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+qualified+" WHERE extra IS NULL AND other IS NULL").Scan(&count); err != nil || count != 1 {
		t.Fatalf("existing rows changed: %d %v", count, err)
	}
	c := &collect{}
	b := arrowbatch.NewBuilder(arrowbatch.Schema(model), nil, arrowbatch.Options{MaxRows: 1}, c)
	defer b.Close()
	b.Int64(2)
	b.String("new")
	b.Bool(true)
	b.String(`{"ok":true}`)
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	chunk := c.chunks[0]
	defer chunk.Release()
	chunk.Resource = "items"
	if _, err := retry.Apply(ctx, chunk, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); err != nil {
		t.Fatal(err)
	}

	if _, err := s.db.ExecContext(ctx, "ALTER TABLE "+qualified+" MODIFY COLUMN name LONGBLOB"); err != nil {
		t.Fatal(err)
	}
	if _, err := retry.Apply(ctx, chunk, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); !errors.Is(err, schema.ErrBlocked) {
		t.Fatalf("write drift not blocked: %v", err)
	}
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM "+qualified).Scan(&count); err != nil || count != 2 {
		t.Fatalf("blocked write changed rows: %d %v", count, err)
	}
}

func TestCheckedWriteHoldsMetadataLock(t *testing.T) {
	ctx, s, _ := driftSink(t, "nullable_add", filament.IngestionFullAppend)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	c := &collect{}
	builder := arrowbatch.NewBuilder(arrowbatch.Schema(model), nil, arrowbatch.Options{MaxRows: 1}, c)
	defer builder.Close()
	builder.Int64(1)
	builder.String("value")
	if err := builder.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	batch := c.chunks[0]
	defer batch.Release()
	batch.Resource = "items"
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := s.validateTable(ctx, tx, s.tables["items"], batch); err != nil {
		t.Fatal(err)
	}
	external, err := s.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer external.Close()
	if _, err := external.ExecContext(ctx, "SET SESSION lock_wait_timeout=1"); err != nil {
		t.Fatal(err)
	}
	ddl := "ALTER TABLE " + s.tables["items"].qualified + " ADD COLUMN extra BIGINT NULL"
	if _, err := external.ExecContext(ctx, ddl); err != nil {
		var server *mysqldriver.MySQLError
		if !errors.As(err, &server) || server.Number != 1205 {
			t.Fatalf("expected metadata lock timeout: %v", err)
		}
	} else {
		t.Fatal("external DDL bypassed write metadata lock")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := external.ExecContext(ctx, ddl); err != nil {
		t.Fatalf("lock not released after rollback: %v", err)
	}
}

func TestCheckedUnsupportedTableFeatures(t *testing.T) {
	for _, change := range []string{"MODIFY COLUMN id BIGINT UNSIGNED NOT NULL", "ADD COLUMN derived BIGINT GENERATED ALWAYS AS (id+1)", "ADD UNIQUE KEY duplicate_key (id)", "MODIFY COLUMN name LONGTEXT COLLATE utf8mb4_0900_ai_ci"} {
		t.Run(change, func(t *testing.T) {
			ctx, s, _ := driftSink(t, "nullable_add", filament.IngestionFullAppend)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.ExecContext(ctx, "ALTER TABLE "+s.tables["items"].qualified+" "+change); err != nil {
				t.Fatal(err)
			}
			if err := s.EnsureSchema(ctx, "items", model); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("unsupported feature admitted: %v", err)
			}
		})
	}
}
