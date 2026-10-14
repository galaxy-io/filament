package kernel

import (
	"context"

	"github.com/apache/arrow-go/v18/arrow/compute"
)

// And and Or use Kleene logic, as SQL does: false and null is false, true or
// null is true, and null otherwise.

// And is true where both are true.
func And(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "and_kleene", nil, args...)
}

// Or is true where either is true.
func Or(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "or_kleene", nil, args...)
}

// Not flips a boolean column.
func Not(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "not", nil, args...)
}
