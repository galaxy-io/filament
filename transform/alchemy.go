package transform

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// FunctionRef pins one version of an alchemy function.
type FunctionRef struct {
	ID      string
	Version int64
}

func (r FunctionRef) String() string { return fmt.Sprintf("%s@v%d", r.ID, r.Version) }

// Contract is a function version's Arrow input and output schemas.
type Contract struct {
	Input  *arrow.Schema
	Output *arrow.Schema
}

// Contracts looks up function contracts; a lookup error becomes a compile issue on the step.
type Contracts interface {
	Contract(ref FunctionRef) (Contract, error)
}

// ContractMap is a fixed set of contracts.
type ContractMap map[FunctionRef]Contract

// Contract returns ref's contract, or an error naming it when the set does not hold it.
func (m ContractMap) Contract(ref FunctionRef) (Contract, error) {
	c, ok := m[ref]
	if !ok {
		return Contract{}, fmt.Errorf("function %s is not available", ref)
	}
	return c, nil
}

// Call identifies one alchemy step invocation; Step is the step's definition path.
type Call struct {
	Step     string
	Function FunctionRef
}

// Invoker runs a function over a batch and returns a row-aligned batch the caller owns.
type Invoker interface {
	Invoke(ctx context.Context, call Call, in arrow.RecordBatch) (arrow.RecordBatch, error)
}

// FunctionService is the service alchemy steps resolve and run on, acting as one tenant.
type FunctionService interface {
	// Resolve fetches refs' contracts; Contract reports why a ref cannot run.
	Resolve(ctx context.Context, tenant filament.TenantID, refs []FunctionRef) Contracts
	// Invoke runs call over in; the caller releases Result.Record.
	Invoke(ctx context.Context, tenant filament.TenantID, call Call, in arrow.RecordBatch) (Result, error)
}

// Result is a function's row-aligned output and every remote call that produced it.
type Result struct {
	Record      arrow.RecordBatch
	Invocations []Invocation
}

// Invocation is one remote call; an input over the payload cap is split across several.
type Invocation struct {
	ID       string
	Records  int64
	Attempts int32
	Replayed bool
	Duration time.Duration
}

// FunctionError is a function that ran and failed.
type FunctionError struct {
	InvocationID string
	Kind         string
	Message      string
}

func (e *FunctionError) Error() string {
	return fmt.Sprintf("function failed (%s, invocation %s): %s", e.Kind, e.InvocationID, e.Message)
}

// Option configures Compile and Analyze.
type Option func(*compiler)

// WithContracts supplies the contracts alchemy steps are checked against.
func WithContracts(c Contracts) Option { return func(cc *compiler) { cc.contracts = c } }

// WithInvoker binds the invoker alchemy steps call at run time.
func WithInvoker(inv Invoker) Option { return func(cc *compiler) { cc.invoker = inv } }

// Functions lists the distinct function versions the alchemy steps of resources reference, or of all of them when none are named.
func (d *Definition) Functions(resources ...string) []FunctionRef {
	var refs []FunctionRef
	for name, res := range d.Resources {
		if len(resources) > 0 && !slices.Contains(resources, name) {
			continue
		}
		for _, st := range res.Steps {
			if st.Alchemy != nil {
				refs = append(refs, FunctionRef{ID: st.Alchemy.Function, Version: st.Alchemy.Version})
			}
		}
	}
	slices.SortFunc(refs, func(a, b FunctionRef) int {
		return cmp.Or(cmp.Compare(a.ID, b.ID), cmp.Compare(a.Version, b.Version))
	})
	return slices.Compact(refs)
}

