package transform

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
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
			// A decimal needs a precision to be compared as decimal128
			// rather than as text.
			schema := rowmodel.Schema{
				Resource: "items",
				Fields: []rowmodel.Field{
					{Name: "left", Logical: typ, Precision: 12, Scale: 2},
					{Name: "right", Logical: typ, Precision: 12, Scale: 2},
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

// TestCompileRejectsTextDecimalComparison covers decimals arrowbatch carries
// as text: no precision (Postgres numeric without a typmod) or more than 38
// digits. Compared as strings, 10 > 9 would be false and 1.0 = 1.00 too.
func TestCompileRejectsTextDecimalComparison(t *testing.T) {
	for _, precision := range []int{0, 39, 40} {
		for _, fn := range []string{"eq", "neq", "gt", "gte", "lt", "lte"} {
			t.Run(fmt.Sprintf("%s precision %d", fn, precision), func(t *testing.T) {
				cond := Expr{Fn: fn, Args: []Expr{{Col: "left"}, {Col: "right"}}}
				// The same comparison as a computed column and as a where
				// condition, which compiles on its own path.
				for _, tc := range []struct {
					step Step
					path string
				}{
					{Step{Compute: map[string]Expr{"compared": cond}}, `resources["items"].steps[0].compute["compared"].` + fn + `[0]`},
					{Step{Compute: map[string]Expr{"flag": {Lit: "x"}}, Where: &cond}, `resources["items"].steps[0].where.` + fn + `[0]`},
				} {
					definition := &Definition{Version: 1, Resources: map[string]Resource{"items": {Steps: []Step{tc.step}}}}
					schema := rowmodel.Schema{
						Resource: "items",
						Fields: []rowmodel.Field{
							{Name: "left", Logical: rowmodel.LogicalDecimal, Precision: precision},
							{Name: "right", Logical: rowmodel.LogicalDecimal, Precision: precision},
						},
					}
					_, err := Compile(definition, schema)
					var errs *Errors
					if !errors.As(err, &errs) || len(errs.Issues) != 1 {
						t.Fatalf("Compile %s over decimal precision %d = %v, want one issue", fn, precision, err)
					}
					if got := errs.Issues[0]; got.Path != tc.path || !strings.Contains(got.Message, "carried as text") {
						t.Errorf("issue = %q: %q, want %q: carried as text", got.Path, got.Message, tc.path)
					}
				}
			})
		}
	}
}

// TestApplyComparesBoundedDecimalsNumerically checks that decimals stored as
// decimal128 still compile and compare as numbers, up to 38 digits.
func TestApplyComparesBoundedDecimalsNumerically(t *testing.T) {
	for _, precision := range []int{12, 38} {
		t.Run(fmt.Sprintf("precision %d", precision), func(t *testing.T) {
			definition := &Definition{Version: 1, Resources: map[string]Resource{"items": {Steps: []Step{{Compute: map[string]Expr{
				"greater": {Fn: "gt", Args: []Expr{{Col: "left"}, {Col: "right"}}},
				"equal":   {Fn: "eq", Args: []Expr{{Col: "left"}, {Col: "right"}}},
			}}}}}}
			schema := rowmodel.Schema{
				Resource: "items",
				Fields: []rowmodel.Field{
					{Name: "left", Logical: rowmodel.LogicalDecimal, Precision: precision, Scale: 2},
					{Name: "right", Logical: rowmodel.LogicalDecimal, Precision: precision, Scale: 2},
				},
			}
			plan, err := Compile(definition, schema)
			if err != nil {
				t.Fatalf("Compile: %v", err)
			}
			b := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
			defer b.Release()
			for i, values := range [][]string{{"10", "-1", "1.0", "2"}, {"9", "-2", "1.00", "10"}} {
				for _, v := range values {
					if err := b.Field(i).AppendValueFromString(v); err != nil {
						t.Fatal(err)
					}
				}
			}
			in := b.NewRecordBatch()
			defer in.Release()
			out, err := plan.Apply(context.Background(), memory.DefaultAllocator, in)
			if err != nil {
				t.Fatalf("Apply: %v", err)
			}
			defer out.Release()
			greater := out.Column(out.Schema().FieldIndices("greater")[0]).(*array.Boolean)
			equal := out.Column(out.Schema().FieldIndices("equal")[0]).(*array.Boolean)
			for i, want := range []struct{ greater, equal bool }{{true, false}, {true, false}, {false, true}, {false, false}} {
				if greater.Value(i) != want.greater || equal.Value(i) != want.equal {
					t.Errorf("row %d gt = %v eq = %v, want %v %v", i, greater.Value(i), equal.Value(i), want.greater, want.equal)
				}
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
	operators := map[string]string{
		"eq": "=", "neq": "≠", "gt": ">", "gte": "≥", "lt": "<", "lte": "≤",
		"add": "+", "sub": "−", "mul": "×", "div": "÷",
	}
	conditionOperators := map[string]bool{
		"eq": true, "neq": true, "gt": true, "gte": true, "lt": true, "lte": true,
		"regex_match": true, "is_null": true,
	}
	conditionJoins := map[string]string{"and": "and", "or": "or"}
	variadicLabels := map[string]string{
		"concat": "Add part", "coalesce": "Add value",
		"md5": "Add column", "sha1": "Add column", "sha256": "Add column", "sha512": "Add column",
	}
	for _, spec := range Functions() {
		if spec.DisplayName == "" {
			t.Errorf("function %q has no display name", spec.Name)
		}
		// A picker row holds about 16 characters; an operator shows its symbol beside the name.
		if len([]rune(spec.DisplayName)) > 16 && spec.OperatorSymbol == "" {
			t.Errorf("function %q display name %q is longer than 16 characters", spec.Name, spec.DisplayName)
		}
		if want, ok := operators[spec.Name]; ok {
			if spec.OperatorSymbol != want {
				t.Errorf("function %q operator symbol = %q, want %q", spec.Name, spec.OperatorSymbol, want)
			}
		} else if spec.OperatorSymbol != "" {
			t.Errorf("non-operator function %q has operator symbol %q", spec.Name, spec.OperatorSymbol)
		}
		for _, arg := range spec.Args {
			if arg.DisplayName == "" {
				t.Errorf("function %q argument %q has no display name", spec.Name, arg.Name)
			}
		}
		if spec.ConditionOperator && spec.Returns != rowmodel.LogicalBool {
			t.Errorf("condition operator %q returns %s, want bool", spec.Name, spec.Returns)
		}
		if spec.ConditionOperator != conditionOperators[spec.Name] {
			t.Errorf("function %q condition operator = %t, want %t", spec.Name, spec.ConditionOperator, conditionOperators[spec.Name])
		}
		if spec.ConditionJoin != "" && spec.ConditionJoin != "and" && spec.ConditionJoin != "or" {
			t.Errorf("condition join %q has unsupported mode %q", spec.Name, spec.ConditionJoin)
		}
		if spec.ConditionJoin != conditionJoins[spec.Name] {
			t.Errorf("function %q condition join = %q, want %q", spec.Name, spec.ConditionJoin, conditionJoins[spec.Name])
		}
		if spec.VariadicAddLabel != "" && !spec.Args[len(spec.Args)-1].Variadic {
			t.Errorf("non-variadic function %q has variadic add label %q", spec.Name, spec.VariadicAddLabel)
		}
		if spec.VariadicAddLabel != variadicLabels[spec.Name] {
			t.Errorf("function %q variadic add label = %q, want %q", spec.Name, spec.VariadicAddLabel, variadicLabels[spec.Name])
		}
	}
}

func TestCompileRejectsNegativeRegexExtractGroup(t *testing.T) {
	definition := &Definition{
		Version: 1,
		Resources: map[string]Resource{
			"items": {Steps: []Step{{Compute: map[string]Expr{
				"match": {
					Fn: "regex_extract",
					Args: []Expr{
						{Col: "value"},
						{Lit: "(.*)"},
						{Lit: int64(-1)},
					},
				},
			}}}},
		},
	}
	schema := rowmodel.Schema{
		Resource: "items",
		Fields:   []rowmodel.Field{{Name: "value", Logical: rowmodel.LogicalString}},
	}

	if _, err := Compile(definition, schema); err == nil {
		t.Fatal("Compile regex_extract with negative group succeeded, want an error")
	}
}
