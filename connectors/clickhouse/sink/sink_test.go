package clickhouse

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/column"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/shopspring/decimal"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestSpecAdvertisesInitialWritePolicies(t *testing.T) {
	spec := New().Spec()
	if spec.Name != "clickhouse" || !spec.Capabilities.Schematized {
		t.Fatalf("unexpected spec: %+v", spec)
	}
	if spec.Capabilities.PreferredBatchRows != 10_000 {
		t.Fatalf("preferred rows = %d", spec.Capabilities.PreferredBatchRows)
	}
	want := []filament.WriteMode{filament.WriteReplace, filament.WriteAppend, filament.WriteUpsert, filament.WriteUpsert, filament.WriteAppend}
	for i, policy := range spec.Capabilities.WritePolicies {
		if policy.Mode != want[i] {
			t.Fatalf("policy %d mode = %q, want %q", i, policy.Mode, want[i])
		}
	}
	if spec.Version != "2" {
		t.Fatalf("version = %q, want 2", spec.Version)
	}
	fields := make(map[string]filament.ConfigField, len(spec.Config.Fields))
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if got := fields["connection_method"].Default; got != "fields" {
		t.Fatalf("connection_method default = %v, want fields", got)
	}
	if fields["dsn"].Type != filament.FieldSecret || !fields["dsn"].Required {
		t.Fatalf("unexpected dsn field: %+v", fields["dsn"])
	}
	if !fields["host"].Required || fields["password"].Type != filament.FieldSecret || !fields["password"].Required {
		t.Fatalf("unexpected connection fields: %+v", fields)
	}
}

func TestClickHouseColumnType(t *testing.T) {
	tests := []struct {
		name  string
		field filament.SchemaField
		want  string
	}{
		{name: "bool", field: filament.SchemaField{Logical: filament.LogicalBool}, want: "Bool"},
		{name: "int64 nullable", field: filament.SchemaField{Logical: filament.LogicalInt64, Nullable: true}, want: "Nullable(Int64)"},
		{name: "decimal bounded", field: filament.SchemaField{Logical: filament.LogicalDecimal, Precision: 12, Scale: 2}, want: "Decimal(12, 2)"},
		{name: "decimal unbounded", field: filament.SchemaField{Logical: filament.LogicalDecimal, Native: "numeric"}, want: "Decimal(38, 9)"},
		{name: "timestamp", field: filament.SchemaField{Logical: filament.LogicalTimestamp}, want: "DateTime64(6)"},
		{name: "timestamptz", field: filament.SchemaField{Logical: filament.LogicalTimestampTZ}, want: "DateTime64(6, 'UTC')"},
		{name: "json text", field: filament.SchemaField{Logical: filament.LogicalJSON}, want: "String"},
		{name: "array text", field: filament.SchemaField{Logical: filament.LogicalArray}, want: "String"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnType(tt.field); got != tt.want {
				t.Fatalf("columnType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreateTableDDLForUpsert(t *testing.T) {
	schema := filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "tenant_id", Logical: filament.LogicalString},
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "name", Logical: filament.LogicalString, Nullable: true},
		},
		PrimaryKey: []string{"tenant_id", "id"},
	}
	ddl, columns, err := createTableDDL("analytics", "users", schema, true, filament.VersionPolicy{Strategy: filament.VersionInsertOrder})
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{
		"CREATE TABLE IF NOT EXISTS `analytics`.`users`",
		"`name` Nullable(String)",
		"ENGINE = ReplacingMergeTree()",
		"ORDER BY (`tenant_id`, `id`)",
	} {
		if !strings.Contains(ddl, fragment) {
			t.Fatalf("DDL missing %q:\n%s", fragment, ddl)
		}
	}
	if got := columns[len(columns)-1]; got != "`name`" {
		t.Fatalf("last insert column = %q", got)
	}
	if strings.Contains(ddl, "_filament_version") {
		t.Fatalf("insertion-order DDL contains removed version column:\n%s", ddl)
	}
}

func TestCreateTableDDLRejectsInvalidUpsertSchema(t *testing.T) {
	_, _, err := createTableDDL("db", "events", filament.RecordSchema{
		Fields: []filament.SchemaField{{Name: "id", Logical: filament.LogicalInt64}},
	}, true, filament.VersionPolicy{Strategy: filament.VersionInsertOrder})
	if err == nil || !strings.Contains(err.Error(), "requires a primary key") {
		t.Fatalf("error = %v", err)
	}
}

