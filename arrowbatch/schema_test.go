package arrowbatch

import (
	"slices"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

// everyLogical lists each logical type with any variant that changes its storage.
var everyLogical = []rowmodel.Field{
	{Logical: rowmodel.LogicalBool},
	{Logical: rowmodel.LogicalInt16},
	{Logical: rowmodel.LogicalInt32},
	{Logical: rowmodel.LogicalInt64},
	{Logical: rowmodel.LogicalFloat32},
	{Logical: rowmodel.LogicalFloat64},
	{Logical: rowmodel.LogicalDecimal, Precision: 12, Scale: 2},
	{Logical: rowmodel.LogicalDecimal},
	{Logical: rowmodel.LogicalString},
	{Logical: rowmodel.LogicalBytes},
	{Logical: rowmodel.LogicalDate},
	{Logical: rowmodel.LogicalTime},
	{Logical: rowmodel.LogicalTimestamp},
	{Logical: rowmodel.LogicalTimestampTZ},
	{Logical: rowmodel.LogicalJSON},
	{Logical: rowmodel.LogicalUUID},
	{Logical: rowmodel.LogicalArray},
}

func TestEveryLogicalCoversRowmodel(t *testing.T) {
	for _, logical := range rowmodel.LogicalTypes {
		if !slices.ContainsFunc(everyLogical, func(f rowmodel.Field) bool { return f.Logical == logical }) {
			t.Errorf("logical type %q has no round-trip case; add it here and to FieldOf", logical)
		}
	}
}

func TestFieldOfInvertsField(t *testing.T) {
	for _, want := range everyLogical {
		want.Name, want.Nullable, want.Native = "c", true, "native_"+string(want.Logical)
		got, err := FieldOf(Field(want))
		if err != nil {
			t.Fatalf("%s: %v", want.Logical, err)
		}
		if got != want {
			t.Errorf("FieldOf(Field(%+v)) = %+v", want, got)
		}
	}
}
