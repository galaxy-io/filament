package transform

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/apache/arrow-go/v18/arrow/compute"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform/kernel"
)

// ArgSpec constrains one positional argument of a catalog function. Types
// lists the logical types it accepts; an empty list accepts any. Literal and
// Column say whether the argument must be a constant or a column. Variadic
// on the last argument lets a call repeat it any number of times.
type ArgSpec struct {
	Name        string
	DisplayName string
	Types       []rowmodel.LogicalType // accepted logical types; empty accepts any
	Literal     bool                   // must be a literal
	Column      bool                   // must be a column-shaped value
	Optional    bool
	Variadic    bool
}

// FunctionSpec is a catalog function's signature: its arguments, the logical
// type it returns, and presentation metadata for a builder to show. An
// OperatorSymbol marks a binary function that can use compact infix UI. A
// SameType function needs every argument to share one type, and at least one
// column among them; a literal is widened to the column's type when it fits.
// ReturnsInput means the result takes that shared type.
type FunctionSpec struct {
	Name              string
	DisplayName       string
	Description       string
	OperatorSymbol    string
	ConditionOperator bool
	ConditionJoin     string
	VariadicAddLabel  string
	Args              []ArgSpec
	Returns           rowmodel.LogicalType
	SameType          bool
	ReturnsInput      bool
}

// valueType is the type of a compiled value: its logical type and, for a
// decimal, the precision and scale that fix its Arrow storage.
type valueType struct {
	logical          rowmodel.LogicalType
	precision, scale int
}

func typeOf(f rowmodel.Field) valueType {
	return valueType{logical: f.Logical, precision: f.Precision, scale: f.Scale}
}

// String spells a bounded decimal with its precision and scale, so a
// mismatch between two decimals reads as one.
func (t valueType) String() string {
	if t.logical == rowmodel.LogicalDecimal && t.precision > 0 {
		return fmt.Sprintf("%s(%d,%d)", t.logical, t.precision, t.scale)
	}
	return string(t.logical)
}

// argType is what the compiler knows about a compiled expression: its type,
// whether it is a literal, and the literal's value when it is one.
type argType struct {
	valueType
	literal bool
	value   any // set when literal
}

// function is one catalog entry. check is derived from spec; validate adds
// rules that span arguments; exec is the kernel that runs at batch time.
type function struct {
	spec     FunctionSpec
	validate func(args []argType) error
	exec     func(ctx context.Context, args []compute.Datum) (compute.Datum, error)
}

var (
	stringTypes    = []rowmodel.LogicalType{rowmodel.LogicalString}
	boolTypes      = []rowmodel.LogicalType{rowmodel.LogicalBool}
	int64Types     = []rowmodel.LogicalType{rowmodel.LogicalInt64}
	floatTypes     = []rowmodel.LogicalType{rowmodel.LogicalFloat32, rowmodel.LogicalFloat64}
	numericTypes   = []rowmodel.LogicalType{rowmodel.LogicalInt16, rowmodel.LogicalInt32, rowmodel.LogicalInt64, rowmodel.LogicalFloat32, rowmodel.LogicalFloat64}
	temporalTypes  = []rowmodel.LogicalType{rowmodel.LogicalDate, rowmodel.LogicalTimestamp, rowmodel.LogicalTimestampTZ}
	orderableTypes = []rowmodel.LogicalType{
		rowmodel.LogicalInt16, rowmodel.LogicalInt32, rowmodel.LogicalInt64,
		rowmodel.LogicalFloat32, rowmodel.LogicalFloat64, rowmodel.LogicalDecimal,
		rowmodel.LogicalDate, rowmodel.LogicalTime, rowmodel.LogicalTimestamp, rowmodel.LogicalTimestampTZ,
	}
)