func TestCreateTableDDLUsesCursorVersion(t *testing.T) {
	schema := filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "updated_at", Logical: filament.LogicalTimestampTZ},
		},
		PrimaryKey: []string{"id"},
	}
	policy := filament.VersionPolicy{
		Strategy: filament.VersionCursor, Field: "updated_at",
		Logical: filament.LogicalTimestampTZ, Native: "timestamp with time zone",
	}
	ddl, columns, err := createTableDDL("analytics", "users", schema, true, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ddl, "ENGINE = ReplacingMergeTree(`updated_at`)") {
		t.Fatalf("cursor version missing from DDL:\n%s", ddl)
	}
	if strings.Contains(ddl, "_filament_version") {
		t.Fatalf("cursor-version DDL contains run version column:\n%s", ddl)
	}
	if got := columns[len(columns)-1]; got != "`updated_at`" {
		t.Fatalf("last insert column = %q, want cursor source column", got)
	}
}

func TestCreateTableDDLRejectsUnsafeCursorVersion(t *testing.T) {
	tests := []struct {
		name  string
		field filament.SchemaField
		want  string
	}{
		{name: "nullable", field: filament.SchemaField{Name: "updated_at", Logical: filament.LogicalTimestampTZ, Nullable: true}, want: "NOT NULL"},
		{name: "unsupported type", field: filament.SchemaField{Name: "version", Logical: filament.LogicalString}, want: "unsupported ClickHouse type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := createTableDDL("db", "events", filament.RecordSchema{
				Fields:     []filament.SchemaField{{Name: "id", Logical: filament.LogicalInt64}, tt.field},
				PrimaryKey: []string{"id"},
			}, true, filament.VersionPolicy{Strategy: filament.VersionCursor, Field: tt.field.Name})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

type collect struct{ chunks []*arrowbatch.Batch }

func (c *collect) Chunk(ch *arrowbatch.Batch) error { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(rowmodel.Meta, int) error { return nil }

func TestValuesPreserveTypes(t *testing.T) {
	rs := filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "name", Logical: filament.LogicalString},
		{Name: "active", Logical: filament.LogicalBool},
		{Name: "score", Logical: filament.LogicalFloat64},
		{Name: "amount", Logical: filament.LogicalDecimal, Precision: 10, Scale: 3},
		{Name: "big", Logical: filament.LogicalDecimal},
		{Name: "payload", Logical: filament.LogicalJSON},
		{Name: "bytes", Logical: filament.LogicalBytes},
		{Name: "at", Logical: filament.LogicalTimestamp},
		{Name: "tz", Logical: filament.LogicalTimestampTZ},
		{Name: "optional", Logical: filament.LogicalString, Nullable: true},
	}}
	schema := arrowbatch.Schema(rs)
	c := &collect{}
	b := arrowbatch.NewBuilder(schema, nil, arrowbatch.Options{MaxRows: 4}, c)
	b.Int64(9223372036854775806)
	b.String("Ada")
	b.Bool(true)
	b.Float64(math.NaN())
	b.Decimal(decimal128.FromI64(123450))
	b.String("123.450")
	b.String(`{"ok":true}`)
	b.Bytes([]byte("hi"))
	b.Timestamp(1704067200_000000)
	b.Timestamp(1704067200_000000)
	b.Null()
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	defer c.chunks[0].Release()
	rows := c.chunks[0].Rows()
	values := make([]any, rows.NumCols())
	for i, f := range schema.Fields() {
		if rows.Column(i).IsNull(0) {
			continue
		}
		values[i] = valueFor(f)(rows.Column(i), 0)
	}
	if got := values[0]; got != int64(9223372036854775806) {
		t.Fatalf("id = %#v", got)
	}
	if values[1] != "Ada" || values[2] != true || !math.IsNaN(values[3].(float64)) {
		t.Fatalf("scalar values = %#v", values[:4])
	}
	for _, i := range []int{4, 5} {
		if got := values[i].(decimal.Decimal); !got.Equal(decimal.RequireFromString("123.450")) {
			t.Fatalf("decimal %d = %s", i, got)
		}
	}
	if values[6] != `{"ok":true}` || values[7] != "hi" {
		t.Fatalf("structured values = %#v", values[6:8])
	}
	if values[8] != "2024-01-01 00:00:00" {
		t.Fatalf("naive timestamp = %#v, want wall-clock text", values[8])
	}
	if got := values[9].(time.Time); got.Unix() != 1704067200 || got.Location() != time.UTC {
		t.Fatalf("instant = %v", got)
	}
	if values[10] != nil {
		t.Fatalf("nullable value = %#v", values[10])
	}
}

