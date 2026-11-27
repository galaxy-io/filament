package iceberg

import (
	"context"
	"fmt"
	"maps"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// The two composite keys used below collided under the old keyOf encoding,
// which joined key columns with 0x1f: a string key can itself contain
// 0x1f, so ("a\u001f","b") and ("a","\u001fb") rendered as the same folded
// key and one row silently ate the other.

func TestKeyOfSeparatesCollidingCompositeKeys(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "a", Type: arrow.BinaryTypes.String},
		{Name: "b", Type: arrow.BinaryTypes.String},
	}, nil)
	b := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer b.Release()
	b.Field(0).(*array.StringBuilder).AppendValues([]string{"a\u001f", "a", "ab", "a", "2:ab", "2"}, nil)
	b.Field(1).(*array.StringBuilder).AppendValues([]string{"b", "\u001fb", "c", "bc", "c", "ab:c"}, nil)
	rec := b.NewRecordBatch()
	defer rec.Release()

	colliding := keyOf(rec, []int{0, 1}, 0)
	if other := keyOf(rec, []int{0, 1}, 1); colliding == other {
		t.Fatalf("distinct keys (\"a\\u001f\",\"b\") and (\"a\",\"\\u001fb\") both fold to %q", colliding)
	}
	if keyOf(rec, []int{0, 1}, 2) == keyOf(rec, []int{0, 1}, 3) {
		t.Fatal("control keys (\"ab\",\"c\") and (\"a\",\"bc\") fold together")
	}
	if keyOf(rec, []int{0, 1}, 4) == keyOf(rec, []int{0, 1}, 5) {
		t.Fatal("keys (\"2:ab\",\"c\") and (\"2\",\"ab:c\") fold together")
	}
	if again := keyOf(rec, []int{0, 1}, 0); again != colliding {
		t.Fatalf("keyOf is not deterministic: %q then %q", colliding, again)
	}
}

func TestCollidingCompositeKeysSurviveFolding(t *testing.T) {
	k1 := []any{"a\u001f", "b"}
	k2 := []any{"a", "\u001fb"}
	for _, mode := range updateModes {
		t.Run(mode, func(t *testing.T) {
			env := newTestEnv(t)
			env.updateMode(mode)
			env.schema = rowmodel.Schema{
				Fields: []rowmodel.Field{
					{Name: "a", Logical: rowmodel.LogicalString},
					{Name: "b", Logical: rowmodel.LogicalString},
					{Name: "value", Logical: rowmodel.LogicalString},
				},
				PrimaryKey: []string{"a", "b"},
			}
			policy := func(ingestion filament.IngestionType) filament.WritePolicy {
				p := eventsPolicy(ingestion)
				p.Keys = env.schema.PrimaryKey
				return p
			}

			// Two colliding inserts in one buffer: both rows must land.
			env.run(t, policy(filament.IngestionIncrementalUpsert), applyRows(t, env.schema, []mutationRow{
				{k1, "first", rowmodel.OpInsert},
				{k2, "second", rowmodel.OpInsert},
			}, false))
			assertKeyRows(t, env, map[string]string{
				quotedKey(k1): "first",
				quotedKey(k2): "second",
			})
		})

		t.Run(mode+"/delete", func(t *testing.T) {
			env := newTestEnv(t)
			env.updateMode(mode)
			env.schema = rowmodel.Schema{
				Fields: []rowmodel.Field{
					{Name: "a", Logical: rowmodel.LogicalString},
					{Name: "b", Logical: rowmodel.LogicalString},
					{Name: "value", Logical: rowmodel.LogicalString},
				},
				PrimaryKey: []string{"a", "b"},
			}
			policy := func(ingestion filament.IngestionType) filament.WritePolicy {
				p := eventsPolicy(ingestion)
				p.Keys = env.schema.PrimaryKey
				return p
			}

			// An insert and a delete for the other colliding key in one
			// buffer: the delete must not swallow the insert.
			env.run(t, policy(filament.IngestionCDCMerge), applyRows(t, env.schema, []mutationRow{
				{k1, "first", rowmodel.OpInsert},
				{k2, "", rowmodel.OpDelete},
			}, true))
			assertKeyRows(t, env, map[string]string{
				quotedKey(k1): "first",
			})
		})
	}
}

// quotedKey renders a two-part string key unambiguously for comparisons;
// the table harness's own key rendering joins with no separator and would
// collide on exactly the keys under test.
func quotedKey(key []any) string {
	return fmt.Sprintf("%q|%q", key[0], key[1])
}

// assertKeyRows compares the events table with want, keyed by quotedKey.
func assertKeyRows(t *testing.T, env *testEnv, want map[string]string) {
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
	got := map[string]string{}
	for rec, err := range records {
		if err != nil {
			t.Fatal(err)
		}
		for i := range int(rec.NumRows()) {
			k := fmt.Sprintf("%q|%q", rec.Column(0).ValueStr(i), rec.Column(1).ValueStr(i))
			if _, dup := got[k]; dup {
				t.Fatalf("key %s appears twice", k)
			}
			got[k] = rec.Column(2).(*array.String).Value(i)
		}
		rec.Release()
	}
	if !maps.Equal(got, want) {
		t.Fatalf("table = %v, want %v", got, want)
	}
}