// catalog is the closed set of functions a definition may call, in the order
// a builder lists them: related functions side by side, sections in the order
// of grammar.v1.json's function enum, which mirrors these names.
var catalog = []function{
	// strings
	unary("lower", "Lowercase", "Lower-cases a string column.", stringTypes, rowmodel.LogicalString, kernel.Lower),
	unary("upper", "Uppercase", "Upper-cases a string column.", stringTypes, rowmodel.LogicalString, kernel.Upper),
	unary("trim", "Trim", "Removes leading and trailing whitespace from a string column.", stringTypes, rowmodel.LogicalString, kernel.Trim),
	unary("length", "Length", "The number of characters in a string column.", stringTypes, rowmodel.LogicalInt64, kernel.Length),
	{
		spec: FunctionSpec{
			Name: "replace", DisplayName: "Replace", Description: "Replaces every occurrence of a substring in a string column.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: stringTypes, Column: true},
				{Name: "find", DisplayName: "Find", Types: stringTypes, Literal: true},
				{Name: "with", DisplayName: "Replace with", Types: stringTypes, Literal: true},
			}, Returns: rowmodel.LogicalString,
		},
		exec: kernel.Replace,
	},
	{
		spec: FunctionSpec{
			Name: "substring", DisplayName: "Substring", Description: "A slice of a string column, counting characters from 1.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: stringTypes, Column: true},
				{Name: "start", DisplayName: "Start position", Types: int64Types, Literal: true},
				{Name: "length", DisplayName: "Length", Types: int64Types, Literal: true, Optional: true},
			}, Returns: rowmodel.LogicalString,
		},
		validate: positive("substring", 1),
		exec:     kernel.Substring,
	},
	{
		spec: FunctionSpec{
			Name: "concat", DisplayName: "Concatenate", Description: "Joins strings end to end; null in any part gives null.",
			Args: []ArgSpec{{Name: "part", DisplayName: "Append", Types: stringTypes, Variadic: true}}, Returns: rowmodel.LogicalString,
			SameType: true, VariadicAddLabel: "Add part",
		},
		exec: kernel.Concat,
	},
	{
		spec: FunctionSpec{
			Name: "regex_match", DisplayName: "Regex match", Description: "True where a string column matches a Go regular expression.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: stringTypes, Column: true},
				{Name: "pattern", DisplayName: "Pattern", Types: stringTypes, Literal: true},
			}, Returns: rowmodel.LogicalBool, ConditionOperator: true,
		},
		validate: validPattern("regex_match", 1),
		exec:     kernel.RegexMatch,
	},
	{
		spec: FunctionSpec{
			Name: "regex_extract", DisplayName: "Regex extract", Description: "The first match of a Go regular expression, or of one of its groups.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: stringTypes, Column: true},
				{Name: "pattern", DisplayName: "Pattern", Types: stringTypes, Literal: true},
				{Name: "group", DisplayName: "Group", Types: int64Types, Literal: true, Optional: true},
			}, Returns: rowmodel.LogicalString,
		},
		validate: validators(validPattern("regex_extract", 1), nonnegative("regex_extract", "group", 2), groupInPattern("regex_extract", 1, 2)),
		exec:     kernel.RegexExtract,
	},
	{
		spec: FunctionSpec{
			Name: "regex_replace", DisplayName: "Regex replace", Description: "Replaces every match of a Go regular expression; $1 refers to a group.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: stringTypes, Column: true},
				{Name: "pattern", DisplayName: "Pattern", Types: stringTypes, Literal: true},
				{Name: "with", DisplayName: "Replace with", Types: stringTypes, Literal: true},
			}, Returns: rowmodel.LogicalString,
		},
		validate: validPattern("regex_replace", 1),
		exec:     kernel.RegexReplace,
	},

	// numbers
	arithmetic("add", "Add", "+", "Adds two numbers.", kernel.Add),
	arithmetic("sub", "Subtract", "−", "Subtracts the second number from the first.", kernel.Sub),
	arithmetic("mul", "Multiply", "×", "Multiplies two numbers.", kernel.Mul),
	arithmetic("div", "Divide", "÷", "Divides the first number by the second; integer inputs divide as integers.", kernel.Div),
	{
		spec: FunctionSpec{
			Name: "abs", DisplayName: "Absolute value", Description: "The absolute value of a number column.",
			Args: []ArgSpec{{Name: "value", DisplayName: "Value", Types: numericTypes, Column: true}}, ReturnsInput: true, SameType: true,
		},
		exec: kernel.Abs,
	},
	unary("floor", "Floor", "Rounds a decimal column down to the nearest whole number.", floatTypes, "", kernel.Floor),
	unary("ceil", "Ceiling", "Rounds a decimal column up to the nearest whole number.", floatTypes, "", kernel.Ceil),
	{
		spec: FunctionSpec{
			Name: "round", DisplayName: "Round", Description: "Rounds a decimal column to a number of places, halves to even.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: floatTypes, Column: true},
				{Name: "places", DisplayName: "Places", Types: int64Types, Literal: true, Optional: true},
			}, ReturnsInput: true, SameType: true,
		},
		exec: kernel.Round,
	},

	// comparisons
	comparison("eq", "Equals", "=", "True where two values are equal.", nil, kernel.Eq),
	comparison("neq", "Not equals", "≠", "True where two values differ.", nil, kernel.Neq),
	comparison("gt", "Greater than", ">", "True where the first orderable value is greater than the second.", orderableTypes, kernel.Gt),
	comparison("gte", "Greater than or equal", "≥", "True where the first orderable value is greater than or equal to the second.", orderableTypes, kernel.Gte),
	comparison("lt", "Less than", "<", "True where the first orderable value is less than the second.", orderableTypes, kernel.Lt),
	comparison("lte", "Less than or equal", "≤", "True where the first orderable value is less than or equal to the second.", orderableTypes, kernel.Lte),

	// logic
	{
		spec: FunctionSpec{
			Name: "and", DisplayName: "And", Description: "True where both conditions are true.",
			Args: []ArgSpec{{Name: "left", DisplayName: "Left", Types: boolTypes}, {Name: "right", DisplayName: "Right", Types: boolTypes}}, Returns: rowmodel.LogicalBool, SameType: true, ConditionJoin: "and",
		},
		exec: kernel.And,
	},
	{
		spec: FunctionSpec{
			Name: "or", DisplayName: "Or", Description: "True where either condition is true.",
			Args: []ArgSpec{{Name: "left", DisplayName: "Left", Types: boolTypes}, {Name: "right", DisplayName: "Right", Types: boolTypes}}, Returns: rowmodel.LogicalBool, SameType: true, ConditionJoin: "or",
		},
		exec: kernel.Or,
	},
	unary("not", "Not", "Flips a condition.", boolTypes, rowmodel.LogicalBool, kernel.Not),

	// nulls
	{
		spec: FunctionSpec{
			Name: "is_null", DisplayName: "Is null", Description: "True where a column has no value.",
			Args: []ArgSpec{{Name: "value", DisplayName: "Value", Column: true}}, Returns: rowmodel.LogicalBool, ConditionOperator: true,
		},
		exec: kernel.IsNull,
	},
	{
		spec: FunctionSpec{
			Name: "coalesce", DisplayName: "Coalesce", Description: "The first value that is not null, left to right.",
			Args: []ArgSpec{{Name: "value", DisplayName: "Value", Variadic: true}}, ReturnsInput: true, SameType: true, VariadicAddLabel: "Add value",
		},
		exec: kernel.Coalesce,
	},

	// dates
	{
		spec: FunctionSpec{
			Name: "to_date", DisplayName: "To date", Description: "Parses a string column of ISO dates (2006-01-02) into a date.",
			Args: []ArgSpec{
				{Name: "value", DisplayName: "Value", Types: stringTypes, Column: true},
			}, Returns: rowmodel.LogicalDate,
		},
		exec: kernel.ToDate,
	},
	unary("year", "Year", "The calendar year of a date or timestamp column.", temporalTypes, rowmodel.LogicalInt64, kernel.Year),
	unary("month", "Month", "The calendar month, 1 to 12, of a date or timestamp column.", temporalTypes, rowmodel.LogicalInt64, kernel.Month),
	unary("day", "Day", "The day of the month of a date or timestamp column.", temporalTypes, rowmodel.LogicalInt64, kernel.Day),
}

