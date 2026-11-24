//go:build integration

package sqlserver_test

import (
	"context"
	"encoding/hex"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	_ "github.com/galaxy-io/filament/connectors/sqlserver" // registers "sqlserver"
	source "github.com/galaxy-io/filament/connectors/sqlserver/source"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	"github.com/galaxy-io/filament/tests/testcontainers"
)

func TestSQLServerSource(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	ms := testcontainers.MSSQLContainer(t)
	exec := func(t *testing.T, stmts ...string) {
		t.Helper()
		for _, stmt := range stmts {
			if _, err := ms.DB.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
	}
	configure := func(t *testing.T, extra map[string]any) *source.Source {
		t.Helper()
		cfg := map[string]any{"connection_method": "url", "dsn": ms.DSN}
		for k, v := range extra {
			cfg[k] = v
		}
		src := source.New()
		if err := src.TestConnection(ctx, filament.NewConfig(cfg)); err != nil {
			t.Fatalf("test connection: %v", err)
		}
		if err := src.Configure(ctx, filament.NewConfig(cfg)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = src.Teardown(ctx) })
		return src
	}

	t.Run("every type", func(t *testing.T) {
		exec(t, `CREATE TABLE dbo.types (
			id int NOT NULL PRIMARY KEY,
			b bit, ti tinyint, si smallint, i int, bi bigint,
			r real, f float, d decimal(38,10), n numeric(9,4), m money, sm smallmoney,
			dt date, tm time(7), dtm datetime, sdt smalldatetime, dt2 datetime2(7), dto datetimeoffset(3),
			c char(5), vc varchar(20), vcm varchar(max), nc nchar(3), nvc nvarchar(20), nvcm nvarchar(max),
			txt text, ntxt ntext,
			bin binary(3), vb varbinary(10), vbm varbinary(max), img image,
			g uniqueidentifier, x xml, h hierarchyid, geo geography, sv sql_variant, rv rowversion)`,
			`INSERT INTO dbo.types (id, b, ti, si, i, bi, r, f, d, n, m, sm, dt, tm, dtm, sdt, dt2, dto,
				c, vc, vcm, nc, nvc, nvcm, txt, ntxt, bin, vb, vbm, img, g, x, h, geo, sv)
			VALUES (1, 1, 255, -32768, -2147483648, 9223372036854775807,
				1.5, -2.25, -1234567890123456789012345678.0123456789, 12345.6789, -922337203685477.5808, 214748.3647,
				'0001-01-01', '23:59:59.9999999', '1753-01-01T00:00:00.003', '2079-06-06T23:59:00',
				'9999-12-31T23:59:59.9999999', '2024-02-29T12:00:00.123+05:30',
				'ab', 'café', REPLICATE(CAST('x' AS varchar(max)), 9000), N'日本', N'😀 emoji',
				REPLICATE(CAST(N'ab' AS nvarchar(max)), 5000), 'legacy text', N'légacy',
				0x010203, 0xDEAD, CAST(REPLICATE(CAST('A' AS varchar(max)), 10000) AS varbinary(max)), 0x00FF,
				'6F9619FF-8B86-D011-B42D-00C04FC964FF', N'<a b="1"/>', hierarchyid::Parse('/1/2/'),
				geography::Point(47.65, -122.34, 4326), CAST(42 AS int))`,
			`INSERT INTO dbo.types (id) VALUES (2)`)

		src := configure(t, nil)
		schema, err := src.Schema(ctx, "types")
		if err != nil {
			t.Fatal(err)
		}
		logical := map[string]rowmodel.LogicalType{}
		for _, f := range schema.Fields {
			logical[f.Name] = f.Logical
		}
		for name, want := range map[string]rowmodel.LogicalType{
			"b": rowmodel.LogicalBool, "ti": rowmodel.LogicalInt16, "bi": rowmodel.LogicalInt64, "r": rowmodel.LogicalFloat32,
			"m": rowmodel.LogicalDecimal, "tm": rowmodel.LogicalTime, "dto": rowmodel.LogicalTimestampTZ,
			"g": rowmodel.LogicalUUID, "geo": rowmodel.LogicalBytes, "rv": rowmodel.LogicalBytes, "x": rowmodel.LogicalString,
		} {
			if logical[name] != want {
				t.Errorf("%s is %s, want %s", name, logical[name], want)
			}
		}

		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.Extract(ctx, out, filament.ExtractOpts{Resources: []string{"types"}}); err != nil {
			t.Fatal(err)
		}
		rows := renderRows(t, out.BatchesFor("types"))
		if len(rows) != 2 {
			t.Fatalf("rows = %d, want 2", len(rows))
		}
		got := rows[0]
		want := map[string]string{
			"b": "true", "ti": "255", "si": "-32768", "i": "-2147483648", "bi": "9223372036854775807",
			"r": "1.5", "f": "-2.25", "d": "-1234567890123456789012345678.0123456789", "n": "12345.6789",
			"m": "-922337203685477.5808", "sm": "214748.3647",
			"dt": "0001-01-01", "tm": "23:59:59.999999", "dtm": "1753-01-01T00:00:00.003333Z",
			"sdt": "2079-06-06T23:59:00Z", "dt2": "9999-12-31T23:59:59.999999Z", "dto": "2024-02-29T06:30:00.123Z",
			"c": "ab   ", "vc": "café", "nc": "日本 ", "nvc": "😀 emoji", "txt": "legacy text", "ntxt": "légacy",
			"bin": "010203", "vb": "dead", "img": "00ff",
			"g": "6F9619FF-8B86-D011-B42D-00C04FC964FF", "x": `<a b="1"/>`, "h": "/1/2/", "sv": "42",
		}
		for name, w := range want {
			if got[name] != w {
				t.Errorf("%s = %q, want %q", name, got[name], w)
			}
		}
		if got["vcm"] != strings.Repeat("x", 9000) || got["nvcm"] != strings.Repeat("ab", 5000) {
			t.Errorf("(max) text lengths %d, %d", len(got["vcm"]), len(got["nvcm"]))
		}
		if got["vbm"] != strings.Repeat("41", 10000) {
			t.Errorf("varbinary(max) length %d", len(got["vbm"])/2)
		}
		// WKB point: little-endian, type 1, then longitude and latitude.
		if !strings.HasPrefix(got["geo"], "0101000000") || len(got["geo"]) != 42 {
			t.Errorf("geography WKB = %s", got["geo"])
		}
		if len(got["rv"]) != 16 {
			t.Errorf("rowversion = %s", got["rv"])
		}
		for name, v := range rows[1] {
			if name != "id" && name != "rv" && v != "null" {
				t.Errorf("row 2 %s = %q, want null", name, v)
			}
		}
	})

	exec(t, `CREATE TABLE dbo.orders (order_key int NOT NULL PRIMARY KEY, note nvarchar(20))`,
		`CREATE TABLE dbo.lines (order_key int NOT NULL, line_no int NOT NULL, note nvarchar(20), PRIMARY KEY (order_key, line_no))`,
		`WITH n AS (SELECT TOP (20000) ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS k FROM sys.all_objects a CROSS JOIN sys.all_objects b)
		INSERT INTO dbo.orders SELECT k, CONCAT(N'order ', k) FROM n`,
		`INSERT INTO dbo.lines SELECT o.order_key, l.v, CONCAT(N'line ', l.v) FROM dbo.orders o CROSS JOIN (VALUES (1), (2), (3)) l(v)`)
	const lines = 60000

	t.Run("sharded keyset has no gaps or duplicates", func(t *testing.T) {
		src := configure(t, map[string]any{"shard_rows": 10000, "max_conns": 4})
		plan, err := src.PlanResume(ctx, []string{"lines"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ks, ok := checkpoint.ParseKeyset(plan["lines"])
		if !ok || len(ks.Shards) < 4 || !slices.Equal(ks.Cols, []string{"order_key", "line_no"}) {
			t.Fatalf("plan = %#v", plan["lines"])
		}
		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.ExtractFrom(ctx, out, filament.ExtractOpts{Resources: []string{"lines"}, Parallelism: 4}, plan); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, r := range testutil.DataRecords(out.Records) {
			if seen[r.ID] {
				t.Fatalf("duplicate key %q", r.ID)
			}
			seen[r.ID] = true
			if want := strings.Split(r.ID, "\x1f"); !slices.Equal(r.Key, want) {
				t.Fatalf("row %q carries checkpoint key %v", r.ID, r.Key)
			}
		}
		if len(seen) != lines {
			t.Fatalf("distinct rows = %d, want %d", len(seen), lines)
		}
	})

	t.Run("a changed key type starts over", func(t *testing.T) {
		exec(t, `CREATE TABLE dbo.retyped (k int NOT NULL PRIMARY KEY)`, `INSERT INTO dbo.retyped VALUES (1), (2), (10)`)
		src := configure(t, map[string]any{"shard_rows": 0})
		plan, err := src.PlanResume(ctx, []string{"retyped"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ks, _ := checkpoint.ParseKeyset(plan["retyped"])
		ks.Shards[0].Key = []string{"2"}
		// As text, 10 sorts before 2, so resuming after "2" would skip it.
		exec(t, `DROP TABLE dbo.retyped`, `CREATE TABLE dbo.retyped (k varchar(10) NOT NULL PRIMARY KEY)`,
			`INSERT INTO dbo.retyped VALUES ('1'), ('2'), ('10')`)
		resumed, err := src.PlanResume(ctx, []string{"retyped"}, map[string]filament.Checkpoint{"retyped": ks.ToCheckpoint("retyped")})
		if err != nil {
			t.Fatal(err)
		}
		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.ExtractFrom(ctx, out, filament.ExtractOpts{Resources: []string{"retyped"}}, resumed); err != nil {
			t.Fatal(err)
		}
		if n := len(testutil.DataRecords(out.Records)); n != 3 {
			t.Fatalf("rows after the key type changed = %d, want all 3", n)
		}
	})

	t.Run("a text key resumes under its own collation", func(t *testing.T) {
		// The database default is Latin1, which cannot hold these letters.
		exec(t, `CREATE TABLE dbo.cyrillic (k varchar(10) COLLATE Cyrillic_General_CI_AS NOT NULL PRIMARY KEY)`,
			`INSERT INTO dbo.cyrillic VALUES (N'А'), (N'Б'), (N'В')`)
		src := configure(t, map[string]any{"shard_rows": 0})
		plan, err := src.PlanResume(ctx, []string{"cyrillic"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ks, _ := checkpoint.ParseKeyset(plan["cyrillic"])
		ks.Shards[0].Key = []string{"Б"}
		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.ExtractFrom(ctx, out, filament.ExtractOpts{Resources: []string{"cyrillic"}}, map[string]filament.Checkpoint{"cyrillic": ks.ToCheckpoint("cyrillic")}); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, r := range testutil.DataRecords(out.Records) {
			ids = append(ids, r.ID)
		}
		if !slices.Equal(ids, []string{"В"}) {
			t.Fatalf("resumed after Б = %q, want [В]", ids)
		}
	})

	t.Run("a sql_variant key is read whole", func(t *testing.T) {
		exec(t, `CREATE TABLE dbo.variant_keyed (k sql_variant NOT NULL PRIMARY KEY)`,
			// One row per statement: a multi-row VALUES converts every row to
			// a common type, which would make 1 and '1' collide.
			`INSERT INTO dbo.variant_keyed VALUES (CAST(1 AS int))`,
			`INSERT INTO dbo.variant_keyed VALUES (CAST('1' AS varchar(5)))`,
			`INSERT INTO dbo.variant_keyed VALUES (CAST(2 AS int))`)
		src := configure(t, map[string]any{"shard_rows": 0})
		plan, err := src.PlanResume(ctx, []string{"variant_keyed"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if plan["variant_keyed"] != nil {
			t.Fatalf("plan = %v, want none for a sql_variant key", plan["variant_keyed"])
		}
		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.ExtractFrom(ctx, out, filament.ExtractOpts{Resources: []string{"variant_keyed"}}, plan); err != nil {
			t.Fatal(err)
		}
		if n := len(testutil.DataRecords(out.Records)); n != 3 {
			t.Fatalf("rows = %d, want 3", n)
		}
	})

	t.Run("a hierarchyid key splits and resumes", func(t *testing.T) {
		exec(t, `CREATE TABLE dbo.tree (node hierarchyid NOT NULL PRIMARY KEY, n int NOT NULL)`,
			`WITH n AS (SELECT TOP (300) ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS k FROM sys.all_objects)
			INSERT INTO dbo.tree SELECT hierarchyid::Parse(CONCAT('/', k, '/')), k FROM n`)
		src := configure(t, map[string]any{"shard_rows": 50, "max_conns": 4})
		plan, err := src.PlanResume(ctx, []string{"tree"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ks, _ := checkpoint.ParseKeyset(plan["tree"])
		if len(ks.Shards) < 2 {
			t.Fatalf("plan = %#v, want sampled shards", ks)
		}
		ks.Shards[0].Key = []string{"/2/"}
		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.ExtractFrom(ctx, out, filament.ExtractOpts{Resources: []string{"tree"}, Parallelism: 4}, map[string]filament.Checkpoint{"tree": ks.ToCheckpoint("tree")}); err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		for _, r := range testutil.DataRecords(out.Records) {
			if seen[r.ID] {
				t.Fatalf("duplicate key %q", r.ID)
			}
			seen[r.ID] = true
		}
		// The first shard resumes after /2/, so /1/ and /2/ are not read again.
		if len(seen) != 298 || seen["/1/"] || seen["/2/"] {
			t.Fatalf("read %d nodes, want the 298 after /2/", len(seen))
		}
	})

	t.Run("discover", func(t *testing.T) {
		src := configure(t, nil)
		res, err := src.Discover(ctx, filament.DiscoverOpts{})
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, r := range res.Resources {
			if r.Name == "lines" {
				found = true
				if !slices.Equal(r.PrimaryKey, []string{"order_key", "line_no"}) || r.Estimated != lines {
					t.Fatalf("lines = %+v", r)
				}
			}
		}
		if !found {
			t.Fatalf("lines not discovered in %+v", res.Resources)
		}
	})
}

// renderRows renders each row as column name → value text: timestamps in
// RFC 3339, dates as YYYY-MM-DD, times as HH:MM:SS.ffffff, binary as hex and
// decimals at their scale.
func renderRows(t *testing.T, batches []*arrowbatch.Batch) []map[string]string {
	t.Helper()
	var out []map[string]string
	for _, b := range batches {
		rec := b.Rows()
		for i := range int(rec.NumRows()) {
			row := map[string]string{}
			for c := range int(rec.NumCols()) {
				row[rec.ColumnName(c)] = renderValue(rec.Column(c), i)
			}
			out = append(out, row)
		}
	}
	return out
}

func renderValue(col arrow.Array, i int) string {
	if col.IsNull(i) {
		return "null"
	}
	switch a := col.(type) {
	case *array.Timestamp:
		unit := a.DataType().(*arrow.TimestampType).Unit
		return a.Value(i).ToTime(unit).UTC().Format(time.RFC3339Nano)
	case *array.Date32:
		return a.Value(i).ToTime().Format("2006-01-02")
	case *array.Time64:
		unit := a.DataType().(*arrow.Time64Type).Unit
		return a.Value(i).ToTime(unit).Format("15:04:05.999999")
	case *array.Binary:
		return hex.EncodeToString(a.Value(i))
	case *array.Decimal128:
		return a.Value(i).ToString(a.DataType().(*arrow.Decimal128Type).Scale)
	default:
		return col.ValueStr(i)
	}
}
