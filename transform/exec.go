// This file is the interpreter. The compiler emits ops over nodes, and Apply
// walks them against a frame of Arrow columns. Nothing outside this file
// turns a literal into a scalar, a column into a datum, or calls a kernel.
package transform

import (
	"context"
	"fmt"
	"slices"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
	"github.com/apache/arrow-go/v18/arrow/scalar"

	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform/kernel"
)

// frame is the working column set for one batch. It owns a reference to every
// array in cols and releases them all when the plan is done.
type frame struct {
	rows int
	cols []arrow.Array
}

func (f *frame) release() {
	for _, c := range f.cols {
		c.Release()
	}
}

// op is one compiled step. apply mutates the frame in place.
type op interface {
	apply(ctx context.Context, f *frame) error
}

// dropOp removes the column at idx.
type dropOp struct{ idx int }

func (o dropOp) apply(_ context.Context, f *frame) error {
	f.cols[o.idx].Release()
	f.cols = slices.Delete(f.cols, o.idx, o.idx+1)
	return nil
}

// computeOp evaluates every entry against the frame as it stood before the step,
// then assigns all of them. Under where, each new value is merged with the
// existing column so unmatched rows keep what they had; a new column has
// nothing to keep, so those rows are null.
type computeOp struct {
	where   node
	entries []computeEntry
}

// computeEntry is one assignment in a computeOp.
type computeEntry struct {
	idx  int // -1 appends a new column
	expr node
}

func (o computeOp) apply(ctx context.Context, f *frame) error {
	var mask *array.Boolean
	if o.where != nil {
		m, err := evalArray(ctx, o.where, f)
		if err != nil {
			return err
		}
		defer m.Release()
		mask = m.(*array.Boolean)
	}

	outs := make([]arrow.Array, 0, len(o.entries))
	defer func() {
		for _, a := range outs {
			a.Release()
		}
	}()
	for _, e := range o.entries {
		arr, err := evalArray(ctx, e.expr, f)
		if err != nil {
			return err
		}
		if mask != nil {
			var els arrow.Array
			if e.idx < 0 {
				els = array.MakeArrayOfNull(compute.GetAllocator(ctx), arr.DataType(), f.rows)
			} else {
				els = f.cols[e.idx]
			}
			sel, err := kernel.IfElse(ctx, mask, arr, els)
			arr.Release()
			if e.idx < 0 {
				els.Release()
			}
			if err != nil {
				return err
			}
			arr = sel
		}
		outs = append(outs, arr)
	}

	for i, e := range o.entries {
		if e.idx < 0 {
			f.cols = append(f.cols, outs[i])
			continue
		}
		f.cols[e.idx].Release()
		f.cols[e.idx] = outs[i]
	}
	outs = nil
	return nil
}

// node is a compiled expression. eval returns a datum the caller owns.
type node interface {
	eval(ctx context.Context, f *frame) (compute.Datum, error)
}

// colNode reads the column at idx.
type colNode struct{ idx int }

func (n colNode) eval(_ context.Context, f *frame) (compute.Datum, error) {
	return compute.NewDatum(f.cols[n.idx]), nil
}

type literalNode struct{ sc scalar.Scalar }

// newLiteralNode boxes a literal into an Arrow scalar of the logical type t
// once, at compile time, so no batch pays for it. The compiler has already
// checked that v converts to t.
func newLiteralNode(v any, t rowmodel.LogicalType) literalNode {
	switch x := v.(type) {
	case string:
		return literalNode{sc: scalar.NewStringScalar(x)}
	case bool:
		return literalNode{sc: scalar.NewBooleanScalar(x)}
	case int64:
		switch t {
		case rowmodel.LogicalInt16:
			return literalNode{sc: scalar.NewInt16Scalar(int16(x))} //nolint:gosec // range checked by coercible
		case rowmodel.LogicalInt32:
			return literalNode{sc: scalar.NewInt32Scalar(int32(x))} //nolint:gosec // range checked by coercible
		case rowmodel.LogicalFloat32:
			return literalNode{sc: scalar.NewFloat32Scalar(float32(x))}
		case rowmodel.LogicalFloat64:
			return literalNode{sc: scalar.NewFloat64Scalar(float64(x))}
		}
		return literalNode{sc: scalar.NewInt64Scalar(x)}
	case float64:
		if t == rowmodel.LogicalFloat32 {
			return literalNode{sc: scalar.NewFloat32Scalar(float32(x))}
		}
		return literalNode{sc: scalar.NewFloat64Scalar(x)}
	}
	panic(fmt.Sprintf("transform: literal %T not admitted by Expr", v))
}

func (n literalNode) eval(context.Context, *frame) (compute.Datum, error) {
	return compute.NewDatum(n.sc), nil
}

// callNode applies a catalog function to its evaluated arguments.
type callNode struct {
	fn   function
	args []node
}

func (n callNode) eval(ctx context.Context, f *frame) (compute.Datum, error) {
	args := make([]compute.Datum, 0, len(n.args))
	defer func() {
		for _, a := range args {
			a.Release()
		}
	}()
	for _, a := range n.args {
		d, err := a.eval(ctx, f)
		if err != nil {
			return nil, err
		}
		args = append(args, d)
	}
	return n.fn.exec(ctx, args)
}

// evalArray evaluates n and returns its value as an owned array.
func evalArray(ctx context.Context, n node, f *frame) (arrow.Array, error) {
	d, err := n.eval(ctx, f)
	if err != nil {
		return nil, err
	}
	defer d.Release()
	return toArray(ctx, d, f.rows)
}

// toArray returns a datum as an owned array. A scalar is broadcast to n rows.
func toArray(ctx context.Context, d compute.Datum, n int) (arrow.Array, error) {
	switch v := d.(type) {
	case *compute.ArrayDatum:
		return array.MakeFromData(v.Value), nil
	case *compute.ScalarDatum:
		return scalar.MakeArrayFromScalar(v.Value, n, compute.GetAllocator(ctx))
	}
	return nil, fmt.Errorf("unexpected datum %s", d)
}
