package motherduck

import (
	"context"
	"database/sql"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	duckdb "github.com/marcboeker/go-duckdb/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

func TestSpecAdvertisesInitialWritePolicies(t *testing.T) {
	wantModes := []filament.WriteMode{
		filament.WriteReplace,
		filament.WriteAppend,
		filament.WriteUpsert,
		filament.WriteUpsert,
		filament.WriteAppend,
	}
	spec := New().Spec()
	if spec.Name != "motherduck" || spec.DisplayName != "MotherDuck" {
		t.Fatalf("unexpected identity: %+v", spec)
	}
	if !spec.Capabilities.Schematized {
		t.Fatalf("unexpected capabilities: %+v", spec.Capabilities)
	}
	if spec.Capabilities.PreferredBatchRows != 100_000 || spec.Capabilities.PreferredBatchBytes != 64<<20 {
		t.Fatalf("unexpected preferred batch: %+v", spec.Capabilities)
	}
	if len(spec.Capabilities.WritePolicies) != len(wantModes) {
		t.Fatalf("write policies = %d, want %d", len(spec.Capabilities.WritePolicies), len(wantModes))
	}
	for i, policy := range spec.Capabilities.WritePolicies {
		if policy.Mode != wantModes[i] {
			t.Fatalf("policy %d mode = %q, want %q", i, policy.Mode, wantModes[i])
		}
	}
	if spec.SchemaField != "schema" {
		t.Fatalf("schema field = %q", spec.SchemaField)
	}
	fields := make(map[string]filament.ConfigField, len(spec.Config.Fields))
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	for _, name := range []string{"token", "database", "schema"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("missing config field %q", name)
		}
	}
	if fields["schema"].Default != defaultSchema || fields["schema"].Scope != filament.ScopePipeline {
		t.Errorf("unexpected schema field: %+v", fields["schema"])
	}
	if fields["token"].Type != filament.FieldSecret || !fields["token"].Required || !fields["database"].Required || fields["database"].Scope != filament.ScopeConnection {
		t.Errorf("unexpected connection fields: %+v", fields)
	}
}

