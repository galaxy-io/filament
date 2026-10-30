package kernel

import (
	"context"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
)

// IfElse returns then where mask is true and els everywhere else, including
// where mask is null, which matches SQL CASE. then and els must share a type.
// utf8 has a direct path; every other type goes through concatenate and take.
func IfElse(ctx context.Context, mask *array.Boolean, then, els arrow.Array) (arrow.Array, error) {
	if !arrow.TypeEqual(then.DataType(), els.DataType()) {
		return nil, fmt.Errorf("if_else branches differ: %s vs %s", then.DataType(), els.DataType())
	}
	mem := compute.GetAllocator(ctx)
	n := mask.Len()

	if t, ok := then.(*array.String); ok {
		e := els.(*array.String)
		b := array.NewStringBuilder(mem)
		defer b.Release()
		b.Reserve(n)
		b.ReserveData(max(len(t.ValueBytes()), len(e.ValueBytes())))
		for i := range n {
			src := e
			if mask.IsValid(i) && mask.Value(i) {
				src = t
			}
			if src.IsNull(i) {
				b.AppendNull()
				continue
			}
			b.Append(src.Value(i))
		}
		return b.NewArray(), nil
	}

	// Concatenate both branches, then take row i from then or row n+i from
	// els. Two copies, but it works for every type.
	joined, err := array.Concatenate([]arrow.Array{then, els}, mem)
	if err != nil {
		return nil, err
	}
	defer joined.Release()
	ib := array.NewInt64Builder(mem)
	defer ib.Release()
	ib.Reserve(n)
	for i := range n {
		if mask.IsValid(i) && mask.Value(i) {
			ib.UnsafeAppend(int64(i))
			continue
		}
		ib.UnsafeAppend(int64(n + i))
	}
	idx := ib.NewArray()
	defer idx.Release()
	return compute.TakeArray(ctx, joined, idx)
}