func TestMatchesReplacingVersion(t *testing.T) {
	tests := []struct {
		engine string
		field  string
		want   bool
	}{
		{engine: "ReplacingMergeTree() ORDER BY id", want: true},
		{engine: "ReplacingMergeTree ORDER BY id", want: true},
		{engine: "ReplacingMergeTree(`updated_at`) ORDER BY id", field: "updated_at", want: true},
		{engine: "SharedReplacingMergeTree(`updated_at`) ORDER BY id", field: "updated_at", want: true},
		{engine: "ReplacingMergeTree(`updated_at`) ORDER BY id", want: false},
		{engine: "ReplacingMergeTree() ORDER BY id", field: "updated_at", want: false},
	}
	for _, tt := range tests {
		if got := matchesReplacingVersion(tt.engine, tt.field); got != tt.want {
			t.Errorf("matchesReplacingVersion(%q, %q) = %v, want %v", tt.engine, tt.field, got, tt.want)
		}
	}
}

func TestMatchesTableEngineSupportsClickHouseCloud(t *testing.T) {
	tests := []struct {
		got  string
		want string
		ok   bool
	}{
		{got: "MergeTree", want: mergeTreeEngine, ok: true},
		{got: "SharedMergeTree", want: mergeTreeEngine, ok: true},
		{got: "ReplacingMergeTree", want: replacingMergeTreeEngine, ok: true},
		{got: "SharedReplacingMergeTree", want: replacingMergeTreeEngine, ok: true},
		{got: "SharedMergeTree", want: replacingMergeTreeEngine, ok: false},
		{got: "ReplicatedMergeTree", want: mergeTreeEngine, ok: false},
	}
	for _, tt := range tests {
		if got := matchesTableEngine(tt.got, tt.want); got != tt.ok {
			t.Errorf("matchesTableEngine(%q, %q) = %v, want %v", tt.got, tt.want, got, tt.ok)
		}
	}
}

func TestApplyRejectsPolicyDifferentFromOpenedMode(t *testing.T) {
	sink := New()
	sink.policies = map[string]filament.WritePolicy{
		"": {Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend}},
	}
	b := arrowbatch.NewMarker()
	defer b.Release()
	_, err := sink.Apply(context.Background(), b, filament.ApplyOptions{
		Policy: filament.WritePolicy{Capability: filament.WritePolicyCapability{Mode: filament.WriteUpsert}},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v", err)
	}
}

func TestStageTableNameIsStableAndResourceScoped(t *testing.T) {
	a := stageTableName("run-a", "users")
	if a != stageTableName("run-a", "users") {
		t.Fatal("stage name is not stable")
	}
	if a == stageTableName("run-a", "orders") || !strings.HasPrefix(a, "__filament_stage_") {
		t.Fatalf("stage name = %q", a)
	}
}

func TestExistsTableQueryUsesClickHouseStatementSyntax(t *testing.T) {
	got := existsTableQuery(qualified("tpch", "nation"))
	want := "EXISTS TABLE `tpch`.`nation`"
	if got != want {
		t.Fatalf("exists query = %q, want %q", got, want)
	}
}