func TestColumnType(t *testing.T) {
	tests := []struct {
		name  string
		field filament.SchemaField
		want  string
	}{
		{name: "bool", field: filament.SchemaField{Logical: filament.LogicalBool}, want: "BOOLEAN"},
		{name: "int16", field: filament.SchemaField{Logical: filament.LogicalInt16}, want: "SMALLINT"},
		{name: "int32", field: filament.SchemaField{Logical: filament.LogicalInt32}, want: "INTEGER"},
		{name: "int64", field: filament.SchemaField{Logical: filament.LogicalInt64}, want: "BIGINT"},
		{name: "float32", field: filament.SchemaField{Logical: filament.LogicalFloat32}, want: "FLOAT"},
		{name: "float64", field: filament.SchemaField{Logical: filament.LogicalFloat64}, want: "DOUBLE"},
		{name: "decimal", field: filament.SchemaField{Logical: filament.LogicalDecimal, Precision: 18, Scale: 3}, want: "DECIMAL(18, 3)"},
		{name: "unbounded decimal", field: filament.SchemaField{Logical: filament.LogicalDecimal}, want: "VARCHAR"},
		{name: "bytes", field: filament.SchemaField{Logical: filament.LogicalBytes}, want: "BLOB"},
		{name: "date", field: filament.SchemaField{Logical: filament.LogicalDate}, want: "DATE"},
		{name: "time", field: filament.SchemaField{Logical: filament.LogicalTime}, want: "TIME"},
		{name: "timestamp", field: filament.SchemaField{Logical: filament.LogicalTimestamp}, want: "TIMESTAMP"},
		{name: "timestamptz", field: filament.SchemaField{Logical: filament.LogicalTimestampTZ}, want: "TIMESTAMPTZ"},
		{name: "uuid", field: filament.SchemaField{Logical: filament.LogicalUUID}, want: "UUID"},
		{name: "json", field: filament.SchemaField{Logical: filament.LogicalJSON}, want: "VARCHAR"},
		{name: "array", field: filament.SchemaField{Logical: filament.LogicalArray}, want: "VARCHAR"},
		{name: "unknown", field: filament.SchemaField{Logical: filament.LogicalUnknown}, want: "VARCHAR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnType(tt.field); got != tt.want {
				t.Fatalf("columnType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateTableDDLForAppend(t *testing.T) {
	schema := testSchema()
	ddl, err := createTableDDL(qualified("", "main", "events"), schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		`CREATE TABLE IF NOT EXISTS "main"."events"`,
		`"id" BIGINT NOT NULL`,
		`"name" VARCHAR`,
	} {
		if !strings.Contains(ddl, fragment) {
			t.Fatalf("DDL missing %q:\n%s", fragment, ddl)
		}
	}
	if strings.Contains(ddl, "PRIMARY KEY") {
		t.Fatalf("append DDL unexpectedly has a primary key:\n%s", ddl)
	}
}

func TestCreateTableDDLForReplace(t *testing.T) {
	stage := stageTableName("events", "run-123")
	ddl, err := createTableDDL(qualified("", "main", stage), testSchema(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, `"main"."events__stage_run_123"`) {
		t.Fatalf("replace DDL does not use run stage:\n%s", ddl)
	}
}

func TestCreateTableDDLForUpsert(t *testing.T) {
	schema := testSchema()
	ddl, err := createTableDDL(qualified("warehouse", "main", "events"), schema, []string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, `PRIMARY KEY ("id")`) {
		t.Fatalf("upsert DDL lacks primary key:\n%s", ddl)
	}
	merge := upsertSQL(
		qualified("warehouse", "main", "events"),
		qualified("warehouse", "main", "events__stage_run"),
		[]string{"id", "name"}, []string{"id"},
	)
	for _, fragment := range []string{
		`INSERT INTO "warehouse"."main"."events" ("id", "name") SELECT "id", "name" FROM "warehouse"."main"."events__stage_run"`,
		`ON CONFLICT ("id") DO UPDATE SET "name" = excluded."name"`,
	} {
		if !strings.Contains(merge, fragment) {
			t.Fatalf("merge SQL missing %q:\n%s", fragment, merge)
		}
	}
}

func TestCreateTableDDLRejectsInvalidUpsertSchema(t *testing.T) {
	_, err := createTableDDL(`"main"."events"`, testSchema(), []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), "absent from schema") {
		t.Fatalf("error = %v", err)
	}
}

func TestAddColumnDDLOmitsUnsupportedConstraint(t *testing.T) {
	statements, err := addColumnDDLs(`"main"."events"`, testSchema())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(statements[0], "NOT NULL") {
		t.Fatalf("ADD COLUMN contains unsupported constraint: %s", statements[0])
	}
	if !strings.Contains(statements[0], `ADD COLUMN IF NOT EXISTS "id" BIGINT`) {
		t.Fatalf("unexpected ADD COLUMN: %s", statements[0])
	}
}

func TestQuoteIdent(t *testing.T) {
	if got, want := quoteIdent(`a"b`), `"a""b"`; got != want {
		t.Fatalf("quoteIdent() = %q, want %q", got, want)
	}
	if got, want := qualified("warehouse", "odd schema", `a"b`), `"warehouse"."odd schema"."a""b"`; got != want {
		t.Fatalf("qualified() = %q, want %q", got, want)
	}
}

func TestApplyRejectsPolicyDifferentFromOpenedMode(t *testing.T) {
	sink := New()
	sink.policies = map[string]filament.WritePolicy{
		"": {Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend}},
	}
	batch := arrowbatch.NewMarker()
	defer batch.Release()
	_, err := sink.Apply(context.Background(), batch, filament.ApplyOptions{
		Policy: filament.WritePolicy{Capability: filament.WritePolicyCapability{Mode: filament.WriteUpsert}},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v", err)
	}
}

func testSchema() filament.RecordSchema {
	return filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "name", Logical: filament.LogicalString, Nullable: true},
	}, PrimaryKey: []string{"id"}}
}

