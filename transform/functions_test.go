package transform

import (
	"slices"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

func functionNames(specs []FunctionSpec) []string {
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.Name
	}
	return names
}

func TestFunctionsForOrderableTypes(t *testing.T) {
	for _, typ := range []rowmodel.LogicalType{rowmodel.LogicalDecimal, rowmodel.LogicalTime} {
		names := functionNames(FunctionsFor(typ))
		for _, name := range []string{"gt", "gte", "lt", "lte"} {
			if !slices.Contains(names, name) {
				t.Errorf("FunctionsFor(%s) does not contain %q", typ, name)
			}
		}
	}

	for _, name := range []string{"gt", "gte", "lt", "lte"} {
		if slices.Contains(functionNames(FunctionsFor(rowmodel.LogicalString)), name) {
			t.Errorf("FunctionsFor(string) contains %q", name)
		}
	}
}

func TestFunctionsForEveryTypeIncludeEquality(t *testing.T) {
	all := []rowmodel.LogicalType{
		rowmodel.LogicalBool,
		rowmodel.LogicalInt16,
		rowmodel.LogicalInt32,
		rowmodel.LogicalInt64,
		rowmodel.LogicalFloat32,
		rowmodel.LogicalFloat64,
		rowmodel.LogicalDecimal,
		rowmodel.LogicalString,
		rowmodel.LogicalBytes,
		rowmodel.LogicalDate,
		rowmodel.LogicalTime,
		rowmodel.LogicalTimestamp,
		rowmodel.LogicalTimestampTZ,
		rowmodel.LogicalJSON,
		rowmodel.LogicalUUID,
		rowmodel.LogicalArray,
	}
	for _, typ := range all {
		names := functionNames(FunctionsFor(typ))
		for _, name := range []string{"eq", "neq"} {
			if !slices.Contains(names, name) {
				t.Errorf("FunctionsFor(%s) does not contain %q", typ, name)
			}
		}
	}
}

func TestCompileOrderableDecimalAndTimeComparisons(t *testing.T) {
	for _, typ := range []rowmodel.LogicalType{rowmodel.LogicalDecimal, rowmodel.LogicalTime} {
		t.Run(string(typ), func(t *testing.T) {
			definition := &Definition{
				Version: 1,
				Resources: map[string]Resource{
					"items": {Steps: []Step{{Compute: map[string]Expr{
						"ordered": {
							Fn: "gt",
							Args: []Expr{
								{Col: "left"},
								{Col: "right"},
							},
						},
					}}}},
				},
			}
			schema := rowmodel.Schema{
				Resource: "items",
				Fields: []rowmodel.Field{
					{Name: "left", Logical: typ},
					{Name: "right", Logical: typ},
				},
			}

			plan, err := Compile(definition, schema)
			if err != nil {
				t.Fatalf("Compile comparison over %s: %v", typ, err)
			}
			got := plan.Schema().Fields[2].Logical
			if got != rowmodel.LogicalBool {
				t.Fatalf("comparison result type = %s, want bool", got)
			}
		})
	}
}

func TestCompileRejectsStringOrdering(t *testing.T) {
	definition := &Definition{
		Version: 1,
		Resources: map[string]Resource{
			"items": {Steps: []Step{{Compute: map[string]Expr{
				"ordered": {
					Fn: "gt",
					Args: []Expr{
						{Col: "left"},
						{Col: "right"},
					},
				},
			}}}},
		},
	}
	schema := rowmodel.Schema{
		Resource: "items",
		Fields: []rowmodel.Field{
			{Name: "left", Logical: rowmodel.LogicalString},
			{Name: "right", Logical: rowmodel.LogicalString},
		},
	}

	if _, err := Compile(definition, schema); err == nil {
		t.Fatal("Compile string ordering succeeded, want an unsupported-type error")
	}
}

func TestFunctionCatalogCarriesBuilderMetadata(t *testing.T) {
	for _, spec := range Functions() {
		if spec.DisplayName == "" {
			t.Errorf("function %q has no display name", spec.Name)
		}
	}
}
