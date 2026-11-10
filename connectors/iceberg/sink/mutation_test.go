package iceberg

import (
	"context"
	"maps"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestMutationHonorsWriteMode(t *testing.T) {
	for _, tc := range []struct {
		name      string
		props     iceberg.Properties
		eqDeletes string
	}{
		{"merge-on-read", iceberg.Properties{writeMergeModeKey: table.WriteModeMergeOnRead}, "1"},
		{"copy-on-write", iceberg.Properties{}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			schema := iceberg.NewSchema(0,
				iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
				iceberg.NestedField{ID: 2, Name: "v", Type: iceberg.PrimitiveTypes.String},
			)
			tbl := testTable(t, ctx, schema, tc.props)
			out, err := tbl.AppendTable(ctx, kvTable(t, []int64{1, 2, 3}, []string{"a", "b", "c"}), 8, nil)
			if err != nil {
				t.Fatal(err)
			}
			it := &iceTable{tbl: out, schema: out.Schema(), primaryKey: []string{"id"}}

			// id 1 is updated twice, id 5 is inserted then deleted in the same
			// epoch, so only each key's last row may land.
			merge(t, ctx, it,
				[]int64{1, 2, 4, 5, 1, 5},
				[]string{"x", "", "d", "e", "y", ""},
				[]rowmodel.Operation{rowmodel.OpUpdate, rowmodel.OpDelete, rowmodel.OpInsert, rowmodel.OpInsert, rowmodel.OpUpdate, rowmodel.OpDelete},
			)
			if got := it.tbl.CurrentSnapshot().Summary.Properties["added-equality-delete-files"]; got != tc.eqDeletes {
				t.Fatalf("equality delete files = %q, want %q", got, tc.eqDeletes)
			}
			if got, want := readKV(t, ctx, it.tbl), map[int64]string{1: "y", 3: "c", 4: "d"}; !maps.Equal(got, want) {
				t.Fatalf("rows = %v, want %v", got, want)
			}

			merge(t, ctx, it, []int64{3}, []string{""}, []rowmodel.Operation{rowmodel.OpDelete})
			if got, want := readKV(t, ctx, it.tbl), map[int64]string{1: "y", 4: "d"}; !maps.Equal(got, want) {
				t.Fatalf("rows after delete-only epoch = %v, want %v", got, want)
			}
		})
	}
}

func TestMaintenanceFoldsEqualityDeletes(t *testing.T) {
	ctx := context.Background()
	schema := iceberg.NewSchema(0,
		iceberg.NestedField{ID: 1, Name: "id", Type: iceberg.PrimitiveTypes.Int64, Required: true},
		iceberg.NestedField{ID: 2, Name: "v", Type: iceberg.PrimitiveTypes.String},
	)
	tbl := testTable(t, ctx, schema, iceberg.Properties{writeMergeModeKey: table.WriteModeMergeOnRead})
	it := &iceTable{tbl: tbl, schema: tbl.Schema(), primaryKey: []string{"id"}}
	for i := range maintenanceCommits {
		merge(t, ctx, it, []int64{1, int64(i + 2)}, []string{string(rune('a' + i)), "row"}, []rowmodel.Operation{rowmodel.OpUpdate, rowmodel.OpInsert})
	}
	if eqDeletes(t, ctx, it.tbl) == 0 {
		t.Fatal("merge-on-read epochs left no equality deletes")
	}

	if err := maintain(ctx, it); err != nil {
		t.Fatal(err)
	}
	if n := eqDeletes(t, ctx, it.tbl); n != 0 {
		t.Fatalf("equality deletes after compaction = %d, want 0", n)
	}
	if got := readKV(t, ctx, it.tbl)[1]; got != "j" {
		t.Fatalf("id 1 = %q after compaction, want the last update", got)
	}
}

func eqDeletes(t *testing.T, ctx context.Context, tbl *table.Table) int {
	t.Helper()
	tasks, err := tbl.Scan().PlanFiles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, task := range tasks {
		n += len(task.EqualityDeleteFiles)
	}
	return n
}

func merge(t *testing.T, ctx context.Context, it *iceTable, ids []int64, vs []string, ops []rowmodel.Operation) {
	t.Helper()
	rb := newRecordBuf(1 << 20)
	defer rb.close()
	if err := rb.setPolicy(filament.WritePolicy{Capability: filament.WritePolicyCapability{Mode: filament.WriteMerge}, Keys: []string{"id"}}); err != nil {
		t.Fatal(err)
	}
	rows := kvRecord(ids, vs)
	b := arrowbatch.NewBatch(rows, arrowbatch.NewOperations(ops))
	if err := rb.append(b, arrowbatch.Bytes(b.Rows())); err != nil {
		t.Fatal(err)
	}
	b.Release()
	arrowSchema, err := table.SchemaToArrowSchema(it.schema, nil, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := (&Sink{}).writeMutationBuffer(ctx, it, rb, writeModeMerge, arrowSchema); err != nil {
		t.Fatal(err)
	}
}

// kvRecord builds id/v rows; an empty v is null.
func kvRecord(ids []int64, vs []string) arrow.RecordBatch {
	sc := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "v", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
	idb := array.NewInt64Builder(memory.DefaultAllocator)
	vb := array.NewStringBuilder(memory.DefaultAllocator)
	defer idb.Release()
	defer vb.Release()
	for i, id := range ids {
		idb.Append(id)
		if vs[i] == "" {
			vb.AppendNull()
		} else {
			vb.Append(vs[i])
		}
	}
	idCol, vCol := idb.NewArray(), vb.NewArray()
	defer idCol.Release()
	defer vCol.Release()
	return array.NewRecordBatch(sc, []arrow.Array{idCol, vCol}, int64(len(ids)))
}

func kvTable(t *testing.T, ids []int64, vs []string) arrow.Table {
	t.Helper()
	rec := kvRecord(ids, vs)
	defer rec.Release()
	tbl := array.NewTableFromRecords(rec.Schema(), []arrow.RecordBatch{rec})
	t.Cleanup(tbl.Release)
	return tbl
}

func readKV(t *testing.T, ctx context.Context, tbl *table.Table) map[int64]string {
	t.Helper()
	at, err := tbl.Scan().ToArrowTable(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer at.Release()
	out := map[int64]string{}
	rdr := array.NewTableReader(at, 0)
	defer rdr.Release()
	for rdr.Next() {
		rec := rdr.RecordBatch()
		ids := rec.Column(0).(*array.Int64)
		vs := rec.Column(1).(*array.String)
		for i := range int(rec.NumRows()) {
			out[ids.Value(i)] = vs.Value(i)
		}
	}
	return out
}
