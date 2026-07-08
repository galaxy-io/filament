package mysql

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestMysqlColumnTypeMapsPortableLogicalTypes(t *testing.T) {
	tests := []struct {
		name  string
		field ingestion.SchemaField
		want  string
	}{
		{name: "bool", field: ingestion.SchemaField{Logical: ingestion.LogicalBool, Native: "tinyint(1)"}, want: "tinyint(1)"},
		{name: "int16", field: ingestion.SchemaField{Logical: ingestion.LogicalInt16}, want: "smallint"},
		{name: "int32", field: ingestion.SchemaField{Logical: ingestion.LogicalInt32}, want: "int"},
		{name: "int64", field: ingestion.SchemaField{Logical: ingestion.LogicalInt64}, want: "bigint"},
		{name: "float32", field: ingestion.SchemaField{Logical: ingestion.LogicalFloat32}, want: "float"},
		{name: "float64", field: ingestion.SchemaField{Logical: ingestion.LogicalFloat64}, want: "double"},
		{name: "decimal default", field: ingestion.SchemaField{Logical: ingestion.LogicalDecimal}, want: "decimal(38,9)"},
		{name: "decimal native mysql", field: ingestion.SchemaField{Logical: ingestion.LogicalDecimal, Native: "decimal(12,2)"}, want: "decimal(12,2)"},
		{name: "string default", field: ingestion.SchemaField{Logical: ingestion.LogicalString}, want: "longtext"},
		{name: "string native mysql", field: ingestion.SchemaField{Logical: ingestion.LogicalString, Native: "varchar(64)"}, want: "varchar(64)"},
		{name: "string native foreign", field: ingestion.SchemaField{Logical: ingestion.LogicalString, Native: "character varying(20)"}, want: "longtext"},
		{name: "bytes default", field: ingestion.SchemaField{Logical: ingestion.LogicalBytes}, want: "longblob"},
		{name: "bytes native mysql", field: ingestion.SchemaField{Logical: ingestion.LogicalBytes, Native: "varbinary(255)"}, want: "varbinary(255)"},
		{name: "date", field: ingestion.SchemaField{Logical: ingestion.LogicalDate}, want: "date"},
		{name: "time", field: ingestion.SchemaField{Logical: ingestion.LogicalTime}, want: "time(6)"},
		{name: "timestamp", field: ingestion.SchemaField{Logical: ingestion.LogicalTimestamp}, want: "datetime(6)"},
		{name: "timestamptz avoids 2038 range", field: ingestion.SchemaField{Logical: ingestion.LogicalTimestampTZ, Native: "timestamp with time zone"}, want: "datetime(6)"},
		{name: "json", field: ingestion.SchemaField{Logical: ingestion.LogicalJSON, Native: "json"}, want: "json"},
		{name: "array maps to json", field: ingestion.SchemaField{Logical: ingestion.LogicalArray, Native: "text[]"}, want: "json"},
		{name: "uuid", field: ingestion.SchemaField{Logical: ingestion.LogicalUUID}, want: "char(36)"},
		{name: "unknown default", field: ingestion.SchemaField{}, want: "longtext"},
		{name: "unknown native mysql", field: ingestion.SchemaField{Logical: ingestion.LogicalUnknown, Native: "year"}, want: "year"},
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
	schema := ingestion.RecordSchema{
		PrimaryKey: []string{"id"},
		Fields: []ingestion.SchemaField{
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
	pkOnly := ingestion.RecordSchema{PrimaryKey: []string{"id"}, Fields: []ingestion.SchemaField{{Name: "id"}}}
	want = " ON DUPLICATE KEY UPDATE `id` = new.`id`"
	if got := onDuplicate(true, pkOnly); got != want {
		t.Fatalf("pk-only clause = %q, want %q", got, want)
	}
}
