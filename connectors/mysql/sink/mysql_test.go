package mysql

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestMysqlColumnTypeMapsPortableLogicalTypes(t *testing.T) {
	tests := []struct {
		name  string
		field filament.SchemaField
		want  string
	}{
		{name: "bool", field: filament.SchemaField{Logical: filament.LogicalBool, Native: "tinyint(1)"}, want: "tinyint(1)"},
		{name: "int16", field: filament.SchemaField{Logical: filament.LogicalInt16}, want: "smallint"},
		{name: "int32", field: filament.SchemaField{Logical: filament.LogicalInt32}, want: "int"},
		{name: "int64", field: filament.SchemaField{Logical: filament.LogicalInt64}, want: "bigint"},
		{name: "float32", field: filament.SchemaField{Logical: filament.LogicalFloat32}, want: "float"},
		{name: "float64", field: filament.SchemaField{Logical: filament.LogicalFloat64}, want: "double"},
		{name: "decimal default", field: filament.SchemaField{Logical: filament.LogicalDecimal}, want: "decimal(38,9)"},
		{name: "decimal native mysql", field: filament.SchemaField{Logical: filament.LogicalDecimal, Native: "decimal(12,2)"}, want: "decimal(12,2)"},
		{name: "string default", field: filament.SchemaField{Logical: filament.LogicalString}, want: "longtext"},
		{name: "string native mysql", field: filament.SchemaField{Logical: filament.LogicalString, Native: "varchar(64)"}, want: "varchar(64)"},
		{name: "string native foreign", field: filament.SchemaField{Logical: filament.LogicalString, Native: "character varying(20)"}, want: "longtext"},
		{name: "bytes default", field: filament.SchemaField{Logical: filament.LogicalBytes}, want: "longblob"},
		{name: "bytes native mysql", field: filament.SchemaField{Logical: filament.LogicalBytes, Native: "varbinary(255)"}, want: "varbinary(255)"},
		{name: "date", field: filament.SchemaField{Logical: filament.LogicalDate}, want: "date"},
		{name: "time", field: filament.SchemaField{Logical: filament.LogicalTime}, want: "time(6)"},
		{name: "timestamp", field: filament.SchemaField{Logical: filament.LogicalTimestamp}, want: "datetime(6)"},
		{name: "timestamptz avoids 2038 range", field: filament.SchemaField{Logical: filament.LogicalTimestampTZ, Native: "timestamp with time zone"}, want: "datetime(6)"},
		{name: "json", field: filament.SchemaField{Logical: filament.LogicalJSON, Native: "json"}, want: "json"},
		{name: "array maps to json", field: filament.SchemaField{Logical: filament.LogicalArray, Native: "text[]"}, want: "json"},
		{name: "uuid", field: filament.SchemaField{Logical: filament.LogicalUUID}, want: "char(36)"},
		{name: "unknown default", field: filament.SchemaField{}, want: "longtext"},
		{name: "unknown native mysql", field: filament.SchemaField{Logical: filament.LogicalUnknown, Native: "year"}, want: "year"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mysqlColumnType(tt.field); got != tt.want {
				t.Fatalf("mysqlColumnType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOnDuplicate(t *testing.T) {
	schema := filament.RecordSchema{
		PrimaryKey: []string{"id"},
		Fields: []filament.SchemaField{
			{Name: "id"}, {Name: "name"}, {Name: "age"},
		},
	}
	if got := onDuplicate(false, schema); got != "" {
		t.Fatalf("non-upsert clause = %q, want empty", got)
	}
	want := " ON DUPLICATE KEY UPDATE `name` = new.`name`, `age` = new.`age`"
	if got := onDuplicate(true, schema); got != want {
		t.Fatalf("upsert clause = %q, want %q", got, want)
	}
	// PK-only table degrades to a no-op update (MySQL's DO NOTHING idiom).
	pkOnly := filament.RecordSchema{PrimaryKey: []string{"id"}, Fields: []filament.SchemaField{{Name: "id"}}}
	want = " ON DUPLICATE KEY UPDATE `id` = new.`id`"
	if got := onDuplicate(true, pkOnly); got != want {
		t.Fatalf("pk-only clause = %q, want %q", got, want)
	}
}
