package motherduck

import (
	"context"
	"strings"
	"testing"

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
