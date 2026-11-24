package iceberg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	icecatalog "github.com/apache/iceberg-go/catalog"
	icetable "github.com/apache/iceberg-go/table"
	_ "modernc.org/sqlite" // database/sql driver for the local test catalog

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

var testSchema = rowmodel.Schema{
	Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "note", Logical: rowmodel.LogicalString},
	},
	PrimaryKey: []string{"id"},
}

// testEnv drives the sink through its real lifecycle against a local
// iceberg-go SQL catalog (SQLite) and a file:// warehouse, so tests need no
// containers.
type testEnv struct {
	dir    string
	cfg    map[string]any
	schema rowmodel.Schema
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	dir, err := os.MkdirTemp(t.TempDir(), "iceberg")
	if err != nil {
		t.Fatal(err)
	}
	return &testEnv{dir: dir, schema: testSchema, cfg: map[string]any{
		"namespace": "test",
		"catalog": map[string]any{
			"provider": "generic", "type": "sql",
			// SQLite serializes writes; the busy timeout lets parallel table
			// commits wait for the lock instead of failing.
			"uri":       "file:" + filepath.Join(dir, "catalog.db") + "?_pragma=busy_timeout(10000)",
			"warehouse": "file://" + filepath.Join(dir, "warehouse"),
			"properties": map[string]any{
				"sql.driver": "sqlite", "sql.dialect": "sqlite", "init_catalog_tables": "true",
			},
		},
	}}
}

// updateMode sets table.update_mode for the tables the sink creates.
func (e *testEnv) updateMode(mode string) {
	e.cfg["table"] = map[string]any{"update_mode": mode}
}

// run drives one committed sink run into the events table.
func (e *testEnv) run(t *testing.T, policy filament.WritePolicy, write func(apply func(*arrowbatch.Batch))) {
	t.Helper()
	ctx := context.Background()
	sink := New()
	if err := sink.Open(ctx, filament.RunSpec{
		Run: "test", StartedAt: time.Now(), Resources: []string{"events"},
		Sink:          filament.Ref{Connector: "iceberg", Config: e.cfg},
		WritePolicies: map[string]filament.WritePolicy{"events": policy},
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sink.Close(ctx) }()
	if err := sink.EnsureSchema(ctx, "events", e.schema); err != nil {
		t.Fatal(err)
	}
	if err := sink.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	write(func(batch *arrowbatch.Batch) {
		if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
			t.Fatal(err)
		}
	})
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// table opens a sink on the env's catalog for reading committed tables.
func (e *testEnv) table(t *testing.T) *Sink {
	t.Helper()
	sink := New()
	if err := sink.open(context.Background(), filament.RunSpec{Run: "verify", Sink: filament.Ref{Config: e.cfg}}); err != nil {
		t.Fatal(err)
	}
	return sink
}

// tableRows scans a committed table and counts its rows.
func tableRows(t *testing.T, env *testEnv, resource string) int64 {
	t.Helper()
	ctx := context.Background()
	sink := env.table(t)
	tbl, err := sink.cat.LoadTable(ctx, sink.tableIdent(resource))
	if err != nil {
		t.Fatal(err)
	}
	_, records, err := tbl.Scan().ToArrowRecords(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var rows int64
	for rec, err := range records {
		if err != nil {
			t.Fatal(err)
		}
		rows += rec.NumRows()
		rec.Release()
	}
	return rows
}

// dataFiles reports the data files in the events table's current snapshot.
func (e *testEnv) dataFiles(t *testing.T) int {
	t.Helper()
	sink := e.table(t)
	tbl, err := sink.cat.LoadTable(context.Background(), sink.tableIdent("events"))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := tbl.CurrentSnapshot()
	if snapshot == nil || snapshot.Summary == nil {
		return 0
	}
	files, _ := strconv.Atoi(snapshot.Summary.Properties["total-data-files"])
	return files
}

func eventsPolicy(ingestion filament.IngestionType) filament.WritePolicy {
	policy := filament.WritePolicyForIngestion(ingestion)
	policy.Resource = "events"
	policy.Keys = []string{"id"}
	return policy
}

// idBatch builds events rows with ids start..start+count-1.
func idBatch(t *testing.T, start, count int) *arrowbatch.Batch {
	t.Helper()
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(testSchema))
	defer builder.Release()
	for id := start; id < start+count; id++ {
		builder.Field(0).(*array.Int64Builder).Append(int64(id))
		builder.Field(1).(*array.StringBuilder).Append("note-" + strconv.Itoa(id))
	}
	batch := arrowbatch.NewBatch(builder.NewRecordBatch(), arrowbatch.NewOperations(nil))
	batch.Resource = "events"
	return batch
}

// failingCatalog fails the catalog commit of one table after a short delay,
// so parallel commits are still in flight when it fails.
type failingCatalog struct {
	icecatalog.Catalog
	table string
}

// LoadTable binds the table to this catalog, since a table commits through
// the catalog that loaded it.
func (c *failingCatalog) LoadTable(ctx context.Context, ident icetable.Identifier) (*icetable.Table, error) {
	tbl, err := c.Catalog.LoadTable(ctx, ident)
	if err != nil {
		return nil, err
	}
	return icetable.New(tbl.Identifier(), tbl.Metadata(), tbl.MetadataLocation(), tbl.FS, c), nil
}

func (c *failingCatalog) CommitTable(ctx context.Context, ident icetable.Identifier, reqs []icetable.Requirement, updates []icetable.Update) (icetable.Metadata, string, error) {
	time.Sleep(5 * time.Millisecond)
	if ident[len(ident)-1] == c.table {
		return nil, "", errInjectedCommit
	}
	return c.Catalog.CommitTable(ctx, ident, reqs, updates)
}

var errInjectedCommit = errors.New("injected commit failure")
