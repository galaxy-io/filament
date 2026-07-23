package postgres

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestPostgresColumnTypeMapsPortableLogicalTypes(t *testing.T) {
	tests := []struct {
		name  string
		field filament.SchemaField
		want  string
	}{
		{name: "string", field: filament.SchemaField{Logical: filament.LogicalString, Native: "string"}, want: "text"},
		{name: "json", field: filament.SchemaField{Logical: filament.LogicalJSON, Native: "json"}, want: "jsonb"},
		{name: "unknown native", field: filament.SchemaField{Logical: filament.LogicalUnknown, Native: "inet"}, want: "inet"},
		{name: "unknown default", field: filament.SchemaField{}, want: "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postgresColumnType(tt.field); got != tt.want {
				t.Fatalf("postgresColumnType() = %q, want %q", got, tt.want)
			}
		})
	}
}
