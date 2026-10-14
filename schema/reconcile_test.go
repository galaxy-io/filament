package schema

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

type testTarget struct {
	model          rowmodel.Schema
	added          [][]rowmodel.Field
	failAfterFirst bool
	ignore         bool
}

func (t *testTarget) Inspect(context.Context) (rowmodel.Schema, error) { return t.model.Clone(), nil }

func (t *testTarget) AddNullable(_ context.Context, fields []rowmodel.Field) error {
	t.added = append(t.added, fields)
	if t.ignore {
		return nil
	}
	for _, f := range fields {
		t.model.Fields = append(t.model.Fields, f)
		if t.failAfterFirst {
			return errors.New("lost DDL response")
		}
	}
	return nil
}

func TestReconcileWholeDiffAndVerification(t *testing.T) {
	original := rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}}
	for _, mode := range []string{"mixed", "strict", "nullable", "ignored DDL"} {
		t.Run(mode, func(t *testing.T) {
			target := &testTarget{model: original.Clone(), ignore: mode == "ignored DDL"}
			desired := original.Clone()
			desired.Fields = append(desired.Fields, rowmodel.Field{Name: "extra", Logical: rowmodel.LogicalString, Nullable: true}, rowmodel.Field{Name: "other", Logical: rowmodel.LogicalBool, Nullable: true})
			policy := NullableAdd
			if mode == "strict" {
				policy = Strict
			}
			if mode == "mixed" {
				desired.Fields[0].Logical = rowmodel.LogicalString
			}
			err := Reconcile(t.Context(), target, desired, policy)
			if mode == "nullable" {
				if err != nil || len(target.added) != 1 || len(target.added[0]) != 2 {
					t.Fatalf("batch additions: %+v %v", target.added, err)
				}
			} else if !errors.Is(err, ErrBlocked) {
				t.Fatalf("unsafe reconciliation: %v", err)
			}
			if (mode == "mixed" || mode == "strict") && len(target.added) != 0 {
				t.Fatal("DDL before complete validation")
			}
		})
	}
}

func TestReconcileRetryAfterCommittedAddition(t *testing.T) {
	target := &testTarget{model: rowmodel.Schema{Resource: "items"}, failAfterFirst: true}
	desired := rowmodel.Schema{Resource: "items", Fields: []rowmodel.Field{{Name: "a", Logical: rowmodel.LogicalString, Nullable: true}, {Name: "b", Logical: rowmodel.LogicalBool, Nullable: true}}}
	if err := Reconcile(t.Context(), target, desired, NullableAdd); err == nil {
		t.Fatal("expected failure after committed addition")
	}
	target.failAfterFirst = false
	if err := Reconcile(t.Context(), target, desired, NullableAdd); err != nil {
		t.Fatal(err)
	}
	if len(target.added) != 2 || len(target.added[1]) != 1 || target.added[1][0].Name != "b" {
		t.Fatalf("retry did not inspect actual state: %+v", target.added)
	}
	// A repeated reconciliation performs no DDL and physical order is irrelevant.
	desired.Fields[0], desired.Fields[1] = desired.Fields[1], desired.Fields[0]
	if err := Reconcile(t.Context(), target, desired, Strict); err != nil {
		t.Fatal(err)
	}
	if len(target.added) != 2 {
		t.Fatal("repeated DDL")
	}
}