// unary builds a one-column function. An empty returns means the input type.
// catalogByName indexes the catalog for the compiler's lookups.
var catalogByName = func() map[string]function {
	byName := make(map[string]function, len(catalog))
	for _, fn := range catalog {
		byName[fn.spec.Name] = fn
	}
	return byName
}()

func unary(name, display, desc string, types []rowmodel.LogicalType, returns rowmodel.LogicalType, exec func(context.Context, []compute.Datum) (compute.Datum, error)) function {
	return function{
		spec: FunctionSpec{
			Name: name, DisplayName: display, Description: desc,
			Args:    []ArgSpec{{Name: "value", DisplayName: "Value", Types: types, Column: true}},
			Returns: returns, ReturnsInput: returns == "", SameType: returns == "",
		},
		exec: exec,
	}
}

// arithmetic builds a two-number function whose result takes the numbers' type.
func arithmetic(name, display, symbol, desc string, exec func(context.Context, []compute.Datum) (compute.Datum, error)) function {
	return function{
		spec: FunctionSpec{
			Name: name, DisplayName: display, Description: desc, OperatorSymbol: symbol,
			Args:         []ArgSpec{{Name: "left", DisplayName: "Left", Types: numericTypes}, {Name: "right", DisplayName: "Right", Types: numericTypes}},
			ReturnsInput: true, SameType: true,
		},
		exec: exec,
	}
}

