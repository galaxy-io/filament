package sqlserver

import (
	"reflect"
	"slices"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
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

func TestRewind(t *testing.T) {
	for _, tc := range []struct {
		in      string
		seconds int
		decl    string
		want    string
	}{
		{"2024-03-01T00:00:30.1234567", 60, "datetime2(7)", "2024-02-29T23:59:30.1234567"},
		{"2024-03-01T00:00:30.003", 1, "datetime", "2024-03-01T00:00:29.003"},
		{"2024-03-01T00:00:30Z", 30, "datetimeoffset(0)", "2024-03-01T00:00:00Z"},
		{"1753-01-01T00:00:00.000", 1, "datetime", ""},
		{"1900-01-01T00:00:59", 60, "smalldatetime", ""},
		{"0001-01-01T00:00:01.0000000", 1, "datetime2(7)", "0001-01-01T00:00:00.0000000"},
		{"0001-01-01T00:00:01.0000000", 2, "datetime2(7)", ""},
	} {
		got, inRange, err := rewind(tc.in, tc.seconds, tc.decl)
		if err != nil || got != tc.want || inRange != (tc.want != "") {
			t.Errorf("rewind(%s, %d, %s) = %q, %v, %v; want %q", tc.in, tc.seconds, tc.decl, got, inRange, err, tc.want)
		}
	}
	if _, _, err := rewind("b64:AAEC", 1, "timestamp"); err == nil {
		t.Error("rewound a binary cursor")
	}
}

func TestIncrementalKeyHoldsTheCursorOnce(t *testing.T) {
	ts := column{name: "updated_at", typeName: "datetime2", scale: 7, pkOrder: 1}
	id := column{name: "id", typeName: "int", pkOrder: 2}
	for _, tc := range []struct {
		name string
		pks  []column
		want []string
	}{
		{"cursor outside the key", []column{id}, []string{"updated_at", "id"}},
		{"cursor in the key", []column{ts, id}, []string{"updated_at", "id"}},
		{"cursor is the key", []column{ts}, []string{"updated_at"}},
	} {
		cols, types := incrementalKey(ts, tc.pks)
		if !slices.Equal(cols, tc.want) || len(types) != len(cols) {
			t.Errorf("%s: key %v %v, want %v", tc.name, cols, types, tc.want)
		}
	}
}

func TestCursorColumns(t *testing.T) {
	cols := []column{
		{name: "id", typeName: "int", pkOrder: 1},
		{name: "created", typeName: "datetime2", scale: 7},
		{name: "ModifiedDate", typeName: "datetime"},
		{name: "rv", typeName: "timestamp"},
		{name: "updated_at", typeName: "datetime2", scale: 7, nullable: true},
	}
	tbl := &table{cols: cols, pks: cols[:1]}
	for _, c := range cols {
		tbl.schema.Fields = append(tbl.schema.Fields, rowmodel.Field{Name: c.name})
	}
	got := cursorColumns(tbl, "")
	if r := recommendedNames(got); !slices.Equal(r, []string{"ModifiedDate"}) {
		t.Fatalf("recommended %v, want [ModifiedDate]", r)
	}
	if got[4].Eligible || got[4].Warning == "" {
		t.Error("nullable updated_at is eligible")
	}
	if !got[3].Eligible || got[3].SupportsLookback {
		t.Errorf("rowversion: eligible %v, lookback %v", got[3].Eligible, got[3].SupportsLookback)
	}

	// A configured cursor is the only recommendation, even next to a
	// conventionally named column.
	pickCols := []column{
		{name: "id", typeName: "int", pkOrder: 1},
		{name: "created", typeName: "datetime2", scale: 7},
		{name: "updated_at", typeName: "datetime2", scale: 7},
	}
	pick := &table{cols: pickCols, pks: pickCols[:1]}
	for _, c := range pickCols {
		pick.schema.Fields = append(pick.schema.Fields, rowmodel.Field{Name: c.name})
	}
	if r := recommendedNames(cursorColumns(pick, "created")); !slices.Equal(r, []string{"created"}) {
		t.Fatalf("recommended %v with explicit cursor, want [created]", r)
	}

	// Without a conventional name, the rowversion column is recommended.
	tbl.cols, tbl.schema.Fields = cols[:4], tbl.schema.Fields[:4]
	tbl.cols[2].name = "stamp"
	if r := recommendedNames(cursorColumns(tbl, "")); !slices.Equal(r, []string{"rv"}) {
		t.Fatalf("recommended %v, want [rv]", r)
	}
}

func recommendedNames(cols []filament.CursorColumn) []string {
	var out []string
	for _, c := range cols {
		if c.Recommended {
			out = append(out, c.Name)
		}
	}
	return out
}

func TestCursorPrefersTheExactColumnName(t *testing.T) {
	cols := []column{
		{name: "id", typeName: "int", pkOrder: 1},
		{name: "Updated_at", typeName: "datetime2", scale: 7},
		{name: "updated_at", typeName: "datetime2", scale: 7},
	}
	tbl := &table{name: "events", cols: cols, pks: cols[:1]}
	for _, c := range cols {
		tbl.schema.Fields = append(tbl.schema.Fields, rowmodel.Field{Name: c.name})
	}
	for _, configured := range []string{"updated_at", "Updated_at"} {
		s := &Source{cursorColumns: map[string]string{"events": configured}}
		c, err := s.incrementalCursor(tbl)
		if err != nil || c.name != configured {
			t.Fatalf("configured %q resolved to %q, %v", configured, c.name, err)
		}
		if recommended := recommendedNames(cursorColumns(tbl, configured)); !slices.Equal(recommended, []string{configured}) {
			t.Fatalf("configured %q recommends %v", configured, recommended)
		}
	}
	s := &Source{cursorColumns: map[string]string{"events": "UPDATED_AT"}}
	if _, err := s.incrementalCursor(tbl); err == nil {
		t.Fatal("an ambiguous case-insensitive name resolved")
	}
}

func TestClearedCursorFieldSelectsAfresh(t *testing.T) {
	s := &Source{cursorColumns: map[string]string{"events": "created", "orders": "placed_at"}}
	if err := s.configureCursors([]string{"events", "orders"}, map[string]filament.ResourceCursorConfig{"events": {}}); err != nil {
		t.Fatal(err)
	}
	if len(s.cursorColumns) != 0 {
		t.Fatalf("cleared cursor fields kept %v", s.cursorColumns)
	}
	if err := s.configureCursors([]string{"events"}, map[string]filament.ResourceCursorConfig{"events": {Field: "updated_at"}}); err != nil || s.cursorColumns["events"] != "updated_at" {
		t.Fatalf("configured field = %q, %v", s.cursorColumns["events"], err)
	}
}
