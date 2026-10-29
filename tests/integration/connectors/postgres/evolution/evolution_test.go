//go:build integration

package evolution_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func driftModel() rowmodel.Schema {
	return rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
	}, PrimaryKey: []string{"id"}}
}

// driftSink opens a sink on its own schema in the shared database and returns
// that schema's name.
func driftSink(t *testing.T, pg *testcontainers.PG, policy string, ingestion filament.IngestionType, continuous bool) (context.Context, *pgsink.Sink, filament.RunSpec, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	namespace := "drift_" + uuid.NewString()[:8]
	spec := filament.RunSpec{Run: "run", Resources: []string{"items"}, Sink: filament.Ref{Config: map[string]any{"dsn": pg.DSN(), "schema": namespace, "schema_evolution": policy}}, WritePolicies: map[string]filament.WritePolicy{"items": {Capability: filament.WriteCapabilities(ingestion)[0]}}}
	if continuous {
		p := spec.WritePolicies["items"]
		p.Capability.Durability = filament.DurabilityAfterCommit
		spec.WritePolicies["items"] = p
		spec.Options.Execution = filament.ExecutionContinuous
		spec.StreamAttempt = &filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}
	}
	s := pgsink.New()
	if err := s.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close(ctx) })
	return ctx, s, spec, namespace
}

func items(namespace string) string { return pgx.Identifier{namespace, "items"}.Sanitize() }

// withPolicy returns spec with a different schema_evolution setting.
func withPolicy(spec filament.RunSpec, policy string) filament.RunSpec {
	spec.Sink.Config = maps.Clone(spec.Sink.Config)
	spec.Sink.Config["schema_evolution"] = policy
	return spec
}

// oneRow builds a single-row batch for model with the given id and name.
func oneRow(t *testing.T, model rowmodel.Schema, id int64, name string) *arrowbatch.Batch {
	t.Helper()
	col := &testutil.CollectSink{}
	t.Cleanup(col.Release)
	w, err := col.Builder("items", 0, model)
	if err != nil {
		t.Fatal(err)
	}
	w.Int64(id)
	w.String(name)
	if err := w.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	return col.BatchesFor("items")[0]
}

// columns counts the items table's columns, or those with the given name.
func columns(ctx context.Context, pg *testcontainers.PG, namespace, column string) (int, error) {
	var n int
	err := pg.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.columns WHERE table_schema=$1 AND table_name='items' AND ($2='' OR column_name=$2)", namespace, column).Scan(&n)
	return n, err
}

func TestPostgresReconcileBlocksBeforeMutation(t *testing.T) {
	pg := testcontainers.SharedPostgres(t)
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
			ctx, s, _, ns := driftSink(t, pg, tc.policy, filament.IngestionFullReplace, false)
			qualified := items(ns)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			if _, err := pg.Pool().Exec(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
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
			if err := pg.Pool().QueryRow(ctx, "SELECT count(*) FROM "+qualified).Scan(&n); err != nil || n != 1 {
				t.Fatalf("blocked replacement lost rows: %d %v", n, err)
			}
			if n, err := columns(ctx, pg, ns, ""); err != nil || n != 2 {
				t.Fatalf("blocked diff partially applied: %d %v", n, err)
			}
		})
	}
}

func TestPostgresReconcileNullableAddAndRetry(t *testing.T) {
	pg := testcontainers.SharedPostgres(t)
	ctx, s, spec, ns := driftSink(t, pg, "nullable_add", filament.IngestionFullAppend, false)
	qualified := items(ns)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool().Exec(ctx, "INSERT INTO "+qualified+" VALUES (1,'kept')"); err != nil {
		t.Fatal(err)
	}
	model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	// A new process needs only the current table, including after an ambiguous
	// DDL commit. It does not need the predecessor's in-memory result.
	retry := pgsink.New()
	if err := retry.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer retry.Close(ctx)
	if err := retry.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pg.Pool().QueryRow(ctx, "SELECT count(*) FROM "+qualified+" WHERE extra IS NULL").Scan(&n); err != nil || n != 1 {
		t.Fatalf("rows not preserved: %d %v", n, err)
	}
	model.Fields[0], model.Fields[1] = model.Fields[1], model.Fields[0]
	strict := pgsink.New()
	if err := strict.Open(ctx, withPolicy(spec, "strict")); err != nil {
		t.Fatal(err)
	}
	defer strict.Close(ctx)
	if err := strict.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatalf("named COPY should allow reordering: %v", err)
	}
}

func TestPostgresSchemaChecksBeforeWrites(t *testing.T) {
	pg := testcontainers.SharedPostgres(t)
	for _, ingestion := range []filament.IngestionType{filament.IngestionFullAppend, filament.IngestionFullUpsert, filament.IngestionCDCMerge} {
		t.Run(string(ingestion), func(t *testing.T) {
			ctx, s, spec, ns := driftSink(t, pg, "strict", ingestion, false)
			qualified := items(ns)
			model := driftModel()
			if err := s.EnsureSchema(ctx, "items", model); err != nil {
				t.Fatal(err)
			}
			batch := oneRow(t, model, 1, "one")
			if err := s.Begin(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Apply(ctx, batch, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); err != nil {
				t.Fatal(err)
			}
			if err := s.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := pg.Pool().Exec(ctx, "ALTER TABLE "+qualified+" ALTER COLUMN name SET NOT NULL"); err != nil {
				t.Fatal(err)
			}
			if spec.WritePolicies["items"].Capability.Durability == filament.DurabilityAfterCommit {
				if err := s.Begin(ctx); !errors.Is(err, schema.ErrBlocked) {
					t.Fatalf("drift admitted: %v", err)
				}
				return
			}
			if err := s.Begin(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Apply(ctx, batch, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); !errors.Is(err, schema.ErrBlocked) {
				t.Fatalf("write admitted after external drift: %v", err)
			}
		})
	}
}

func TestPostgresEpochSchemaChangeRollsBack(t *testing.T) {
	pg := testcontainers.SharedPostgres(t)
	ctx, s, spec, ns := driftSink(t, pg, "nullable_add", filament.IngestionFullAppend, true)
	qualified := items(ns)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	ref := filament.EpochRef{Attempt: *spec.StreamAttempt, MembershipRevision: 1}
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	model.Fields = append(model.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	if err := s.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	if n, err := columns(ctx, pg, ns, "extra"); err != nil || n != 0 {
		t.Fatalf("aborted DDL survived: %d %v", n, err)
	}
	if err := s.EnsureSchema(ctx, "items", driftModel()); err != nil {
		t.Fatalf("aborted writer schema survived: %v", err)
	}
	if _, err := pg.Pool().Exec(ctx, "ALTER TABLE "+qualified+" ALTER COLUMN name SET NOT NULL"); err != nil {
		t.Fatal(err)
	}
	ref.Epoch++
	if err := s.Begin(ctx); !errors.Is(err, schema.ErrBlocked) {
		t.Fatalf("epoch admitted after external drift: %v", err)
	}
}
