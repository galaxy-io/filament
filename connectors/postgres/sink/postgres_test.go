package postgres

import (
	"strings"
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

func TestPostgresSinkAdvertisesCDCAndBuildsKeyDelete(t *testing.T) {
	spec := New().Spec()
	found := false
	for _, capability := range spec.Capabilities.WritePolicies {
		if capability.Mode == filament.WriteMerge {
			found = true
		}
	}
	if !found {
		t.Fatal("postgres sink does not advertise CDC merge")
	}
	sql := deleteUsingSQL(`"public"."users"`, filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "tenant_id", Logical: filament.LogicalInt64},
			{Name: "id", Logical: filament.LogicalUUID},
		},
		PrimaryKey: []string{"tenant_id", "id"},
	})
	for _, want := range []string{`"tenant_id" bigint`, `"id" uuid`, `t."tenant_id" = x."tenant_id"`, `t."id" = x."id"`} {
		if !strings.Contains(sql, want) {
			t.Fatalf("delete SQL %q does not contain %q", sql, want)
		}
	}
}
