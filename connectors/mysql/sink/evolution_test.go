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