// Postgres infinity reaches the sink as the int32 and int64 sentinels.
// DuckDB dates share the convention, except that its -infinity is -int32
// max rather than int32 min, so only the negative sentinel shifts.
func TestInfinityDateValues(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer mem.AssertSize(t, 0)

	db := array.NewDate32Builder(mem)
	db.AppendValues([]arrow.Date32{math.MaxInt32, math.MinInt32, 19723}, nil) // 19723 = 2024-01-01
	col := db.NewArray()
	defer col.Release()

	fn := valueFor(arrow.Field{Name: "d", Type: arrow.FixedWidthTypes.Date32})
	want := []time.Time{
		time.Unix(int64(math.MaxInt32)*86400, 0).UTC(),  // DuckDB infinity
		time.Unix(-int64(math.MaxInt32)*86400, 0).UTC(), // DuckDB -infinity
		time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	for i, w := range want {
		got, ok := fn(col, i).(time.Time)
		if !ok || !got.Equal(w) {
			t.Errorf("row %d = %#v, want %v", i, fn(col, i), w)
		}
	}
}

// DuckDB cannot take timestamp infinity through the appender, which accepts
// time.Time only. Nullable fields keep NULL; NOT NULL fields clamp to the
// widest finite timestamps the driver accepts, so one infinity value cannot
// fail the whole batch at flush.
func TestInfinityTimestampValues(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer mem.AssertSize(t, 0)

	tb := array.NewTimestampBuilder(mem, &arrow.TimestampType{Unit: arrow.Microsecond})
	tb.AppendValues([]arrow.Timestamp{math.MaxInt64, math.MinInt64, 1704067200000000}, nil)
	col := tb.NewArray()
	defer col.Release()

	maxBound := time.Date(294246, 12, 31, 23, 59, 59, 999999000, time.UTC)
	minBound := time.Date(-290307, 1, 1, 0, 0, 0, 0, time.UTC)
	ordinary := time.Unix(1704067200, 0).UTC()

	notNull := valueFor(arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}})
	for i, w := range []time.Time{maxBound, minBound, ordinary} {
		got, ok := notNull(col, i).(time.Time)
		if !ok || !got.Equal(w) {
			t.Errorf("not null row %d = %#v, want %v", i, notNull(col, i), w)
		}
	}
	nullable := valueFor(arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}, Nullable: true})
	for i := 0; i < 2; i++ {
		if got := nullable(col, i); got != nil {
			t.Errorf("nullable row %d = %#v, want nil", i, got)
		}
	}
	if got, ok := nullable(col, 2).(time.Time); !ok || !got.Equal(ordinary) {
		t.Errorf("nullable row 2 = %#v, want %v", nullable(col, 2), ordinary)
	}
}

