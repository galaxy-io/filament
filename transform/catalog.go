package transform

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow/compute"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform/kernel"
)

// argSpec constrains one positional argument of a catalog function. Types
// lists the logical types it accepts; an empty list accepts any. Literal and
// Column say whether the argument must be a constant or a column.
type argSpec struct {
	Name     string
	Types    []rowmodel.LogicalType // accepted logical types; empty accepts any
	Literal  bool                   // must be a literal
	Column   bool                   // must be a column-shaped value
	Optional bool
}

// functionSpec is a catalog function's signature: its arguments and the
// logical type it returns.
type functionSpec struct {
	Name    string
	Args    []argSpec
	Returns rowmodel.LogicalType
}

// argType is what the compiler knows about a compiled expression: its logical
// type, whether it is a literal, and the literal's value when it is one.
type argType struct {
	logical rowmodel.LogicalType
	literal bool
	value   any // set when literal
}

// function is one catalog entry. check is derived from spec; validate adds
// rules that span arguments; exec is the kernel that runs at batch time.
type function struct {
	spec     functionSpec
	validate func(args []argType) error
	exec     func(ctx context.Context, args []compute.Datum) (compute.Datum, error)
}

var (
	stringTypes   = []rowmodel.LogicalType{rowmodel.LogicalString}
	temporalTypes = []rowmodel.LogicalType{rowmodel.LogicalDate, rowmodel.LogicalTimestamp, rowmodel.LogicalTimestampTZ}
)

// catalog is the closed set of functions a definition may call. The names
// are mirrored by the function enum in grammar.v1.json, and a test fails when
// the two drift.
var catalog = map[string]function{
	"lower": {
		spec: functionSpec{
			Name: "lower",
			Args: []argSpec{{Name: "value", Types: stringTypes, Column: true}}, Returns: rowmodel.LogicalString,
		},
		exec: kernel.Lower,
	},
	"trim": {
		spec: functionSpec{
			Name: "trim",
			Args: []argSpec{{Name: "value", Types: stringTypes, Column: true}}, Returns: rowmodel.LogicalString,
		},
		exec: kernel.Trim,
	},
	"eq": {
		spec: functionSpec{
			Name: "eq",
			Args: []argSpec{{Name: "left"}, {Name: "right"}}, Returns: rowmodel.LogicalBool,
		},
		validate: sameType("eq"),
		exec:     kernel.Equal,
	},
	"to_date": {
		spec: functionSpec{
			Name: "to_date",
			Args: []argSpec{
				{Name: "value", Types: stringTypes, Column: true},
				{Name: "layout", Types: stringTypes, Literal: true, Optional: true},
			}, Returns: rowmodel.LogicalDate,
		},
		validate: validLayout("to_date", 1),
		exec:     kernel.ToDate,
	},
	"year": {
		spec: functionSpec{
			Name: "year",
			Args: []argSpec{{Name: "value", Types: temporalTypes, Column: true}}, Returns: rowmodel.LogicalInt64,
		},
		exec: kernel.Year,
	},
	"month": {
		spec: functionSpec{
			Name: "month",
			Args: []argSpec{{Name: "value", Types: temporalTypes, Column: true}}, Returns: rowmodel.LogicalInt64,
		},
		exec: kernel.Month,
	},
	"day": {
		spec: functionSpec{
			Name: "day",
			Args: []argSpec{{Name: "value", Types: temporalTypes, Column: true}}, Returns: rowmodel.LogicalInt64,
		},
		exec: kernel.Day,
	},
}

// check verifies a call's arity, argument kinds, and argument types against
// the signature, then runs the function's own validate hook.
func (fn function) check(args []argType) (rowmodel.LogicalType, error) {
	spec := fn.spec
	required := 0
	for _, a := range spec.Args {
		if !a.Optional {
			required++
		}
	}
	if len(args) < required || len(args) > len(spec.Args) {
		want := fmt.Sprintf("%d", required)
		if required != len(spec.Args) {
			want = fmt.Sprintf("%d to %d", required, len(spec.Args))
		}
		return "", fmt.Errorf("%s expects %s arguments, got %d", spec.Name, want, len(args))
	}
	for i, arg := range args {
		a := spec.Args[i]
		if a.Column && arg.literal {
			return "", fmt.Errorf("%s expects a column for %s, got a literal", spec.Name, a.Name)
		}
		if a.Literal && !arg.literal {
			return "", fmt.Errorf("%s expects a literal for %s, got a column", spec.Name, a.Name)
		}
		if len(a.Types) > 0 && !slices.Contains(a.Types, arg.logical) {
			return "", fmt.Errorf("%s expects %s for %s, got %s", spec.Name, joinTypes(a.Types), a.Name, arg.logical)
		}
	}
	if fn.validate != nil {
		if err := fn.validate(args); err != nil {
			return "", err
		}
	}
	return spec.Returns, nil
}

// joinTypes renders accepted types for an error message.
func joinTypes(types []rowmodel.LogicalType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, " or ")
}

// sameType requires both arguments to share a logical type and at least one
// of them to be a column, so the result always has a column's shape.
func sameType(name string) func([]argType) error {
	return func(args []argType) error {
		if args[0].literal && args[1].literal {
			return fmt.Errorf("%s expects at least one column", name)
		}
		if args[0].logical != args[1].logical {
			return fmt.Errorf("%s expects matching types, got %s and %s", name, args[0].logical, args[1].logical)
		}
		return nil
	}
}

// validLayout checks the Go time layout at index, when present, by formatting
// and re-parsing the current time. A layout that cannot round-trip fails at
// compile time instead of on the first batch.
func validLayout(name string, index int) func([]argType) error {
	return func(args []argType) error {
		if len(args) <= index {
			return nil
		}
		layout, ok := args[index].value.(string)
		if !ok || layout == "" {
			return fmt.Errorf("%s layout must be a non-empty string", name)
		}
		if _, err := time.Parse(layout, time.Now().Format(layout)); err != nil {
			return fmt.Errorf("%s layout %q does not round-trip: %w", name, layout, err)
		}
		return nil
	}
}
