package postgres

import (
	"testing"

	ingestion "github.com/galaxy-io/filament"
)

func TestPostgresColumnTypeMapsPortableLogicalTypes(t *testing.T) {
	tests := []struct {
		name  string
		field ingestion.SchemaField
		want  string
	}{
		{name: "string", field: ingestion.SchemaField{Logical: ingestion.LogicalString, Native: "string"}, want: "text"},
		{name: "json", field: ingestion.SchemaField{Logical: ingestion.LogicalJSON, Native: "json"}, want: "jsonb"},
		{name: "unknown native", field: ingestion.SchemaField{Logical: ingestion.LogicalUnknown, Native: "inet"}, want: "inet"},
		{name: "unknown default", field: ingestion.SchemaField{}, want: "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postgresColumnType(tt.field); got != tt.want {
				t.Fatalf("postgresColumnType() = %q, want %q", got, tt.want)
			}
		})
	}
}