// comparison builds a two-value predicate over a shared type. A nil types
// admits every type; ordering comparisons pass the types that order.
func comparison(name, display, symbol, desc string, types []rowmodel.LogicalType, exec func(context.Context, []compute.Datum) (compute.Datum, error)) function {
	return function{
		spec: FunctionSpec{
			Name: name, DisplayName: display, Description: desc, OperatorSymbol: symbol, ConditionOperator: true,
			Args: []ArgSpec{{Name: "left", DisplayName: "Left", Types: types}, {Name: "right", DisplayName: "Compare with", Types: types}}, Returns: rowmodel.LogicalBool, SameType: true,
		},
		exec: exec,
	}
}

// argSpec returns the spec for argument i, repeating a variadic last spec.
func (s FunctionSpec) argSpec(i int) ArgSpec {
	if i < len(s.Args) {
		return s.Args[i]
	}
	return s.Args[len(s.Args)-1]
}

// check verifies a call's arity, argument kinds, and argument types against
// the signature, then runs the function's own validate hook. It returns the
// call's result type. A failure one argument is responsible for comes back
// as an argError naming it.
func (fn function) check(args []argType) (valueType, error) {
	spec := fn.spec
	required := 0
	for _, a := range spec.Args {
		if !a.Optional && !a.Variadic {
			required++
		}
	}
	variadic := len(spec.Args) > 0 && spec.Args[len(spec.Args)-1].Variadic
	if variadic {
		required++
	}
	if len(args) < required || (!variadic && len(args) > len(spec.Args)) {
		want := fmt.Sprintf("%d", required)
		switch {
		case variadic:
			want = fmt.Sprintf("at least %d", required)
		case required != len(spec.Args):
			want = fmt.Sprintf("%d to %d", required, len(spec.Args))
		}
		return valueType{}, fmt.Errorf("%s expects %s arguments, got %d", spec.Name, want, len(args))
	}
	for i, arg := range args {
		a := spec.argSpec(i)
		if a.Column && arg.literal {
			return valueType{}, argErrorf(i, "%s expects a column for %s, got a literal", spec.Name, a.Name)
		}
		if a.Literal && !arg.literal {
			return valueType{}, argErrorf(i, "%s expects a literal for %s, got a column", spec.Name, a.Name)
		}
		if len(a.Types) > 0 && !slices.Contains(a.Types, arg.logical) {
			return valueType{}, argErrorf(i, "%s expects %s for %s, got %s", spec.Name, joinTypes(a.Types), a.Name, arg.logical)
		}
	}
	var shared valueType
	if spec.SameType {
		var err error
		if shared, err = sharedType(spec, args); err != nil {
			return valueType{}, err
		}
	}
	if fn.validate != nil {
		if err := fn.validate(args); err != nil {
			return valueType{}, err
		}
	}
	if spec.ReturnsInput {
		return shared, nil
	}
	return valueType{logical: spec.Returns}, nil
}

