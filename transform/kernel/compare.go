package kernel

import (
	"cmp"
	"context"
	"fmt"
	"math/big"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
)

// Op is a comparison operator.
type Op uint8

// Comparison operators, named after the yaml grammar.
const (
	Eq Op = iota
	Neq
	Gt
	Gte
	Lt
	Lte
)

// arrowFunction is the compute registry function for each operator.
var arrowFunction = [...]string{Eq: "equal", Neq: "not_equal", Gt: "greater", Gte: "greater_equal", Lt: "less", Lte: "less_equal"}

// Compare is Arrow's comparison for op. A null on either side compares to null.
func Compare(op Op) func(context.Context, []compute.Datum) (compute.Datum, error) {
	name := arrowFunction[op]
	return func(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
		return compute.CallFunction(ctx, name, nil, args...)
	}
}

// CompareDecimalText is the comparison for decimals arrowbatch carries as
// utf8 rather than decimal128. Both sides are read as exact numbers, so
// "1.0" equals "1.00" and "10" is greater than "9". A null on either side
// compares to null. NaN is above every number and equal to itself, and
// Infinity and -Infinity sit at the ends, as PostgreSQL orders numeric.
func CompareDecimalText(op Op) func(context.Context, []compute.Datum) (compute.Datum, error) {
	return func(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
		if len(args) != 2 {
			return nil, fmt.Errorf("expected two columns, got %d", len(args))
		}
		left, err := textArray(args[0])
		if err != nil {
			return nil, err
		}
		defer left.Release()
		right, err := textArray(args[1])
		if err != nil {
			return nil, err
		}
		defer right.Release()

		b := array.NewBooleanBuilder(compute.GetAllocator(ctx))
		defer b.Release()
		b.Reserve(left.Len())
		var l, r decimalText
		for i := range left.Len() {
			if left.IsNull(i) || right.IsNull(i) {
				b.AppendNull()
				continue
			}
			if err := l.set(left.Value(i)); err != nil {
				return nil, fmt.Errorf("row %d: %w", i, err)
			}
			if err := r.set(right.Value(i)); err != nil {
				return nil, fmt.Errorf("row %d: %w", i, err)
			}
			b.Append(holds(op, l.cmp(&r)))
		}
		out := b.NewArray()
		defer out.Release()
		return compute.NewDatum(out), nil
	}
}

func textArray(d compute.Datum) (*array.String, error) {
	ad, ok := d.(*compute.ArrayDatum)
	if !ok {
		return nil, fmt.Errorf("expected a column, got %s", d)
	}
	a, ok := array.MakeFromData(ad.Value).(*array.String)
	if !ok {
		return nil, fmt.Errorf("expected a utf8 array, got %s", ad.Type())
	}
	return a, nil
}

// decimalText is one parsed value: a number, or one of the specials, which
// order below and above every number.
type decimalText struct {
	class int8 // -1 -Infinity, 0 number, 1 Infinity, 2 NaN
	num   big.Rat
}

func (d *decimalText) set(s string) error {
	switch s {
	case "-Infinity":
		d.class = -1
	case "Infinity":
		d.class = 1
	case "NaN":
		d.class = 2
	default:
		// SetString still refuses an exponent it cannot hold.
		if !decimalSyntax(s) {
			return fmt.Errorf("%q is not a decimal", s)
		}
		if _, ok := d.num.SetString(s); !ok {
			return fmt.Errorf("%q is not a decimal", s)
		}
		d.class = 0
	}
	return nil
}

// decimalSyntax accepts a signed base-10 number with an optional fraction and
// exponent. big.Rat.SetString alone would also take fractions, hex and
// digit separators, which no source emits as a decimal.
func decimalSyntax(s string) bool {
	i, digits := 0, 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i, digits = i+1, digits+1
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i, digits = i+1, digits+1
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i == len(s) {
			return false
		}
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
	}
	return i == len(s)
}

func (d *decimalText) cmp(o *decimalText) int {
	if d.class != o.class || d.class != 0 {
		return cmp.Compare(d.class, o.class)
	}
	return d.num.Cmp(&o.num)
}

func holds(op Op, c int) bool {
	switch op {
	case Eq:
		return c == 0
	case Neq:
		return c != 0
	case Gt:
		return c > 0
	case Gte:
		return c >= 0
	case Lt:
		return c < 0
	default:
		return c <= 0
	}
}