// alchemy checks input against the layout and makes the output the new layout; keys must pass through unchanged.
func (c *compiler) alchemy(path string, st *AlchemyStep) {
	ref := FunctionRef{ID: st.Function, Version: st.Version}
	if c.contracts == nil {
		c.errs.addf(path+".function", "alchemy functions are not configured")
		return
	}
	contract, err := c.contracts.Contract(ref)
	if err != nil {
		c.errs.addf(path+".function", "%v", err)
		return
	}
	in, inIdx, inOK := c.alchemyInput(path, contract.Input)
	out, outIdx, outKeys, outOK := c.alchemyOutput(path, contract.Output)
	if !inOK || !outOK {
		return
	}
	c.ops = append(c.ops, alchemyOp{
		call:    Call{Step: path, Function: ref},
		invoker: c.invoker,
		in:      in,
		inIdx:   inIdx,
		out:     out,
		outIdx:  outIdx,
		outKeys: outKeys,
	})
}

// alchemyInput resolves the contract's input to layout indices and the schema sent on the wire.
func (c *compiler) alchemyInput(path string, schema *arrow.Schema) (*arrow.Schema, []int, bool) {
	ok := true
	fields := make([]arrow.Field, 0, schema.NumFields())
	idx := make([]int, 0, schema.NumFields())
	seen := make(map[string]bool, schema.NumFields())
	for _, f := range schema.Fields() {
		if seen[f.Name] {
			c.errs.addf(path, "function input names %q more than once", f.Name)
			ok = false
			continue
		}
		seen[f.Name] = true
		i, found := c.index(f.Name)
		if !found {
			c.errs.addf(path, "function input %q is not a column of the resource", f.Name)
			ok = false
			continue
		}
		col := c.layout.Fields[i]
		// Strict equality matches alchemy: filament layouts carry no list types, whose child names it relaxes.
		if t := arrowbatch.Type(col); !arrow.TypeEqual(t, f.Type) {
			c.errs.addf(path, "column %q is %s, but the function input is %s", f.Name, t, f.Type)
			ok = false
			continue
		}
		if col.Nullable && !f.Nullable {
			c.errs.addf(path, "column %q is nullable, but the function's input does not accept nulls", f.Name)
			ok = false
			continue
		}
		fields = append(fields, arrow.Field{Name: f.Name, Type: f.Type, Nullable: f.Nullable})
		idx = append(idx, i)
	}
	return arrow.NewSchema(fields, nil), idx, ok
}

// alchemyOutput makes the contract's output the layout; keys must be present and are checked, not replaced.
func (c *compiler) alchemyOutput(path string, schema *arrow.Schema) (*arrow.Schema, []int, []bool, bool) {
	ok := true
	fields := make([]arrow.Field, 0, schema.NumFields())
	idx := make([]int, 0, schema.NumFields())
	keys := make([]bool, 0, schema.NumFields())
	layout := make([]rowmodel.Field, 0, schema.NumFields())
	seen := make(map[string]bool, schema.NumFields())
	for _, f := range schema.Fields() {
		if seen[f.Name] {
			c.errs.addf(path, "function output names %q more than once", f.Name)
			ok = false
			continue
		}
		seen[f.Name] = true
		if rowmodel.IsReservedColumn(f.Name) {
			c.errs.addf(path, "function output %q is a reserved column", f.Name)
			ok = false
			continue
		}
		rf, err := arrowbatch.FieldOf(f)
		if err != nil {
			c.errs.addf(path, "function output: %v", err)
			ok = false
			continue
		}
		i, found := c.index(f.Name)
		isKey := found && slices.Contains(c.layout.PrimaryKey, f.Name)
		switch {
		case isKey:
			if t := arrowbatch.Type(c.layout.Fields[i]); !arrow.TypeEqual(t, f.Type) {
				c.errs.addf(path, "function output %q is a primary key column and must keep its type %s", f.Name, t)
				ok = false
				continue
			}
			rf = c.layout.Fields[i]
		case found:
			prev := c.layout.Fields[i]
			switch {
			case arrowbatch.LogicalOf(f) == rowmodel.LogicalUnknown && arrow.TypeEqual(arrowbatch.Type(prev), f.Type):
				// An untagged column stored like its input keeps the input's json, uuid or decimal type.
				rf = prev
				rf.Nullable = f.Nullable
			case rf.Native == "" && rf.Logical == prev.Logical:
				rf.Native = prev.Native
			}
		}
		fields = append(fields, arrow.Field{Name: f.Name, Type: f.Type, Nullable: f.Nullable})
		idx = append(idx, i)
		keys = append(keys, isKey)
		layout = append(layout, rf)
	}
	for _, key := range c.layout.PrimaryKey {
		if !seen[key] {
			c.errs.addf(path, "function output must include primary key column %q", key)
			ok = false
		}
	}
	if ok {
		c.layout.Fields = layout
	}
	return arrow.NewSchema(fields, nil), idx, keys, ok
}