// sharedType requires every value argument to share one type, precision and
// scale included, and at least one of them to be a column, so the result
// always has a column's shape. Parameters the spec marks Literal, such as a
// count of places, are not values and stay out of it.
func sharedType(spec FunctionSpec, args []argType) (valueType, error) {
	var shared valueType
	for i, a := range args {
		if !a.literal && !spec.argSpec(i).Literal {
			shared = a.valueType
			break
		}
	}
	if shared.logical == rowmodel.LogicalUnknown {
		return valueType{}, fmt.Errorf("%s expects at least one column", spec.Name)
	}
	for i, a := range args {
		if spec.argSpec(i).Literal {
			continue
		}
		if a.valueType != shared {
			return valueType{}, argErrorf(i, "%s expects matching types, got %s and %s", spec.Name, shared, a.valueType)
		}
	}
	return shared, nil
}

// joinTypes renders accepted types for an error message.
func joinTypes(types []rowmodel.LogicalType) string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = string(t)
	}
	return strings.Join(names, " or ")
}

// validPattern compiles the regular expression at index so a bad pattern
// fails at compile time. The kernel compiles it again per batch from a cache.
func validPattern(name string, index int) func([]argType) error {
	return func(args []argType) error {
		pattern, _ := args[index].value.(string)
		if _, err := regexp.Compile(pattern); err != nil {
			return argErrorf(index, "%s pattern: %w", name, err)
		}
		return nil
	}
}

// groupInPattern requires the optional group literal at index to name a
// group the pattern at patternIndex has. validPattern has already run, so
// the pattern compiles.
func groupInPattern(name string, patternIndex, index int) func([]argType) error {
	return func(args []argType) error {
		if len(args) <= index {
			return nil
		}
		pattern, _ := args[patternIndex].value.(string)
		groups := regexp.MustCompile(pattern).NumSubexp()
		if v, _ := args[index].value.(int64); int(v) > groups {
			return argErrorf(index, "%s pattern has %d groups, asked for %d", name, groups, v)
		}
		return nil
	}
}

// positive requires the literal at index to be one or more.
func positive(name string, index int) func([]argType) error {
	return func(args []argType) error {
		if len(args) <= index {
			return nil
		}
		if v, _ := args[index].value.(int64); v < 1 {
			return argErrorf(index, "%s %s must be 1 or more, got %d", name, "start", v)
		}
		return nil
	}
}

// nonnegative requires an optional integer literal to be zero or more.
func nonnegative(name, argument string, index int) func([]argType) error {
	return func(args []argType) error {
		if len(args) <= index {
			return nil
		}
		if v, _ := args[index].value.(int64); v < 0 {
			return argErrorf(index, "%s %s must be zero or more, got %d", name, argument, v)
		}
		return nil
	}
}

// validators composes function-specific checks while keeping the catalog's
// public constraint metadata independent from the compiler implementation.
func validators(checks ...func([]argType) error) func([]argType) error {
	return func(args []argType) error {
		for _, check := range checks {
			if err := check(args); err != nil {
				return err
			}
		}
		return nil
	}
}
