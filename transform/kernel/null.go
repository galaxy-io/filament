package kernel

import (
	"context"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/arrow/scalar"
)

// IsNull is true where the column has no value.
func IsNull(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	return compute.CallFunction(ctx, "is_null", nil, args[0])
}

// Coalesce takes, per row, the first argument that is not null. Arguments
// share one type; a literal is broadcast to the column length.
func Coalesce(ctx context.Context, args []compute.Datum) (compute.Datum, error) {
	n := -1
	for _, a := range args {
		if ad, ok := a.(*compute.ArrayDatum); ok {
			n = int(ad.Len())
			break
		}
	}
	if n < 0 {
		return nil, fmt.Errorf("coalesce: expected at least one column")
	}
	cur, err := asArray(ctx, args[0], n)
	if err != nil {
		return nil, err
	}
	for _, a := range args[1:] {
		if cur.NullN() == 0 {
			break
		}
		next, err := asArray(ctx, a, n)
		if err != nil {
			cur.Release()
			return nil, err
		}
		mask := validityMask(cur)
		merged, err := IfElse(ctx, mask, cur, next)
		mask.Release()
		cur.Release()
		next.Release()
		if err != nil {
			return nil, err
		}
		cur = merged
	}
	defer cur.Release()
	return compute.NewDatum(cur), nil
}

// validityMask views an array's validity bitmap as a boolean array, true
// where the value is present, without copying. The caller releases it.
func validityMask(a arrow.Array) *array.Boolean {
	d := a.Data()
	data := array.NewData(arrow.FixedWidthTypes.Boolean, d.Len(), []*memory.Buffer{nil, d.Buffers()[0]}, nil, 0, d.Offset())
	defer data.Release()
	return array.NewBooleanData(data)
}

// asArray returns a datum as an owned array, broadcasting a scalar to n rows.
func asArray(ctx context.Context, d compute.Datum, n int) (arrow.Array, error) {
	switch v := d.(type) {
	case *compute.ArrayDatum:
		return array.MakeFromData(v.Value), nil
	case *compute.ScalarDatum:
		return scalar.MakeArrayFromScalar(v.Value, n, compute.GetAllocator(ctx))
	}
	return nil, fmt.Errorf("unexpected datum %s", d)
}
