package transform

import (
	"context"
	"fmt"
	"slices"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Plan is a compiled transform for one resource: the schema it produces and
// the ordered column operations that produce it. A Plan never changes after
// Compile returns and may be applied from many goroutines at once.
type Plan struct {
	in       rowmodel.Schema
	out      rowmodel.Schema
	arrowOut *arrow.Schema
	ops      []op
}

// newPlan binds the compiler's output to the Arrow schema Apply will emit.
func newPlan(in, out rowmodel.Schema, ops []op) *Plan {
	return &Plan{in: in.Clone(), out: out, arrowOut: arrowbatch.Schema(out), ops: ops}
}

// Schema is the schema of every record Apply returns.
func (p *Plan) Schema() rowmodel.Schema { return p.out.Clone() }

// Apply runs the plan over rec and returns a new record the caller owns. The
// leading columns of rec must match the schema the plan was compiled against.
// Any columns after them pass through unchanged and land after the plan's
// output, so a caller may append its own fields to a batch without the plan
// knowing. Columns the plan does not touch are shared with rec, not copied.
func (p *Plan) Apply(ctx context.Context, mem memory.Allocator, rec arrow.RecordBatch) (arrow.RecordBatch, error) {
	if err := p.checkInput(rec.Schema()); err != nil {
		return nil, err
	}
	if mem == nil {
		mem = memory.DefaultAllocator
	}
	ctx = compute.WithAllocator(ctx, mem)

	// Pass-through columns stay out of the frame so a step that adds a column
	// lands before them, in the order outputSchema promises.
	n := len(p.in.Fields)
	f := frame{rows: int(rec.NumRows()), cols: slices.Clone(rec.Columns()[:n])}
	for _, c := range f.cols {
		c.Retain()
	}
	defer f.release()

	for _, o := range p.ops {
		if err := o.apply(ctx, &f); err != nil {
			return nil, fmt.Errorf("transform: %w", err)
		}
	}
	cols := append(slices.Clone(f.cols), rec.Columns()[n:]...)
	return array.NewRecordBatch(p.outputSchema(rec.Schema()), cols, rec.NumRows()), nil
}

// outputSchema is the plan's output schema followed by any pass-through
// fields taken from in.
func (p *Plan) outputSchema(in *arrow.Schema) *arrow.Schema {
	n := len(p.in.Fields)
	if in.NumFields() == n {
		return p.arrowOut
	}
	fields := append(slices.Clone(p.arrowOut.Fields()), in.Fields()[n:]...)
	return arrow.NewSchema(fields, nil)
}

// checkInput confirms the record was built from the compiled schema, by
// column count and by name at each position.
func (p *Plan) checkInput(s *arrow.Schema) error {
	if s.NumFields() < len(p.in.Fields) {
		return fmt.Errorf("transform: record has %d columns, plan expects at least %d", s.NumFields(), len(p.in.Fields))
	}
	for i, f := range p.in.Fields {
		if got := s.Field(i).Name; got != f.Name {
			return fmt.Errorf("transform: column %d is %q, plan expects %q", i, got, f.Name)
		}
	}
	return nil
}
