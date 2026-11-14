package iceberg

import (
	"context"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/catalog"
	sqlcat "github.com/apache/iceberg-go/catalog/sql"
	"github.com/apache/iceberg-go/table"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestMaintenanceCompactsExpiresAndStamps(t *testing.T) {
	ctx := context.Background()
	schema := iceberg.NewSchema(0, iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true})
	tbl := testTable(t, ctx, schema, iceberg.Properties{
		maxSnapshotAgeKey:     "0",
		minSnapshotsToKeepKey: "1",
	})
	if maintenanceDue(tbl, time.Now()) {
		t.Fatal("empty table is due")
	}
	for i := range maintenanceCommits {
		tbl = appendRow(t, ctx, tbl, int64(i))
	}
	if !maintenanceDue(tbl, time.Now()) {
		t.Fatal("not due after the commit threshold")
	}

	it := &iceTable{tbl: tbl}
	if err := maintain(ctx, it); err != nil {
		t.Fatal(err)
	}
	if files := dataFiles(t, ctx, it.tbl); files != 1 {
		t.Fatalf("data files after compaction = %d, want 1", files)
	}
	if n := len(it.tbl.Metadata().Snapshots()); n != 1 {
		t.Fatalf("snapshots after expiry = %d, want 1", n)
	}
	if maintenanceDue(it.tbl, time.Now()) {
		t.Fatal("due right after maintenance")
	}

	tbl = appendRow(t, ctx, it.tbl, 100)
	if maintenanceDue(tbl, time.Now()) {
		t.Fatal("due after one commit inside the interval")
	}
	if !maintenanceDue(tbl, time.Now().Add(maintenanceInterval)) {
		t.Fatal("not due once the interval passed")
	}
}

func testTable(t *testing.T, ctx context.Context, schema *iceberg.Schema, props iceberg.Properties) *table.Table {
	t.Helper()
	cat, err := catalog.Load(ctx, "test", iceberg.Properties{
		"type":            "sql",
		"uri":             ":memory:",
		sqlcat.DriverKey:  sqliteshim.ShimName,
		sqlcat.DialectKey: string(sqlcat.SQLite),
		"warehouse":       "file://" + t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cat.CreateNamespace(ctx, catalog.ToIdentifier("ns"), nil); err != nil {
		t.Fatal(err)
	}
	tbl, err := cat.CreateTable(ctx, catalog.ToIdentifier("ns", "t"), schema, catalog.WithProperties(props))
	if err != nil {
		t.Fatal(err)
	}
	return tbl
}

func appendRow(t *testing.T, ctx context.Context, tbl *table.Table, id int64) *table.Table {
	t.Helper()
	sc := arrow.NewSchema([]arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int64}}, nil)
	b := array.NewInt64Builder(memory.DefaultAllocator)
	b.Append(id)
	col := b.NewArray()
	rec := array.NewRecordBatch(sc, []arrow.Array{col}, 1)
	col.Release()
	b.Release()
	at := array.NewTableFromRecords(sc, []arrow.RecordBatch{rec})
	rec.Release()
	defer at.Release()
	out, err := tbl.AppendTable(ctx, at, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func dataFiles(t *testing.T, ctx context.Context, tbl *table.Table) int {
	t.Helper()
	tasks, err := tbl.Scan().PlanFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	return len(tasks)
}
