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
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_enum"}.Sanitize()+` (id integer PRIMARY KEY, m `+pgx.Identifier{s.schema, "mood"}.Sanitize()+`)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_enum", Engine: engine, Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt32, Native: "integer"},
			{Name: "m", Logical: rowmodel.LogicalString, Native: pgx.Identifier{s.schema, "mood"}.Sanitize()},
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
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_dom"}.Sanitize()+` (id integer PRIMARY KEY, c_pos `+pgx.Identifier{s.schema, "pos_int"}.Sanitize()+`)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_dom", Engine: engine, Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt32, Native: "integer"},
			{Name: "c_pos", Logical: rowmodel.LogicalString, Native: pgx.Identifier{s.schema, "pos_int"}.Sanitize()},
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
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_enum_pk"}.Sanitize()+` (m `+pgx.Identifier{s.schema, "mood"}.Sanitize()+` PRIMARY KEY, v integer)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_enum_pk", Engine: engine, Fields: []rowmodel.Field{
			{Name: "m", Logical: rowmodel.LogicalString, Native: pgx.Identifier{s.schema, "mood"}.Sanitize()},
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

	t.Run("domain over length-constrained string", func(t *testing.T) {
		ctx, s, policy := mergeSink(t)
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_short"}.Sanitize()+` (id integer PRIMARY KEY, c_short `+pgx.Identifier{s.schema, "short_text"}.Sanitize()+`)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_short", Engine: engine, Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt32, Native: "integer"},
			{Name: "c_short", Logical: rowmodel.LogicalString, Native: pgx.Identifier{s.schema, "short_text"}.Sanitize()},
		}, PrimaryKey: []string{"id"}}
		if err := s.EnsureSchema(ctx, "t_short", model); err != nil {
			t.Fatal(err)
		}
		if err := mergeApply(t, ctx, s, policy, "t_short", model, rowmodel.OpInsert, func(b *arrowbatch.Builder) { b.Int32(1); b.String("abc") }); err != nil {
			t.Fatal(err)
		}
		if got := mergeOne(t, ctx, s, `SELECT c_short::text FROM `+pgx.Identifier{s.schema, "t_short"}.Sanitize()+` WHERE id = 1`); got != "abc" {
			t.Fatalf("c_short = %q, want abc", got)
		}
		// An over-length value must fail on the length check, not be
		// silently truncated by the merge cast and not fail with 42804:
		// text takes the short_text domain by assignment, so the fold
		// keeps the uncast assignment path.
		err := mergeApply(t, ctx, s, policy, "t_short", model, rowmodel.OpInsert, func(b *arrowbatch.Builder) { b.Int32(2); b.String("abcd") })
		if err == nil {
			t.Fatal("oversized string accepted")
		}
		if strings.Contains(err.Error(), "42804") {
			t.Fatalf("oversized string failed as a cast error: %v", err)
		}
		if !strings.Contains(err.Error(), "value too long") {
			t.Fatalf("oversized string did not fail on length: %v", err)
		}
		mergeCount(t, ctx, s, "t_short", 1)
	})

	t.Run("enum type named like the staging table", func(t *testing.T) {
		ctx, s, policy := mergeSink(t)
		// A destination type whose name matches the staging temp table
		// (_filament_<resource>) must still resolve to the type, not the
		// temp table's row type: the cast target is schema-qualified.
		// The type has to live on the search_path for the collision to
		// happen at all, so it is created in public and dropped again
		// here rather than left to the schema cleanup.
		typ := pgx.Identifier{"public", "_filament_t_enum"}.Sanitize()
		if _, err := s.pool.Exec(ctx, `DROP TYPE IF EXISTS `+typ); err != nil {
			t.Fatal(err)
		}
		if _, err := s.pool.Exec(ctx, `CREATE TYPE `+typ+` AS ENUM ('calm', 'panic')`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = s.pool.Exec(ctx, `DROP TYPE IF EXISTS `+typ+` CASCADE`)
		})
		if _, err := s.pool.Exec(ctx, `CREATE TABLE `+pgx.Identifier{s.schema, "t_enum"}.Sanitize()+` (id integer PRIMARY KEY, m `+typ+`)`); err != nil {
			t.Fatal(err)
		}
		model := rowmodel.Schema{Resource: "t_enum", Engine: engine, Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt32, Native: "integer"},
			{Name: "m", Logical: rowmodel.LogicalString, Native: typ},
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
	})
}

// mergeSink opens a legacy-mode merge sink on its own scratch schema and
// creates the pre-existing types from the issue repro inside that schema,
// so each subtest owns its definitions and the schema cleanup drops them.
// Each subtest takes its own sink because a failed Apply retains the failure
// on the session lifecycle and prevents its reuse.
func mergeSink(t *testing.T) (context.Context, *Sink, filament.WritePolicy) {
	t.Helper()
	ctx, s, spec := driftSink(t, "", filament.IngestionCDCMerge, false)
	for _, ddl := range []string{
		`CREATE TYPE ` + pgx.Identifier{s.schema, "mood"}.Sanitize() + ` AS ENUM ('calm', 'alert', 'panic')`,
		`CREATE DOMAIN ` + pgx.Identifier{s.schema, "pos_int"}.Sanitize() + ` AS integer CHECK (VALUE > 0)`,
		`CREATE DOMAIN ` + pgx.Identifier{s.schema, "short_text"}.Sanitize() + ` AS varchar(3)`,
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
