package clickhouse

import (
	"context"
	"math"
	"strings"
	"testing"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/shopspring/decimal"

	"github.com/galaxy-io/filament"
)

func TestSpecAdvertisesInitialWritePolicies(t *testing.T) {
	spec := New().Spec()
	if spec.Name != "clickhouse" || !spec.Capabilities.Schematized || !spec.Capabilities.Upsertable {
		t.Fatalf("unexpected spec: %+v", spec)
	}
	if spec.Capabilities.PreferredBatchRows != 10_000 {
		t.Fatalf("preferred rows = %d", spec.Capabilities.PreferredBatchRows)
	}
	want := []filament.WriteMode{filament.WriteReplace, filament.WriteAppend, filament.WriteUpsert, filament.WriteUpsert}
	for i, policy := range spec.Capabilities.WritePolicies {
		if policy.Mode != want[i] {
			t.Fatalf("policy %d mode = %q, want %q", i, policy.Mode, want[i])
		}
	}
	if spec.Version != "1" {
		t.Fatalf("version = %q, want 2", spec.Version)
	}
	fields := make(map[string]filament.ConfigField, len(spec.Config.Fields))
	for _, field := range spec.Config.Fields {
		fields[field.Name] = field
	}
	if _, ok := fields["dsn"]; ok {
		t.Fatal("spec still exposes removed dsn field")
	}
	if !fields["host"].Required || fields["password"].Type != filament.FieldSecret || !fields["password"].Required {
		t.Fatalf("unexpected connection fields: %+v", fields)
	}
}

func TestConnectionOptionsFromFields(t *testing.T) {
	opts, err := connectionOptions(filament.NewConfig(map[string]any{
		"host": "https://service.example.clickhouse.cloud", "port": 9440,
		"username": "loader", "password": "secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := opts.Addr; len(got) != 1 || got[0] != "service.example.clickhouse.cloud:9440" {
		t.Fatalf("address = %v", got)
	}
	if opts.Protocol.String() != protocolNative || opts.TLS == nil {
		t.Fatalf("protocol = %s, TLS = %#v", opts.Protocol, opts.TLS)
	}
	if opts.Compression == nil || opts.Compression.Method != ch.CompressionLZ4 {
		t.Fatalf("native compression = %#v, want LZ4", opts.Compression)
	}
	if opts.Auth.Database != defaultDatabase || opts.Auth.Username != "loader" || opts.Auth.Password != "secret" {
		t.Fatalf("auth = %+v", opts.Auth)
	}
}

func TestConnectionOptionsSupportsPlainHTTP(t *testing.T) {
	opts, err := connectionOptions(filament.NewConfig(map[string]any{
		"host": "::1", "port": 8123, "protocol": protocolHTTP, "secure": false, "password": "local-secret",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := opts.Addr[0]; got != "[::1]:8123" {
		t.Fatalf("address = %q", got)
	}
	if opts.Protocol.String() != protocolHTTP || opts.TLS != nil {
		t.Fatalf("protocol = %s, TLS = %#v", opts.Protocol, opts.TLS)
	}
	if opts.Compression != nil {
		t.Fatalf("HTTP compression = %#v, want disabled", opts.Compression)
	}
	if opts.Auth.Username != defaultUsername {
		t.Fatalf("username = %q", opts.Auth.Username)
	}
}

func TestConnectionOptionsRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
		want string
	}{
		{name: "missing host", cfg: map[string]any{}, want: "host is required"},
		{name: "invalid port", cfg: map[string]any{"host": "localhost", "port": 70000}, want: "port must be"},
		{name: "invalid protocol", cfg: map[string]any{"host": "localhost", "protocol": "postgres"}, want: "unsupported protocol"},
		{name: "port in host", cfg: map[string]any{"host": "https://localhost:8443"}, want: "use the port field"},
		{name: "path in host", cfg: map[string]any{"host": "https://localhost/query"}, want: "must not contain"},
		{name: "missing password", cfg: map[string]any{"host": "localhost"}, want: "password is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := connectionOptions(filament.NewConfig(tt.cfg))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
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
		{name: "decimal native", field: filament.SchemaField{Logical: filament.LogicalDecimal, Native: "numeric(12, 2)"}, want: "Decimal(12, 2)"},
		{name: "decimal invalid", field: filament.SchemaField{Logical: filament.LogicalDecimal, Native: "decimal(100,2)"}, want: "Decimal(38, 9)"},
		{name: "timestamp", field: filament.SchemaField{Logical: filament.LogicalTimestamp}, want: "DateTime64(6)"},
		{name: "timestamptz", field: filament.SchemaField{Logical: filament.LogicalTimestampTZ}, want: "DateTime64(6, 'UTC')"},
		{name: "json text", field: filament.SchemaField{Logical: filament.LogicalJSON}, want: "String"},
		{name: "array text", field: filament.SchemaField{Logical: filament.LogicalArray}, want: "String"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clickhouseColumnType(tt.field); got != tt.want {
				t.Fatalf("clickhouseColumnType() = %q, want %q", got, tt.want)
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

func TestDecodeRecordPreservesTypes(t *testing.T) {
	schema := filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "name", Logical: filament.LogicalString},
		{Name: "active", Logical: filament.LogicalBool},
		{Name: "score", Logical: filament.LogicalFloat64},
		{Name: "amount", Logical: filament.LogicalDecimal},
		{Name: "payload", Logical: filament.LogicalJSON},
		{Name: "tags", Logical: filament.LogicalArray},
		{Name: "bytes", Logical: filament.LogicalBytes},
		{Name: "optional", Logical: filament.LogicalString, Nullable: true},
	}}
	values, err := decodeRecord(schema, []byte(`{
		"id":9223372036854775806,"name":"Ada","active":true,"score":"NaN",
		"amount":"123.450","payload":{"ok":true},"tags":["a","b"],"bytes":"aGk="
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := values[0]; got != int64(9223372036854775806) {
		t.Fatalf("id = %#v", got)
	}
	if values[1] != "Ada" || values[2] != true || !math.IsNaN(values[3].(float64)) {
		t.Fatalf("scalar values = %#v", values[:4])
	}
	if got := values[4].(decimal.Decimal); !got.Equal(decimal.RequireFromString("123.450")) {
		t.Fatalf("decimal = %s", got)
	}
	if values[5] != `{"ok":true}` || values[6] != `["a","b"]` || values[7] != "hi" {
		t.Fatalf("structured values = %#v", values[5:8])
	}
	if len(values) != 9 || values[8] != nil {
		t.Fatalf("nullable value = %#v", values[8:])
	}
}

func TestDecodeRecordRejectsMissingRequiredField(t *testing.T) {
	_, err := decodeRecord(filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
	}}, []byte(`{}`))
	if err == nil || !strings.Contains(err.Error(), `field "id"`) {
		t.Fatalf("error = %v", err)
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
	sink.mode = filament.WriteAppend
	_, err := sink.Apply(context.Background(), filament.Batch{}, filament.ApplyOptions{
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
