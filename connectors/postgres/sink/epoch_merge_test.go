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

func TestCommitDurableUpsert(t *testing.T) {
	ctx, s, spec := driftSinkLifecycle(t, "strict", filament.IngestionFullUpsert, false, true)
	if err := s.EnsureSchema(ctx, "items", driftModel()); err != nil {
		t.Fatal(err)
	}
	out := &collect{}
	b := arrowbatch.NewBuilder(arrowbatch.Schema(driftModel()), nil, arrowbatch.Options{MaxRows: 1}, out)
	b.Int64(1)
	b.String("one")
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	batch := out.chunks[0]
	defer batch.Release()
	batch.Resource = "items"
	for _, commit := range []bool{false, true} {
		if err := s.Begin(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Apply(ctx, batch, filament.ApplyOptions{Policy: spec.WritePolicies["items"]}); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.tables["items"].qualified).Scan(&n); err != nil || n != 0 {
			t.Fatalf("uncommitted rows visible: %d %v", n, err)
		}
		if commit {
			if err := s.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := s.Abort(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	var n int
	if err := s.pool.QueryRow(ctx, "SELECT count(*) FROM "+s.tables["items"].qualified).Scan(&n); err != nil || n != 1 {
		t.Fatalf("committed row missing: %d %v", n, err)
	}
}