// Postgres infinity reaches the sink as the int32 and int64 sentinels. In a
// NOT NULL column a nil would be stored as 1970-01-01, so the sentinels clamp
// to the widest values the driver can write; in a nullable column they stay
// nil and land as NULL.
func TestInfinitySentinelValues(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.DefaultAllocator)
	defer mem.AssertSize(t, 0)

	db := array.NewDate32Builder(mem)
	db.AppendValues([]arrow.Date32{math.MaxInt32, math.MinInt32, 19723}, nil) // 19723 = 2024-01-01
	dates := db.NewArray()
	defer dates.Release()

	tb := array.NewTimestampBuilder(mem, &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"})
	tb.AppendValues([]arrow.Timestamp{math.MaxInt64, math.MinInt64, 1704067200000000}, nil)
	instants := tb.NewArray()
	defer instants.Release()

	nb := array.NewTimestampBuilder(mem, &arrow.TimestampType{Unit: arrow.Microsecond})
	nb.AppendValues([]arrow.Timestamp{math.MaxInt64, math.MinInt64, 1704067200000000}, nil)
	naive := nb.NewArray()
	defer naive.Release()

	maxDate := time.Date(2299, 12, 31, 0, 0, 0, 0, time.UTC)
	minDate := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	maxInstant := time.Date(2262, 4, 11, 23, 47, 16, 0, time.UTC)
	ordinaryDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	ordinaryInstant := time.Unix(1704067200, 0).UTC()

	tests := []struct {
		name  string
		field arrow.Field
		col   arrow.Array
		want  []any
	}{
		{
			name:  "date not null",
			field: arrow.Field{Name: "d", Type: arrow.FixedWidthTypes.Date32},
			col:   dates,
			want:  []any{maxDate, minDate, ordinaryDate},
		},
		{
			name:  "date nullable",
			field: arrow.Field{Name: "d", Type: arrow.FixedWidthTypes.Date32, Nullable: true},
			col:   dates,
			want:  []any{nil, nil, ordinaryDate},
		},
		{
			name:  "timestamp with zone not null",
			field: arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}},
			col:   instants,
			want:  []any{maxInstant, minDate, ordinaryInstant},
		},
		{
			name:  "timestamp with zone nullable",
			field: arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}, Nullable: true},
			col:   instants,
			want:  []any{nil, nil, ordinaryInstant},
		},
		{
			name:  "timestamp naive not null",
			field: arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}},
			col:   naive,
			want:  []any{"2262-04-11 23:47:16", "1900-01-01 00:00:00", "2024-01-01 00:00:00"},
		},
		{
			name:  "timestamp naive nullable",
			field: arrow.Field{Name: "ts", Type: &arrow.TimestampType{Unit: arrow.Microsecond}, Nullable: true},
			col:   naive,
			want:  []any{nil, nil, "2024-01-01 00:00:00"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := valueFor(tt.field)
			for i, want := range tt.want {
				if got := fn(tt.col, i); got != want {
					t.Errorf("row %d = %#v, want %#v", i, got, want)
				}
			}
		})
	}
}

// The clamped values must survive the real clickhouse-go columns unchanged:
// a bound the driver wrapped or rejected would just move the corruption.
func TestInfinityClampSurvivesDriverColumns(t *testing.T) {
	sc := &column.ServerContext{Timezone: time.UTC}

	dates, err := column.Type("Date32").Column("d", sc)
	if err != nil {
		t.Fatalf("make Date32 column: %v", err)
	}
	for _, v := range []time.Time{
		time.Date(2299, 12, 31, 0, 0, 0, 0, time.UTC),
		time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if err := dates.AppendRow(v); err != nil {
			t.Fatalf("append %v: %v", v, err)
		}
	}
	if got := dates.Row(0, false); got != time.Date(2299, 12, 31, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("Date32 +infinity clamp stored as %v", got)
	}
	if got := dates.Row(1, false); got != time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("Date32 -infinity clamp stored as %v", got)
	}

	stamps, err := column.Type("DateTime64(6)").Column("ts", sc)
	if err != nil {
		t.Fatalf("make DateTime64 column: %v", err)
	}
	for _, v := range []time.Time{
		time.Date(2262, 4, 11, 23, 47, 16, 0, time.UTC),
		time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
	} {
		if err := stamps.AppendRow(v); err != nil {
			t.Fatalf("append %v: %v", v, err)
		}
	}
	if got := stamps.Row(0, false); got != time.Date(2262, 4, 11, 23, 47, 16, 0, time.UTC) {
		t.Fatalf("DateTime64 +infinity clamp stored as %v", got)
	}
	if got := stamps.Row(1, false); got != time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatalf("DateTime64 -infinity clamp stored as %v", got)
	}
}