// alchemyOp sends the input columns to a function and makes its output the frame.
type alchemyOp struct {
	call    Call
	invoker Invoker
	in      *arrow.Schema
	inIdx   []int
	out     *arrow.Schema
	outIdx  []int
	outKeys []bool
}

func (o alchemyOp) apply(ctx context.Context, f *frame) error {
	if f.rows == 0 {
		o.empty(ctx, f)
		return nil
	}
	if o.invoker == nil {
		return fmt.Errorf("%s: no alchemy invoker is configured", o.call.Function)
	}
	cols := make([]arrow.Array, len(o.inIdx))
	for i, idx := range o.inIdx {
		cols[i] = f.cols[idx]
	}
	in := array.NewRecordBatch(o.in, cols, int64(f.rows))
	defer in.Release()

	out, err := o.invoker.Invoke(ctx, o.call, in)
	if err != nil {
		return fmt.Errorf("%s: %w", o.call.Function, err)
	}
	defer out.Release()
	if err := o.checkOutput(out, f.rows); err != nil {
		return fmt.Errorf("%s: %w", o.call.Function, err)
	}
	for i, idx := range o.outIdx {
		if o.outKeys[i] && !array.Equal(f.cols[idx], out.Column(i)) {
			return fmt.Errorf("%s: changed primary key column %q", o.call.Function, o.out.Field(i).Name)
		}
	}
	next := make([]arrow.Array, len(o.outIdx))
	for i, idx := range o.outIdx {
		col := out.Column(i)
		if o.outKeys[i] {
			col = f.cols[idx]
		}
		col.Retain()
		next[i] = col
	}
	f.release()
	f.cols = next
	return nil
}

// empty gives a zero-row frame the function's output columns without calling it.
func (o alchemyOp) empty(ctx context.Context, f *frame) {
	next := make([]arrow.Array, len(o.outIdx))
	for i, idx := range o.outIdx {
		if o.outKeys[i] {
			next[i] = f.cols[idx]
			next[i].Retain()
			continue
		}
		next[i] = array.MakeArrayOfNull(compute.GetAllocator(ctx), o.out.Field(i).Type, 0)
	}
	f.release()
	f.cols = next
}

// checkOutput holds the result to the contract the step compiled against, whatever the invoker returned.
func (o alchemyOp) checkOutput(out arrow.RecordBatch, rows int) error {
	if out.NumRows() != int64(rows) {
		return fmt.Errorf("returned %d rows, want %d", out.NumRows(), rows)
	}
	got := out.Schema()
	if got.NumFields() != o.out.NumFields() {
		return fmt.Errorf("returned %d columns, want %d", got.NumFields(), o.out.NumFields())
	}
	for i, want := range o.out.Fields() {
		g := got.Field(i)
		if g.Name != want.Name || !arrow.TypeEqual(g.Type, want.Type) {
			return fmt.Errorf("returned column %d as %q %s, want %q %s", i, g.Name, g.Type, want.Name, want.Type)
		}
		if !want.Nullable && out.Column(i).NullN() > 0 {
			return fmt.Errorf("returned nulls in non-nullable column %q", want.Name)
		}
	}
	return nil
}
