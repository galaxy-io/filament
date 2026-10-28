package transform

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// compareDefinition computes gt and eq over columns a and b.
var compareDefinition = &Definition{Version: 1, Resources: map[string]Resource{
	"items": {Steps: []Step{{Compute: map[string]Expr{
		"gt": {Fn: "gt", Args: []Expr{{Col: "a"}, {Col: "b"}}},
		"eq": {Fn: "eq", Args: []Expr{{Col: "a"}, {Col: "b"}}},
	}}}},
}}

func decimalPair(precision int) rowmodel.Schema {
	return rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{
		{Name: "a", Logical: rowmodel.LogicalDecimal, Precision: precision, Scale: 2, Nullable: true},
		{Name: "b", Logical: rowmodel.LogicalDecimal, Precision: precision, Scale: 2, Nullable: true},
	}}
}

// textRows builds a record of two utf8 decimal columns; "" is null.
func textRows(t *testing.T, schema rowmodel.Schema, pairs [][2]string) arrow.RecordBatch {
	t.Helper()
	b := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
	defer b.Release()
	for _, p := range pairs {
		for i, v := range p {
			if v == "" {
				b.Field(i).(*array.StringBuilder).AppendNull()
			} else {
				b.Field(i).(*array.StringBuilder).Append(v)
			}
		}
	}
	rec := b.NewRecordBatch()
	t.Cleanup(rec.Release)
	return rec
}

// boolText renders a boolean column as true, false and null per row.
func boolText(rec arrow.RecordBatch, name string) []string {
	col := rec.Column(rec.Schema().FieldIndices(name)[0]).(*array.Boolean)
	out := make([]string, col.Len())
	for i := range col.Len() {
		switch {
		case col.IsNull(i):
			out[i] = "null"
		case col.Value(i):
			out[i] = "true"
		default:
			out[i] = "false"
		}
	}
	return out
}

func TestCompareDecimalText(t *testing.T) {
	rows := []struct{ a, b, gt, eq string }{
		{"10", "9", "true", "false"},
		{"-1", "-2", "true", "false"},
		{"100", "99", "true", "false"},
		{"1.0", "1.00", "false", "true"},
		{"2", "10", "false", "false"},
		{"", "1", "null", "null"},
		{"NaN", "1", "true", "false"},
		{"NaN", "NaN", "false", "true"},
		{"Infinity", "1e9", "true", "false"},
		{"-Infinity", "-1e9", "false", "false"},
	}
	for _, precision := range []int{0, 40} {
		schema := decimalPair(precision)
		plan, err := Compile(compareDefinition, schema)
		if err != nil {
			t.Fatalf("precision %d: %v", precision, err)
		}
		var pairs [][2]string
		for _, r := range rows {
			pairs = append(pairs, [2]string{r.a, r.b})
		}
		out, err := plan.Apply(context.Background(), nil, textRows(t, schema, pairs))
		if err != nil {
			t.Fatalf("precision %d: %v", precision, err)
		}
		defer out.Release()
		gt, eq := boolText(out, "gt"), boolText(out, "eq")
		for i, r := range rows {
			if gt[i] != r.gt || eq[i] != r.eq {
				t.Errorf("precision %d: (%q, %q) gt=%s eq=%s, want gt=%s eq=%s", precision, r.a, r.b, gt[i], eq[i], r.gt, r.eq)
			}
		}
	}
}

func TestCompareDecimalTextRejectsNonNumbers(t *testing.T) {
	schema := decimalPair(0)
	plan, err := Compile(compareDefinition, schema)
	if err != nil {
		t.Fatal(err)
	}
	// big.Rat.SetString would accept the first group as numbers and refuses
	// the oversized exponents without a syntax error.
	for _, bad := range []string{
		"abc", "1/2", "0x10", "1p2", "1_000", "1e", ".", "+", "1.2.3", " 1",
		"1e1000001", "1e9223372036854775808", "1e-9223372036854775808",
	} {
		_, err = plan.Apply(context.Background(), nil, textRows(t, schema, [][2]string{{"1", "1"}, {bad, "1"}}))
		if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("row 1: %q is not a decimal", bad)) {
			t.Fatalf("Apply(%q) error = %v, want the bad row and value", bad, err)
		}
	}
}

func TestCompareDecimalStorageMustMatch(t *testing.T) {
	schema := rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{
		{Name: "a", Logical: rowmodel.LogicalDecimal, Precision: 40, Scale: 2},
		{Name: "b", Logical: rowmodel.LogicalDecimal, Precision: 10, Scale: 2},
	}}
	_, err := Compile(compareDefinition, schema)
	if err == nil || !strings.Contains(err.Error(), "expects matching types, got decimal(40,2) and decimal(10,2)") {
		t.Fatalf("Compile error = %v, want a type mismatch", err)
	}
}

func TestCompareBoundedDecimalUsesArrow(t *testing.T) {
	schema := decimalPair(10)
	plan, err := Compile(compareDefinition, schema)
	if err != nil {
		t.Fatal(err)
	}
	b := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
	defer b.Release()
	for _, cents := range [][2]int64{{1000, 900}, {100, 100}} {
		b.Field(0).(*array.Decimal128Builder).Append(decimal128.FromI64(cents[0]))
		b.Field(1).(*array.Decimal128Builder).Append(decimal128.FromI64(cents[1]))
	}
	rec := b.NewRecordBatch()
	defer rec.Release()
	out, err := plan.Apply(context.Background(), nil, rec)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Release()
	if gt, eq := boolText(out, "gt"), boolText(out, "eq"); gt[0] != "true" || eq[0] != "false" || gt[1] != "false" || eq[1] != "true" {
		t.Fatalf("gt=%v eq=%v", gt, eq)
	}
}