// The whole batch must land: infinity dates through the sink's own
// converters and appender, into the NOT NULL column shape the sink creates
// for required fields, and into a nullable column as infinity, not NULL.
func TestInfinityDatesLandInDuckDB(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer mem.AssertSize(t, 0)

	db := array.NewDate32Builder(mem)
	db.AppendValues([]arrow.Date32{math.MaxInt32, math.MinInt32, 19723}, nil)
	col := db.NewArray()
	defer col.Release()
	fn := valueFor(arrow.Field{Name: "d", Type: arrow.FixedWidthTypes.Date32})

	conn, ctx := duckDBTestConn(t)
	for _, table := range []string{"dates_not_null", "dates_nullable"} {
		ddl := "CREATE TABLE " + table + " (d DATE NOT NULL)"
		if table == "dates_nullable" {
			ddl = "CREATE TABLE " + table + " (d DATE)"
		}
		if _, err := conn.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create %s: %v", table, err)
		}
		app := duckDBTestAppender(t, conn, table)
		for i := range 3 {
			if err := app.AppendRow(fn(col, i)); err != nil {
				t.Fatalf("%s append row %d: %v", table, i, err)
			}
		}
		if err := app.Flush(); err != nil {
			t.Fatalf("%s flush: %v", table, err)
		}
		if err := app.Close(); err != nil {
			t.Fatalf("%s close appender: %v", table, err)
		}
		rows, err := conn.QueryContext(ctx, "SELECT d::VARCHAR, isinf(d) FROM "+table+" ORDER BY d")
		if err != nil {
			t.Fatalf("%s select: %v", table, err)
		}
		var got []string
		for rows.Next() {
			var s string
			var inf bool
			if err := rows.Scan(&s, &inf); err != nil {
				t.Fatalf("%s scan: %v", table, err)
			}
			got = append(got, s)
			if !inf && s == "2024-01-01" {
				continue
			}
			if !inf {
				t.Errorf("%s stored %q, want an infinity date", table, s)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			t.Fatalf("%s rows: %v", table, err)
		}
		want := []string{"-infinity", "2024-01-01", "infinity"}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s stored %v, want %v", table, got, want)
		}
	}
}

// The clamped timestamps must land in a NOT NULL column alongside ordinary
// rows, with their exact microsecond values intact.
func TestInfinityTimestampsLandInDuckDB(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer mem.AssertSize(t, 0)

	tb := array.NewTimestampBuilder(mem, &arrow.TimestampType{Unit: arrow.Microsecond})
	tb.AppendValues([]arrow.Timestamp{math.MaxInt64, math.MinInt64, 1704067200000000}, nil)
	col := tb.NewArray()
	defer col.Release()
	fn := valueFor(arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}})

	conn, ctx := duckDBTestConn(t)
	if _, err := conn.ExecContext(ctx, "CREATE TABLE stamps (ts TIMESTAMP NOT NULL)"); err != nil {
		t.Fatalf("create: %v", err)
	}
	app := duckDBTestAppender(t, conn, "stamps")
	for i := range 3 {
		if err := app.AppendRow(fn(col, i)); err != nil {
			t.Fatalf("append row %d: %v", i, err)
		}
	}
	if err := app.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if err := app.Close(); err != nil {
		t.Fatalf("close appender: %v", err)
	}
	rows, err := conn.QueryContext(ctx, "SELECT ts FROM stamps ORDER BY ts")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	defer rows.Close()
	wantMicros := []int64{-9223371158400000000, 1704067200000000, 9223371244799999999}
	var gotMicros []int64
	for rows.Next() {
		var v time.Time
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan: %v", err)
		}
		gotMicros = append(gotMicros, v.UnixMicro())
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(gotMicros) != len(wantMicros) {
		t.Fatalf("stored %d rows, want %d", len(gotMicros), len(wantMicros))
	}
	for i := range wantMicros {
		if gotMicros[i] != wantMicros[i] {
			t.Errorf("row %d micros = %d, want %d", i, gotMicros[i], wantMicros[i])
		}
	}
}

func duckDBTestConn(t *testing.T) (*sql.Conn, context.Context) {
	t.Helper()
	sqlDB, err := sql.Open("duckdb", "")
	if err != nil {
		t.Fatalf("open duckdb: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })
	ctx := context.Background()
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		t.Fatalf("conn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn, ctx
}

func duckDBTestAppender(t *testing.T, conn *sql.Conn, table string) *duckdb.Appender {
	t.Helper()
	var app *duckdb.Appender
	err := conn.Raw(func(dc any) error {
		var e error
		app, e = duckdb.NewAppender(dc.(*duckdb.Conn), "", "main", table)
		return e
	})
	if err != nil {
		t.Fatalf("appender %s: %v", table, err)
	}
	return app
}
