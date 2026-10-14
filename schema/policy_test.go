package schema_test

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
)

func TestPolicy(t *testing.T) {
	for _, tc := range []struct {
		name    string
		change  func(*rowmodel.Schema)
		allowed bool
	}{
		{"unchanged", func(*rowmodel.Schema) {}, true},
		{"nullable addition", func(s *rowmodel.Schema) {
			s.Fields = append(s.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true})
		}, true},
		{"required addition", func(s *rowmodel.Schema) {
			s.Fields = append(s.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString})
		}, false},
		{"unknown addition", func(s *rowmodel.Schema) { s.Fields = append(s.Fields, rowmodel.Field{Name: "extra", Nullable: true}) }, false},
		{"drop", func(s *rowmodel.Schema) { s.Fields = s.Fields[:1] }, false},
		{"rename", func(s *rowmodel.Schema) { s.Fields[1].Name = "renamed" }, false},
		{"type", func(s *rowmodel.Schema) { s.Fields[1].Logical = rowmodel.LogicalBytes }, false},
		{"nullability", func(s *rowmodel.Schema) { s.Fields[1].Nullable = false }, false},
		{"key", func(s *rowmodel.Schema) { s.PrimaryKey = nil }, false},
		{"addition with drop", func(s *rowmodel.Schema) {
			s.Fields[1] = rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, after := model(), model()
			tc.change(&after)
			diff, err := schema.Compare(before, after)
			if err != nil {
				t.Fatal(err)
			}
			for _, p := range []schema.Policy{schema.Strict, schema.NullableAdd} {
				err := p.Check(after.Resource, diff)
				allowed := tc.allowed && (p == schema.NullableAdd || len(diff.Changes) == 0)
				if allowed && err != nil || !allowed && !errors.Is(err, schema.ErrBlocked) {
					t.Fatalf("policy %s: %v", p, err)
				}
			}
		})
	}
	if err := schema.Policy("typo").Check("users", schema.Diff{}); err == nil {
		t.Fatal("unknown policy admitted")
	}
}

func TestCompareRejectsInvalidSchemas(t *testing.T) {
	for _, change := range []func(*rowmodel.Schema){
		func(s *rowmodel.Schema) { s.Fields[1].Name = "id" },
		func(s *rowmodel.Schema) { s.PrimaryKey = []string{"missing"} },
		func(s *rowmodel.Schema) { s.Fields[1].Name = "" },
	} {
		s := model()
		change(&s)
		if _, err := schema.Compare(model(), s); err == nil {
			t.Fatal("invalid schema admitted")
		}
	}
}
