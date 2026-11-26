package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// TestMergePreexistingEnumAndDomainColumns replays the #825 repro against
// tables that already carry an enum or a domain column: the source projects
// both as text, the destination types them as mood and pos_int, and the merge
// fold must cast the staged values instead of failing with 42804.
func TestMergePreexistingEnumAndDomainColumns(t *testing.T) {
	t.Run("enum column", func(t *testing.T) {
		ctx, s, policy := mergeSink(t)
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_enum"}.Sanitize()+` (id integer PRIMARY KEY, m mood)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_enum", Engine: engine, Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt32, Native: "integer"},
			{Name: "m", Logical: rowmodel.LogicalString, Native: "mood"},
		}, PrimaryKey: []string{"id"}}
		if err := s.EnsureSchema(ctx, "t_enum", model); err != nil {
			t.Fatal(err)
		}
		if err := mergeApply(t, ctx, s, policy, "t_enum", model, rowmodel.OpInsert, func(b *arrowbatch.Builder) { b.Int32(1); b.String("calm") }); err != nil {
			t.Fatal(err)
		}
		if got := mergeOne(t, ctx, s, `SELECT m::text FROM `+pgx.Identifier{s.schema, "t_enum"}.Sanitize()+` WHERE id = 1`); got != "calm" {
			t.Fatalf("m = %q, want calm", got)
		}
		if err := mergeApply(t, ctx, s, policy, "t_enum", model, rowmodel.OpUpdate, func(b *arrowbatch.Builder) { b.Int32(1); b.String("panic") }); err != nil {
			t.Fatal(err)
		}
		if got := mergeOne(t, ctx, s, `SELECT m::text FROM `+pgx.Identifier{s.schema, "t_enum"}.Sanitize()+` WHERE id = 1`); got != "panic" {
			t.Fatalf("m = %q, want panic", got)
		}
	})

	t.Run("domain column", func(t *testing.T) {
		ctx, s, policy := mergeSink(t)
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_dom"}.Sanitize()+` (id integer PRIMARY KEY, c_pos pos_int)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_dom", Engine: engine, Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt32, Native: "integer"},
			{Name: "c_pos", Logical: rowmodel.LogicalString, Native: "pos_int"},
		}, PrimaryKey: []string{"id"}}
		if err := s.EnsureSchema(ctx, "t_dom", model); err != nil {
			t.Fatal(err)
		}
		if err := mergeApply(t, ctx, s, policy, "t_dom", model, rowmodel.OpInsert, func(b *arrowbatch.Builder) { b.Int32(1); b.String("5") }); err != nil {
			t.Fatal(err)
		}
		if got := mergeOne(t, ctx, s, `SELECT c_pos::text FROM `+pgx.Identifier{s.schema, "t_dom"}.Sanitize()+` WHERE id = 1`); got != "5" {
			t.Fatalf("c_pos = %q, want 5", got)
		}
		// A row that fails the domain check must fail the run on the check,
		// not on the cast.
		err := mergeApply(t, ctx, s, policy, "t_dom", model, rowmodel.OpInsert, func(b *arrowbatch.Builder) { b.Int32(2); b.String("-1") })
		if err == nil {
			t.Fatal("domain check violation accepted")
		}
		if strings.Contains(err.Error(), "42804") {
			t.Fatalf("domain failure is still a cast error: %v", err)
		}
		if !strings.Contains(err.Error(), "check constraint") {
			t.Fatalf("domain failure is not a check violation: %v", err)
		}
		mergeCount(t, ctx, s, "t_dom", 1)
	})

	t.Run("enum primary key", func(t *testing.T) {
		ctx, s, policy := mergeSink(t)
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_enum_pk"}.Sanitize()+` (m mood PRIMARY KEY, v integer)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_enum_pk", Engine: engine, Fields: []rowmodel.Field{
			{Name: "m", Logical: rowmodel.LogicalString, Native: "mood"},
			{Name: "v", Logical: rowmodel.LogicalInt32, Native: "integer"},
		}, PrimaryKey: []string{"m"}}
		if err := s.EnsureSchema(ctx, "t_enum_pk", model); err != nil {
			t.Fatal(err)
		}
		if err := mergeApply(t, ctx, s, policy, "t_enum_pk", model, rowmodel.OpInsert, func(b *arrowbatch.Builder) { b.String("calm"); b.Int32(1) }); err != nil {
			t.Fatal(err)
		}
		mergeCount(t, ctx, s, "t_enum_pk", 1)
		if err := mergeApply(t, ctx, s, policy, "t_enum_pk", model, rowmodel.OpDelete, func(b *arrowbatch.Builder) { b.String("calm"); b.Int32(1) }); err != nil {
			t.Fatal(err)
		}
		mergeCount(t, ctx, s, "t_enum_pk", 0)
	})
}

// mergeSink opens a legacy-mode merge sink on its own scratch schema and
// creates the pre-existing types from the issue repro. The DO blocks keep a
// rerun against the same scratch database from failing on the second create.
// Each subtest takes its own sink because a failed Apply retains the failure
// on the session lifecycle and prevents its reuse.
func mergeSink(t *testing.T) (context.Context, *Sink, filament.WritePolicy) {
	t.Helper()
	ctx, s, spec := driftSink(t, "", filament.IngestionCDCMerge, false)
	for _, ddl := range []string{
		`DO $$ BEGIN CREATE TYPE mood AS ENUM ('calm', 'alert', 'panic'); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
		`DO $$ BEGIN CREATE DOMAIN pos_int AS integer CHECK (VALUE > 0); EXCEPTION WHEN duplicate_object THEN NULL; END $$`,
	} {
		if _, err := s.pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, s, spec.WritePolicies["items"]
}

// mergeApply applies one single-row batch as one write boundary.
func mergeApply(t *testing.T, ctx context.Context, s *Sink, policy filament.WritePolicy, resource string, model rowmodel.Schema, op rowmodel.Operation, write func(b *arrowbatch.Builder)) error {
	t.Helper()
	if err := s.Begin(ctx); err != nil {
		return err
	}
	out := &collect{}
	b := arrowbatch.NewBuilder(arrowbatch.Schema(model), nil, arrowbatch.Options{MaxRows: 100}, out)
	write(b)
	if err := b.EndRow(rowmodel.Meta{Op: op}); err != nil {
		return err
	}
	if err := b.Flush(); err != nil {
		return err
	}
	if err := b.Close(); err != nil {
		return err
	}
	var err error
	for _, batch := range out.chunks {
		batch.Resource = resource
		_, applyErr := s.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
		batch.Release()
		if applyErr != nil {
			err = applyErr
		}
	}
	if err != nil {
		return err
	}
	return s.Commit(ctx)
}

func mergeCount(t *testing.T, ctx context.Context, s *Sink, resource string, want int) {
	t.Helper()
	var got int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{s.schema, resource}.Sanitize()).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s rows=%d want=%d", resource, got, want)
	}
}

func mergeOne(t *testing.T, ctx context.Context, s *Sink, query string) string {
	t.Helper()
	var got string
	if err := s.pool.QueryRow(ctx, query).Scan(&got); err != nil {
		t.Fatal(err)
	}
	return got
}
