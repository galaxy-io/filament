package postgres

import (
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestEpochMergeCommitAndAbort(t *testing.T) {
	for _, bounded := range []bool{false, true} {
		name := "continuous"
		if bounded {
			name = "bounded"
		}
		t.Run(name, func(t *testing.T) { testEpochMerge(t, bounded) })
	}
}

func testEpochMerge(t *testing.T, bounded bool) {
	ctx, s, spec := driftSinkLifecycle(t, "strict", filament.IngestionCDCMerge, !bounded, bounded)
	model := driftModel()
	if err := s.EnsureSchema(ctx, "items", model); err != nil {
		t.Fatal(err)
	}
	apply := func(ops ...rowmodel.Operation) {
		t.Helper()
		out := &collect{}
		b := arrowbatch.NewBuilder(arrowbatch.Schema(model), nil, arrowbatch.Options{MaxRows: 100}, out)
		for _, op := range ops {
			b.Int64(1)
			b.String("latest")
			if err := b.EndRow(rowmodel.Meta{Op: op}); err != nil {
				t.Fatal(err)
			}
		}
		if err := b.Flush(); err != nil {
			t.Fatal(err)
		}
		if err := b.Close(); err != nil {
			t.Fatal(err)
		}
		for _, batch := range out.chunks {
			batch.Resource = "items"
			_, err := s.Apply(ctx, batch, filament.ApplyOptions{Policy: spec.WritePolicies["items"]})
			batch.Release()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	count := func(want int) {
		t.Helper()
		var got int
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{s.schema, "items"}.Sanitize()).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("rows=%d want=%d", got, want)
		}
	}
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	apply(rowmodel.OpInsert, rowmodel.OpUpdate, rowmodel.OpDelete, rowmodel.OpInsert)
	count(0)
	if err := s.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	count(1)
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	apply(rowmodel.OpDelete)
	if err := s.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	count(1)
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	apply(rowmodel.OpDelete)
	if err := s.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	count(0)
}
