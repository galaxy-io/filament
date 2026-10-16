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
	"github.com/galaxy-io/filament/arrowbatch"
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
	t.Helper()
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set FILAMENT_TEST_POSTGRES_DSN to a scratch database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	namespace := "drift_" + uuid.NewString()[:8]
	spec := filament.RunSpec{Run: "run", Resources: []string{"items"}, Sink: filament.Ref{Config: map[string]any{"dsn": dsn, "schema": namespace, "schema_evolution": policy}}, WritePolicies: map[string]filament.WritePolicy{"items": {Capability: filament.WriteCapabilities(ingestion)[0]}}}
	if continuous {
		spec.Options.Execution = filament.ExecutionContinuous
		spec.StreamAttempt = &filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}
	}
	s := New()
	if err := s.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if s.stream.active != nil {
			_ = s.AbortEpoch(ctx, s.stream.active.ref)
		}
		_, err := s.pool.Exec(ctx, "DROP SCHEMA "+pgx.Identifier{namespace}.Sanitize()+" CASCADE")
		if err != nil {
			t.Error(err)
		}
		s.release()
	})
	return ctx, s, spec
}

func TestReconcileBlocksBeforeMutation(t *testing.T) {
	for _, tc := range []struct {
		name, policy string
		change       func(*rowmodel.Schema)
	}{
		{"strict addition", "strict", func(m *rowmodel.Schema) {
			m.Fields = append(m.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
		}},
		{"type with allowed addition", "nullable_add", func(m *rowmodel.Schema) {
			m.Fields[1].Logical = rowmodel.LogicalBytes
			m.Fields = append(m.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
		}},
		{"drop", "nullable_add", func(m *rowmodel.Schema) { m.Fields = m.Fields[:1] }},
		{"key", "nullable_add", func(m *rowmodel.Schema) { m.PrimaryKey = nil }},
		{"nullability", "nullable_add", func(m *rowmodel.Schema) { m.Fields[1].Nullable = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, s, _ := driftSink(t, tc.policy, filament.IngestionFullReplace, false)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			qualified := pgx.Identifier{s.schema, "items"}.Sanitize()
			if _, err := s.pool.Exec(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
				t.Fatal(err)
			}
			tc.change(&model)
			var detected []schema.Detection
			ctx = schema.WithObserver(ctx, func(resource string, d schema.Detection) {
				if resource != "items" {
					t.Errorf("wrong resource %q", resource)
				}
				detected = append(detected, d)
			})
			if err := s.EnsureSchema(ctx, "items", model); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("expected schema block: %v", err)
			}
			if len(detected) != 1 || detected[0].Side != "sink" || len(detected[0].Changes) == 0 {
				t.Fatalf("blocked diff not observed: %+v", detected)
			}
			var n int
			if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+qualified).Scan(&n); err != nil || n != 1 {
				t.Fatalf("blocked replacement lost rows: %d %v", n, err)
			}
			if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='items'", s.schema).Scan(&n); err != nil || n != 2 {
				t.Fatalf("blocked diff partially applied: %d %v", n, err)
			}
		})
	}
}

func TestReconcileNullableAddAndRetry(t *testing.T) {
	ctx, s, spec := driftSink(t, "nullable_add", filament.IngestionFullAppend, false)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	qualified := pgx.Identifier{s.schema, "items"}.Sanitize()
	if _, err := s.pool.Exec(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
		t.Fatal(err)
	}
	model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	// A new process needs only the current table, including after an ambiguous
	// DDL commit. It does not need the predecessor's in-memory result.
	retry := New()
	if err := retry.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer retry.release()
	if err := retry.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+qualified+" WHERE extra IS NULL").Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows not preserved: %d %v", n, err)
	}
	model.Fields[0], model.Fields[1] = model.Fields[1], model.Fields[0]
	retry.evolution = schema.Strict
	if err := retry.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatalf("named COPY should allow reordering: %v", err)
	}
}

func TestSchemaChecksBeforeWrites(t *testing.T) {
	for _, ingestion := range []filament.IngestionType{filament.IngestionFullAppend, filament.IngestionFullUpsert, filament.IngestionCDCMerge} {
		t.Run(string(ingestion), func(t *testing.T) {
			ctx, s, spec := driftSink(t, "strict", ingestion, false)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			col := &collect{}
			builder := arrowbatch.NewBuilder(arrowbatch.Schema(model), nil, arrowbatch.Options{MaxRows: 1}, col)
			builder.Int64(1)
			builder.String("one")
			if err := builder.EndRow(rowmodel.Meta{}); err != nil {
				t.Fatal(err)
			}
			defer builder.Close()
			batch := col.chunks[0]
			defer batch.Release()
			batch.Resource = "items"
			if _, err := s.Apply(ctx, batch, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); err != nil {
				t.Fatal(err)
			}
			qualified := pgx.Identifier{s.schema, "items"}.Sanitize()
			if _, err := s.pool.Exec(ctx, "ALTER TABLE "+qualified+" ALTER COLUMN name SET NOT NULL"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Apply(ctx, batch, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("write admitted after external drift: %v", err)
			}
		})
	}
}

func TestEpochSchemaChangeRollsBack(t *testing.T) {
	ctx, s, spec := driftSink(t, "nullable_add", filament.IngestionFullAppend, true)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	ref := filament.EpochRef{Attempt: *spec.StreamAttempt, Epoch: 1, MembershipRevision: 1}
	if err := s.BeginEpoch(ctx, ref); err != nil {
		t.Fatal(err)
	}
	model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	if err := s.AbortEpoch(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if len(s.tables["items"].model.Fields) != 2 {
		t.Fatal("aborted writer schema survived")
	}
	if err := s.EnsureSchema(ctx, "items", driftModel()); err != nil {
		t.Fatalf("aborted DDL survived: %v", err)
	}
	qualified := pgx.Identifier{s.schema, "items"}.Sanitize()
	if _, err := s.pool.Exec(ctx, "ALTER TABLE "+qualified+" ALTER COLUMN name SET NOT NULL"); err != nil {
		t.Fatal(err)
	}
	ref.Epoch++
	if err := s.BeginEpoch(ctx, ref); !errors.Is(err, schema.ErrBlocked) {
		t.Fatalf("epoch admitted after external drift: %v", err)
	}
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
