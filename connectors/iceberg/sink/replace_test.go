package iceberg

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
)

// runRows runs one sink lifecycle writing ids [start, end) under ingestion.
// It returns the Commit error; commit=false aborts instead. prepare, when set,
// may replace the sink's catalog after Open.
func runRows(t *testing.T, env *testEnv, ingestion filament.IngestionType, start, end int, commit bool, prepare func(*Sink)) error {
	t.Helper()
	ctx := context.Background()
	policy := eventsPolicy(ingestion)
	sink := New()
	if err := sink.Open(ctx, filament.RunSpec{
		Run: "run", StartedAt: time.Now(), Resources: []string{"events"},
		Sink:          filament.Ref{Connector: "iceberg", Config: env.cfg},
		WritePolicies: map[string]filament.WritePolicy{"events": policy},
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sink.Close(ctx) }()
	if prepare != nil {
		prepare(sink)
	}
	if err := sink.EnsureSchema(ctx, "events", env.schema); err != nil {
		t.Fatal(err)
	}
	if err := sink.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	for from := start; from < end; from += 1000 {
		batch := idBatch(t, from, min(1000, end-from))
		if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
			t.Fatal(err)
		}
		batch.Release()
	}
	if !commit {
		if err := sink.Abort(ctx); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	return sink.Commit(ctx)
}

func mustRun(t *testing.T, env *testEnv, ingestion filament.IngestionType, start, end int) {
	t.Helper()
	if err := runRows(t, env, ingestion, start, end, true, nil); err != nil {
		t.Fatal(err)
	}
}

// tableIDs returns the table's ids, sorted.
func tableIDs(t *testing.T, env *testEnv) []int64 {
	t.Helper()
	ctx := context.Background()
	sink := env.table(t)
	tbl, err := sink.cat.LoadTable(ctx, sink.tableIdent("events"))
	if err != nil {
		t.Fatal(err)
	}
	_, records, err := tbl.Scan().ToArrowRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rec, err := range records {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, rec.Column(0).(*array.Int64).Int64Values()...)
		rec.Release()
	}
	slices.Sort(ids)
	return ids
}

func idRange(start, end int) []int64 {
	ids := make([]int64, 0, end-start)
	for id := start; id < end; id++ {
		ids = append(ids, int64(id))
	}
	return ids
}

func assertIDs(t *testing.T, env *testEnv, want []int64) {
	t.Helper()
	if got := tableIDs(t, env); !slices.Equal(got, want) {
		t.Fatalf("table has %d ids (%v..), want %d (%v..)", len(got), head(got), len(want), head(want))
	}
}

func head(ids []int64) []int64 { return ids[:min(3, len(ids))] }

func TestReplaceSwapsTableContents(t *testing.T) {
	rotateEveryBatch(t) // several written files and several replaced files
	env := newTestEnv(t)
	mustRun(t, env, filament.IngestionFullReplace, 0, 3000) // into an empty table
	assertIDs(t, env, idRange(0, 3000))
	mustRun(t, env, filament.IngestionFullAppend, 3000, 4000)
	mustRun(t, env, filament.IngestionFullReplace, 10_000, 12_500)
	assertIDs(t, env, idRange(10_000, 12_500))
}

func TestReplaceWithNoRowsEmptiesTable(t *testing.T) {
	env := newTestEnv(t)
	mustRun(t, env, filament.IngestionFullAppend, 0, 2000)
	mustRun(t, env, filament.IngestionFullReplace, 0, 0)
	assertIDs(t, env, nil)
}

func TestReplaceAbortKeepsTableAndRemovesFiles(t *testing.T) {
	rotateEveryBatch(t)
	env := newTestEnv(t)
	mustRun(t, env, filament.IngestionFullAppend, 0, 2000)
	before := len(parquetFiles(t, env))
	if err := runRows(t, env, filament.IngestionFullReplace, 5000, 9000, false, nil); err != nil {
		t.Fatal(err)
	}
	assertIDs(t, env, idRange(0, 2000))
	if after := len(parquetFiles(t, env)); after != before {
		t.Fatalf("aborted replace left files: %d before, %d after", before, after)
	}
}

// TestReplaceConflictsWithConcurrentAppend commits another writer's append in
// the window between the replace listing the table's files and its catalog
// commit. Under the default serializable isolation the replace must fail
// rather than drop or keep the other writer's rows silently.
func TestReplaceConflictsWithConcurrentAppend(t *testing.T) {
	env := newTestEnv(t)
	mustRun(t, env, filament.IngestionFullAppend, 0, 1000)
	err := runRows(t, env, filament.IngestionFullReplace, 5000, 6000, true, func(sink *Sink) {
		sink.cat = &racingCatalog{Catalog: sink.cat, race: func() {
			mustRun(t, env, filament.IngestionFullAppend, 1000, 2000)
		}}
	})
	if err == nil {
		t.Fatal("replace committed over a concurrent append")
	}
	t.Logf("replace failed as expected: %v", err)
	assertIDs(t, env, idRange(0, 2000))
}

// racingCatalog runs race once, just before the first table commit reaches the
// catalog. Tables it loads commit through it.
type racingCatalog struct {
	icecatalog.Catalog
	race func()
	once sync.Once
}

func (c *racingCatalog) LoadTable(ctx context.Context, ident icetable.Identifier) (*icetable.Table, error) {
	tbl, err := c.Catalog.LoadTable(ctx, ident)
	if err != nil {
		return nil, err
	}
	return icetable.New(tbl.Identifier(), tbl.Metadata(), tbl.MetadataLocation(), tbl.FS, c), nil
}

func (c *racingCatalog) CommitTable(ctx context.Context, ident icetable.Identifier, reqs []icetable.Requirement, updates []icetable.Update) (icetable.Metadata, string, error) {
	c.once.Do(c.race)
	return c.Catalog.CommitTable(ctx, ident, reqs, updates)
}
