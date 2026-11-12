package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

func driftModel() rowmodel.Schema {
	return rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
	}, PrimaryKey: []string{"id"}}
}

func TestPhysicalSchema(t *testing.T) {
	model := driftModel()
	model.Engine = engine
	model.Fields[0].Native = "int8"
	got, err := physicalSchema("items", model)
	if err != nil || got.Fields[0].Native != "bigint" {
		t.Fatalf("normalization: %+v %v", got, err)
	}
	for _, f := range []rowmodel.Field{
		{Name: "name", Logical: rowmodel.LogicalString, Native: "varchar(20)"},
		{Name: "name", Logical: rowmodel.LogicalArray, Native: "text[]"},
		{Name: "name", Logical: rowmodel.LogicalString, Native: "text; DROP TABLE items"},
		{Name: "name"},
	} {
		model.Fields[1] = f
		if _, err := physicalSchema("items", model); !errors.Is(err, schema.ErrBlocked) {
			t.Fatalf("unsupported type admitted: %+v %v", f, err)
		}
	}
	for _, value := range []string{"", "disabled", "strict", "nullable_add"} {
		if _, err := schemaPolicy(value); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := schemaPolicy("typo"); err == nil {
		t.Fatal("unknown policy admitted")
	}
}

func driftSink(t *testing.T, policy string, ingestion filament.IngestionType, continuous bool) (context.Context, *Sink, filament.RunSpec) {
	return driftSinkLifecycle(t, policy, ingestion, continuous, false)
}

func driftSinkLifecycle(t *testing.T, policy string, ingestion filament.IngestionType, continuous, explicit bool) (context.Context, *Sink, filament.RunSpec) {
	t.Helper()
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set FILAMENT_TEST_POSTGRES_DSN to a scratch database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	namespace := "drift_" + uuid.NewString()[:8]
	spec := filament.RunSpec{Run: "run", Resources: []string{"items"}, Sink: filament.Ref{Config: map[string]any{"dsn": dsn, "schema": namespace, "schema_evolution": policy}}, WritePolicies: map[string]filament.WritePolicy{"items": {Capability: filament.WriteCapabilities(ingestion)[0]}}}
	if continuous || explicit {
		p := spec.WritePolicies["items"]
		p.Capability.Durability = filament.DurabilityAfterCommit
		spec.WritePolicies["items"] = p
	}
	if continuous {
		spec.Options.Execution = filament.ExecutionContinuous
		spec.StreamAttempt = &filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}
	}
	s := New()
	var openErr error
	if explicit {
		spec.ExecutionID = "bounded-session"
		openErr = s.Open(ctx, spec)
	} else {
		openErr = s.Open(ctx, spec)
	}
	if err := openErr; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s.session.active != nil {
			_ = s.Abort(ctx)
		}
		_, err := s.pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{namespace}.Sanitize()+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		s.release()
	})
	return ctx, s, spec
}

func TestSchemaLockHeldUntilWriteTransactionEnds(t *testing.T) {
	ctx, s, _ := driftSink(t, "strict", filament.IngestionFullAppend, false)
	if err := s.EnsureSchema(ctx, "items", driftModel()); err != nil {
		t.Fatal(err)
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.validateTable(ctx, tx, s.tables["items"]); err != nil {
		t.Fatal(err)
	}
	other, err := s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Rollback(ctx)
	if _, err := other.Exec(ctx, "SET LOCAL lock_timeout='100ms'"); err != nil {
		t.Fatal(err)
	}
	_, err = other.Exec(ctx, "ALTER TABLE "+pgx.Identifier{s.schema, "items"}.Sanitize()+" ADD COLUMN extra text")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "55P03" {
		t.Fatalf("concurrent DDL was not excluded: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := s.pool.Exec(ctx, "DROP TABLE "+pgx.Identifier{s.schema, "items"}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	tx, err = s.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err := s.validateTable(ctx, tx, s.tables["items"]); !errors.Is(err, schema.ErrBlocked) {
		t.Fatalf("removed table did not block: %v", err)
	}
}
