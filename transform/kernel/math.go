package kernel

import (
	"context"

	"github.com/apache/arrow-go/v18/arrow/compute"
)

// Arithmetic and rounding come from Arrow's compute registry. The compiler
// has already made both sides one type, so no implicit cast happens here.

// Add sums two numbers.
func Add(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "add", nil, args...)
}

// Sub subtracts the second number from the first.
func Sub(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "subtract", nil, args...)
}

// Mul multiplies two numbers.
func Mul(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "multiply", nil, args...)
}

// Div fails the batch on integer division by zero, as Arrow's checked
// divide does.
func Div(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "divide", nil, args...)
}

// Abs is the absolute value.
func Abs(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "abs", nil, args...)
}

// Floor rounds down to a whole number.
func Floor(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "floor", nil, args...)
}

// Ceil rounds up to a whole number.
func Ceil(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "ceil", nil, args...)
}

// Round rounds to the given number of decimal places, halves to even.
func Round(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	opts := compute.DefaultRoundOptions
	if len(args) > 1 {
		places, err := int64Scalar(args[1])
		if err != nil {
			return nil, err
		}
		opts.NDigits = places
	}
	return compute.CallFunction(ctx, "round", &opts, args[0])
}
