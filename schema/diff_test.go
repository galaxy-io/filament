package schema_test

import (
	"reflect"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

func TestDiffDoesNotInferRename(t *testing.T) {
	before, after := model(), model()
	after.Fields[1].Name = "label"
	diff, err := schema.Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Changes) != 2 || diff.Changes[0].Kind != schema.DropField || diff.Changes[1].Kind != schema.AddField {
		t.Fatalf("rename guessed: %+v", diff)
	}
	after.Fields[1].Name = "mutated"
	diff.Changes[0].Before.Native = "mutated"
	if diff.Changes[1].After.Name != "label" || before.Fields[1].Native != "text" {
		t.Fatal("diff aliases inputs")
	}
	diff.OldKey[0] = "mutated"
	if before.PrimaryKey[0] != "id" {
		t.Fatal("diff aliases keys")
	}
}

func TestDiffOrderAndMultipleChanges(t *testing.T) {
	before, after := model(), model()
	after.Fields[0].Native = "numeric"
	after.Fields[0].Logical = rowmodel.LogicalDecimal
	after.Fields[0].Precision = 22
	after.Fields[0].Scale = 3
	after.Fields[0].Nullable = true
	after.Fields[0], after.Fields[1] = after.Fields[1], after.Fields[0]
	after.PrimaryKey = []string{"name", "id"}
	after.Engine = "other"
	after.DestinationResource = "output"
	diff, err := schema.Compare(before, after)
	if err != nil {
		t.Fatal(err)
	}
	kinds := make([]schema.ChangeKind, len(diff.Changes))
	for i, c := range diff.Changes {
		kinds[i] = c.Kind
	}
	want := []schema.ChangeKind{schema.ChangeType, schema.ChangeNullability, schema.ChangeOrder, schema.ChangeKey, schema.ChangeEngine, schema.ChangeDestination}
	if !reflect.DeepEqual(kinds, want) {
		t.Fatalf("got %v, want %v", kinds, want)
	}
}

func TestDiffInsertionIsNotReorder(t *testing.T) {
	before, after := model(), model()
	after.Fields = append([]rowmodel.Field{{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true}}, after.Fields...)
	diff, err := schema.Compare(before, after)
	if err != nil || len(diff.Changes) != 1 || diff.Changes[0].Kind != schema.AddField {
		t.Fatalf("insertion classified as reorder: %+v, %v", diff, err)
	}
	after.Resource = "other"
	if _, err := schema.Compare(before, after); err == nil {
		t.Fatal("compared different resources")
	}
}

func TestDiffIncludesSemanticFlags(t *testing.T) {
	marked, err := rowmodel.WithEnvelopeFields(model())
	if err != nil {
		t.Fatal(err)
	}
	plain := rowmodel.Schema{Resource: marked.Resource, Engine: marked.Engine, Fields: marked.Fields, PrimaryKey: marked.PrimaryKey}
	diff, err := schema.Compare(plain, marked)
	if err != nil || len(diff.Changes) != 1 || diff.Changes[0].Kind != schema.ChangeSemantics {
		t.Fatalf("lost semantic change: %+v %v", diff, err)
	}
}

func model() rowmodel.Schema {
	return rowmodel.Schema{Resource: "users", Engine: "postgres", Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64, Native: "bigint"},
		{Name: "name", Logical: rowmodel.LogicalString, Native: "text", Nullable: true},
	}, PrimaryKey: []string{"id"}}
}
