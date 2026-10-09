package schema

import (
	"fmt"
	"slices"

	"github.com/galaxy-io/filament/rowmodel"
)

// ChangeKind classifies one structural difference.
type ChangeKind string

// Structural change categories. A rename is a drop and an addition.
const (
	AddField          ChangeKind = "add_field"
	DropField         ChangeKind = "drop_field"
	ChangeType        ChangeKind = "type"
	ChangeNullability ChangeKind = "nullability"
	ChangeKey         ChangeKind = "key"
	ChangeOrder       ChangeKind = "order"
	ChangeEngine      ChangeKind = "engine"
	ChangeDestination ChangeKind = "destination"
	ChangeSemantics   ChangeKind = "semantics"
)

// Change owns its before/after values independently of the compared schemas.
// Renames are deliberately represented as drop/add until stable identities or
// an explicit mapping provide evidence of a rename.
type Change struct {
	Kind          ChangeKind
	Field         string
	Before, After *rowmodel.Field
}

// Diff contains all differences and the compared primary keys.
type Diff struct {
	Changes        []Change
	OldKey, NewKey []string
}

// Compare compares complete models of the same resource. Both inputs must be complete;
// absence in a sample is not evidence that a field was dropped. Changes are ordered
// by old fields, then additions by new fields, followed by resource-level changes.
func Compare(before, after rowmodel.Schema) (Diff, error) {
	if err := Validate(before); err != nil {
		return Diff{}, err
	}
	if err := Validate(after); err != nil {
		return Diff{}, err
	}
	if before.Resource != after.Resource {
		return Diff{}, fmt.Errorf("cannot compare different resources %q and %q", before.Resource, after.Resource)
	}
	d := Diff{OldKey: slices.Clone(before.PrimaryKey), NewKey: slices.Clone(after.PrimaryKey)}
	old, next := map[string]rowmodel.Field{}, map[string]rowmodel.Field{}
	for _, f := range before.Fields {
		old[f.Name] = f
	}
	for _, f := range after.Fields {
		next[f.Name] = f
	}
	var oldOrder, newOrder []string
	for _, f := range before.Fields {
		n, ok := next[f.Name]
		if !ok {
			d.Changes = append(d.Changes, Change{Kind: DropField, Field: f.Name, Before: &f})
			continue
		}
		oldOrder = append(oldOrder, f.Name)
		if f.Logical != n.Logical || f.Native != n.Native || f.Precision != n.Precision || f.Scale != n.Scale {
			d.Changes = append(d.Changes, Change{ChangeType, f.Name, &f, &n})
		}
		if f.Nullable != n.Nullable {
			d.Changes = append(d.Changes, Change{ChangeNullability, f.Name, &f, &n})
		}
	}
	for _, f := range after.Fields {
		if _, ok := old[f.Name]; !ok {
			d.Changes = append(d.Changes, Change{Kind: AddField, Field: f.Name, After: &f})
		} else {
			newOrder = append(newOrder, f.Name)
		}
	}
	if !slices.Equal(oldOrder, newOrder) {
		d.Changes = append(d.Changes, Change{Kind: ChangeOrder})
	}
	if !slices.Equal(before.PrimaryKey, after.PrimaryKey) {
		d.Changes = append(d.Changes, Change{Kind: ChangeKey})
	}
	if before.Engine != after.Engine {
		d.Changes = append(d.Changes, Change{Kind: ChangeEngine})
	}
	if before.DestinationResource != after.DestinationResource {
		d.Changes = append(d.Changes, Change{Kind: ChangeDestination})
	}
	a := before
	a.Fields, a.PrimaryKey = after.Fields, after.PrimaryKey
	a.Engine, a.DestinationResource = after.Engine, after.DestinationResource
	if !a.Equal(after) {
		d.Changes = append(d.Changes, Change{Kind: ChangeSemantics})
	}
	return d, nil
}
