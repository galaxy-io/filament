package kernel

import (
	"context"

	"github.com/apache/arrow-go/v18/arrow/compute"
)

// Comparisons come from Arrow's compute registry. Names follow the yaml
// grammar, so eq is Eq. A null on either side compares to null.

// Eq is true where the two values are equal.
func Eq(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "equal", nil, args...)
}

// Neq is true where the two values differ.
func Neq(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "not_equal", nil, args...)
}

// Gt is true where the first value is greater.
func Gt(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "greater", nil, args...)
}

// Gte is true where the first value is greater or equal.
func Gte(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "greater_equal", nil, args...)
}

// Lt is true where the first value is less.
func Lt(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "less", nil, args...)
}

// Lte is true where the first value is less or equal.
func Lte(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "less_equal", nil, args...)
}
