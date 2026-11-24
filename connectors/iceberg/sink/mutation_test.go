package iceberg

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/extensions"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// TestMutationsChangeExactlyTheirKeys checks table contents, not just counts,
// after an upsert and a merge, for single-column keys of each kind and for
// composite keys, under both update modes. Updated keys carry their last
// value, deleted keys are gone, new keys appear, and every other row is
// untouched. Composite tables repeat each id under several tenants, so a
// filter that matched each column separately would hit the trap rows.
func TestMutationsChangeExactlyTheirKeys(t *testing.T) {
	uuid := func(n int) string { return fmt.Sprintf("00000000-0000-0000-0000-%012d", n) }
	for _, tc := range []struct {
		name string
		keys []rowmodel.Field
		key  func(tenant, id int) []any
	}{
		{
			"int64",
			[]rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}},
			func(tenant, id int) []any { return []any{int64(tenant*100 + id)} },
		},
		{
			"string",
			[]rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalString}},
			func(tenant, id int) []any { return []any{fmt.Sprint(tenant*100 + id)} },
		},
		{
			"uuid",
			[]rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalUUID}},
			func(tenant, id int) []any { return []any{uuid(tenant*100 + id)} },
		},
		{"string-int64", []rowmodel.Field{
			{Name: "tenant", Logical: rowmodel.LogicalString}, {Name: "id", Logical: rowmodel.LogicalInt64},
		}, func(tenant, id int) []any { return []any{fmt.Sprintf("t%d", tenant), int64(id)} }},
		{"int64-uuid", []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt64}, {Name: "tenant", Logical: rowmodel.LogicalUUID},
		}, func(tenant, id int) []any { return []any{int64(id), uuid(tenant)} }},
	} {
		for _, mode := range updateModes {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				env := newTestEnv(t)
				env.updateMode(mode)
				fields := append(append([]rowmodel.Field(nil), tc.keys...), rowmodel.Field{Name: "value", Logical: rowmodel.LogicalString})
				env.schema = rowmodel.Schema{Fields: fields}
				for _, key := range tc.keys {
					env.schema.PrimaryKey = append(env.schema.PrimaryKey, key.Name)
				}
				policy := func(ingestion filament.IngestionType) filament.WritePolicy {
					p := eventsPolicy(ingestion)
					p.Keys = env.schema.PrimaryKey
					return p
				}
				name := func(tenant, id int) string { return fmt.Sprint(tc.key(tenant, id)...) }
				want := map[string]string{}

				var base []mutationRow
				for tenant := range 3 {
					for id := range 20 {
						base = append(base, mutationRow{tc.key(tenant, id), "base", rowmodel.OpInsert})
						want[name(tenant, id)] = "base"
					}
				}
				env.run(t, policy(filament.IngestionFullAppend), applyRows(t, env.schema, base, false))

				// Upsert: one key twice (last wins), one once, one new.
				upsert := []mutationRow{
					{tc.key(1, 5), "first", rowmodel.OpInsert},
					{tc.key(1, 5), "second", rowmodel.OpInsert},
					{tc.key(2, 6), "upserted", rowmodel.OpInsert},
					{tc.key(7, 5), "new", rowmodel.OpInsert},
				}
				env.run(t, policy(filament.IngestionIncrementalUpsert), applyRows(t, env.schema, upsert, false))
				want[name(1, 5)], want[name(2, 6)], want[name(7, 5)] = "second", "upserted", "new"
				assertTable(t, env, want)

				// Merge: delete one, update one, delete then reinsert one.
				merge := []mutationRow{
					{tc.key(0, 5), "", rowmodel.OpDelete},
					{tc.key(2, 7), "merged", rowmodel.OpUpdate},
					{tc.key(1, 9), "", rowmodel.OpDelete},
					{tc.key(1, 9), "back", rowmodel.OpInsert},
				}
				env.run(t, policy(filament.IngestionCDCMerge), applyRows(t, env.schema, merge, true))
				delete(want, name(0, 5))
				want[name(2, 7)], want[name(1, 9)] = "merged", "back"
				assertTable(t, env, want)
			})
		}
	}
}

type mutationRow struct {
	key   []any
	value string
	op    rowmodel.Operation
}

func applyRows(t *testing.T, schema rowmodel.Schema, rows []mutationRow, withOps bool) func(func(*arrowbatch.Batch)) {
	return func(apply func(*arrowbatch.Batch)) {
		builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
		defer builder.Release()
		ops := make([]rowmodel.Operation, 0, len(rows))
		for _, row := range rows {
			for c, value := range row.key {
				switch v := value.(type) {
				case int64:
					builder.Field(c).(*array.Int64Builder).Append(v)
				case string: // UUID keys travel as strings too
					builder.Field(c).(*array.StringBuilder).Append(v)
				default:
					t.Fatalf("unexpected key value %T", value)
				}
			}
			builder.Field(len(row.key)).(*array.StringBuilder).Append(row.value)
			ops = append(ops, row.op)
		}
		if !withOps {
			ops = nil
		}
		batch := arrowbatch.NewBatch(builder.NewRecordBatch(), arrowbatch.NewOperations(ops))
		batch.Resource = "events"
		defer batch.Release()
		apply(batch)
	}
}

// assertTable compares the events table, keyed by its primary key columns
// rendered as fmt.Sprint does, with want.
func assertTable(t *testing.T, env *testEnv, want map[string]string) {
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
	keys := len(env.schema.PrimaryKey)
	got := map[string]string{}
	for rec, err := range records {
		if err != nil {
			t.Fatal(err)
		}
		for i := range int(rec.NumRows()) {
			parts := make([]any, keys)
			for c := range keys {
				parts[c] = keyValue(rec.Column(c), i)
			}
			k := fmt.Sprint(parts...)
			if _, dup := got[k]; dup {
				t.Fatalf("key %s appears twice", k)
			}
			got[k] = rec.Column(keys).(*array.String).Value(i)
		}
		rec.Release()
	}
	if !maps.Equal(got, want) {
		for k, v := range want {
			if got[k] != v {
				t.Errorf("key %s = %q, want %q", k, got[k], v)
			}
		}
		for k := range got {
			if _, ok := want[k]; !ok {
				t.Errorf("unexpected key %s", k)
			}
		}
		t.FailNow()
	}
}

func keyValue(col arrow.Array, i int) any {
	switch c := col.(type) {
	case *array.Int64:
		return c.Value(i)
	case *extensions.UUIDArray:
		return c.Value(i).String()
	default:
		return col.ValueStr(i)
	}
}

// updateModes runs a mutation test against both ways the sink applies changes:
// equality deletes and the copy-on-write rewrite behind the key filter.
var updateModes = []string{table.WriteModeMergeOnRead, table.WriteModeCopyOnWrite}

func TestCreatePropertiesFollowUpdateMode(t *testing.T) {
	for mode, want := range map[string]string{
		"":                         table.WriteModeCopyOnWrite,
		table.WriteModeMergeOnRead: table.WriteModeMergeOnRead,
		table.WriteModeCopyOnWrite: table.WriteModeCopyOnWrite,
	} {
		props := createProperties(mode)
		if props[writeMergeModeKey] != want || props[table.WriteDeleteModeKey] != want {
			t.Fatalf("update mode %q: merge %q, delete %q, want %q", mode, props[writeMergeModeKey], props[table.WriteDeleteModeKey], want)
		}
	}
}
