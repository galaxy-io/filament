package kernel

import (
	"context"

	"github.com/apache/arrow-go/v18/arrow/compute"
)

// Equal compares two values for equality using arrow-go's compare kernel.
// Either side may be an array or a scalar.
func Equal(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "equal", nil, args...)
}
