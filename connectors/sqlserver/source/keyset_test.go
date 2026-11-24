package sqlserver

import (
	"reflect"
	"testing"

	"github.com/galaxy-io/filament/checkpoint"
)

func TestKeysetWhere(t *testing.T) {
	keys := []keyColumn{{"order_id", "int"}, {"line", "smallint"}}
	tbl := &table{name: "lines"}
	cases := []struct {
		name  string
		sh    keyShard
		where string
		args  []any
	}{
		{"open shard", keyShard{t: tbl, keys: keys}, "", nil},
		{
			"bounded shard",
			keyShard{t: tbl, keys: keys, lo: []string{"10"}, hi: []string{"20"}},
			" WHERE t.[order_id] >= CAST(@p1 AS int) AND t.[order_id] < CAST(@p2 AS int)",
			[]any{int64(10), int64(20)},
		},
		{
			"resumed shard",
			keyShard{t: tbl, keys: keys, lo: []string{"10"}, hi: []string{"20"}, seed: []string{"12", "3"}},
			" WHERE (t.[order_id] > CAST(@p1 AS int) OR (t.[order_id] = CAST(@p1 AS int) AND t.[line] > CAST(@p2 AS smallint))) AND t.[order_id] < CAST(@p3 AS int)",
			[]any{int64(12), int64(3), int64(20)},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			where, args := keysetWhere(tc.sh)
			if where != tc.where || !reflect.DeepEqual(args, tc.args) {
				t.Fatalf("got %q %v\nwant %q %v", where, args, tc.where, tc.args)
			}
		})
	}
}

func TestBinaryKeysBindAsBytes(t *testing.T) {
	var args []any
	got := tupleCmp([]keyColumn{{"rv", "timestamp"}}, ">", []string{"b64:AAEC"}, &args)
	if got != "t.[rv] > @p1" || !reflect.DeepEqual(args, []any{[]byte{0, 1, 2}}) {
		t.Fatalf("got %q %v", got, args)
	}
	args = nil
	got = tupleCmp([]keyColumn{{"code", "nvarchar(10)"}}, "<", []string{"x]y"}, &args)
	if got != "t.[code] < CAST(@p1 AS nvarchar(10))" || !reflect.DeepEqual(args, []any{"x]y"}) {
		t.Fatalf("got %q %v", got, args)
	}
}

func TestIntBoundaries(t *testing.T) {
	if got := intBoundaries(1, 100, 4); !reflect.DeepEqual(got, []string{"26", "51", "76"}) {
		t.Errorf("1..100 by 4 = %v", got)
	}
	if got := intBoundaries(-9223372036854775808, 9223372036854775807, 2); !reflect.DeepEqual(got, []string{"0"}) {
		t.Errorf("full int64 range = %v", got)
	}
	if got := intBoundaries(1, 2, 8); !reflect.DeepEqual(got, []string{"2"}) {
		t.Errorf("narrow range repeats a boundary: %v", got)
	}
	shards := splitFromBoundaries([]string{"5", "9"})
	want := []checkpoint.KeysetShard{{Hi: []string{"5"}}, {Lo: []string{"5"}, Hi: []string{"9"}}, {Lo: []string{"9"}}}
	if !reflect.DeepEqual(shards, want) {
		t.Errorf("shards = %+v", shards)
	}
}

func TestTextKeysCastUnderTheirCollation(t *testing.T) {
	code := column{name: "code", typeName: "varchar", maxLength: 10, collation: "Cyrillic_General_CI_AS", pkOrder: 1}
	wide := column{name: "label", typeName: "nvarchar", maxLength: 20, collation: "Cyrillic_General_CI_AS", pkOrder: 2}
	decls := columnDecls([]column{code, wide})
	if !reflect.DeepEqual(decls, []string{"varchar(10) COLLATE Cyrillic_General_CI_AS", "nvarchar(10) COLLATE Cyrillic_General_CI_AS"}) {
		t.Fatalf("key decls = %q", decls)
	}
	if got := castParam(keyColumn{"code", decls[0]}, 1); got != "CAST(@p1 COLLATE Cyrillic_General_CI_AS AS varchar(10))" {
		t.Fatalf("varchar cast = %s", got)
	}
	if got := castParam(keyColumn{"label", decls[1]}, 2); got != "CAST(@p2 COLLATE Cyrillic_General_CI_AS AS nvarchar(10))" {
		t.Fatalf("nvarchar cast = %s", got)
	}
}

func TestOrderedKey(t *testing.T) {
	for _, tc := range []struct {
		name string
		pks  []column
		want bool
	}{
		{"int key", []column{{name: "id", typeName: "int"}}, true},
		{"no key", nil, false},
		{"sql_variant in the key", []column{{name: "id", typeName: "int"}, {name: "v", typeName: "sql_variant"}}, false},
	} {
		if got := (&table{pks: tc.pks}).orderedKey(); got != tc.want {
			t.Errorf("%s: orderedKey = %v, want %v", tc.name, got, tc.want)
		}
	}
}
