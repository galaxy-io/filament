package schema

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/galaxy-io/filament/rowmodel"
)

// MessageLayout is the accepted payload decoder layout. Transport metadata is
// source-owned and reconstructed separately. Epoch certificates persist it with
// progress so missing fields on replay cannot redefine a resource's schema.
type MessageLayout struct {
	Raw    bool
	Fields []rowmodel.Field
}

// Clone returns an independent decoder layout.
func (m MessageLayout) Clone() MessageLayout { m.Fields = slices.Clone(m.Fields); return m }

// Validate accepts the scalar/JSON inference contract used by message sources.
// Null-only values use JSON, never an assumed string or non-nullable type.
func (m MessageLayout) Validate() error {
	if err := Validate(rowmodel.Schema{Fields: m.Fields}); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, f := range m.Fields {
		name := strings.ToLower(f.Name)
		if seen[name] || strings.HasPrefix(name, "_filament_") || !f.Nullable || f.Native != "" || f.Precision != 0 || f.Scale != 0 {
			return fmt.Errorf("invalid message field %q", f.Name)
		}
		seen[name] = true
		if m.Raw {
			if len(m.Fields) != 1 || f.Name != "payload" || f.Logical != rowmodel.LogicalBytes {
				return fmt.Errorf("invalid raw message layout")
			}
		} else if f.Logical != rowmodel.LogicalString && f.Logical != rowmodel.LogicalBool && f.Logical != rowmodel.LogicalJSON {
			return fmt.Errorf("unsupported message type %q", f.Logical)
		}
	}
	if m.Raw && len(m.Fields) != 1 {
		return fmt.Errorf("raw messages require a payload field")
	}
	return nil
}

// CheckMessageChange permits only changes authorized by policy, preserving the
// meaning of all previously decoded fields and the raw/object representation.
func CheckMessageChange(resource string, before, after MessageLayout, policy Policy) error {
	if err := before.Validate(); err != nil {
		return err
	}
	if err := after.Validate(); err != nil {
		return err
	}
	if before.Raw != after.Raw {
		return &BlockError{Resource: resource, Reasons: []string{"raw/object message representation changed"}}
	}
	diff, err := Compare(rowmodel.Schema{Resource: resource, Fields: before.Fields}, rowmodel.Schema{Resource: resource, Fields: after.Fields})
	if err != nil {
		return err
	}
	return policy.Check(resource, diff)
}

// CloneMessageLayouts owns both the resource map and its field slices.
func CloneMessageLayouts(layouts map[string]MessageLayout) map[string]MessageLayout {
	out := maps.Clone(layouts)
	for resource, layout := range out {
		out[resource] = layout.Clone()
	}
	return out
}

// CheckMessageLayouts rejects lost resources or incompatible decoder changes
// between committed epochs. Execution also applies the configured policy.
func CheckMessageLayouts(before, after map[string]MessageLayout) error {
	for resource, layout := range after {
		if resource == "" || !utf8.ValidString(resource) {
			return fmt.Errorf("message layout requires a resource")
		}
		if err := layout.Validate(); err != nil {
			return err
		}
	}
	for resource, old := range before {
		next, ok := after[resource]
		if !ok {
			return &BlockError{Resource: resource, Reasons: []string{"accepted message layout is missing"}}
		}
		if err := CheckMessageChange(resource, old, next, NullableAdd); err != nil {
			return err
		}
	}
	return nil
}
