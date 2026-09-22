package transform

import (
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/galaxy-io/filament/rowmodel"
)

// ExpressionType is the logical type of one compiled sub-expression, at the
// same kind of path an Issue uses, so a client reads types and issues off one
// map. resources["r"].steps[2].compute["x"] is a whole expression,
// resources["r"].steps[2].compute["x"].concat[1] its second argument, and
// resources["r"].steps[2].where.and[0].gt[1] a value inside a condition. A
// literal that was widened to a column's type reports the widened type.
type ExpressionType struct {
	Path    string
	Logical rowmodel.LogicalType
}

// Analysis is what Analyze learns besides the plan: the type of every
// sub-expression that compiled, and the schema as the steps leave it. Layout
// is filled in whether or not the definition compiled: the entries of a step
// that compiled still apply, and the ones that failed are skipped, so a
// builder can offer the right columns to the steps after a broken one.
type Analysis struct {
	Types  []ExpressionType
	Layout rowmodel.Schema
}

// Compile resolves the definition's steps for in.Resource against in and
// returns the plan that applies them. Every column reference and function
// call is checked here, so Apply never meets a name or type it has not seen.
// A resource the definition does not mention compiles to an identity plan.
func Compile(def *Definition, in rowmodel.Schema) (*Plan, error) {
	plan, _, err := Analyze(def, in)
	return plan, err
}

// Analyze compiles like Compile and also reports the type of every
// sub-expression that compiled, in compile order, and the schema the steps
// produce, whether or not the whole definition did. A builder shows those
// while the author is mid-edit.
func Analyze(def *Definition, in rowmodel.Schema) (*Plan, Analysis, error) {
	c := compiler{layout: in.Clone(), typeIndex: map[string]int{}}
	if res, ok := def.Resources[in.Resource]; ok {
		for i, st := range res.Steps {
			c.step(fmt.Sprintf("resources[%q].steps[%d]", in.Resource, i), st)
		}
	}
	analysis := Analysis{Types: c.types, Layout: c.layout}
	if err := c.errs.asError(); err != nil {
		return nil, analysis, err
	}
	return newPlan(in, c.layout, c.ops), analysis, nil
}

// compiler walks the steps while carrying the schema as each step leaves it.
// Ops record column indices against that working layout, and Apply mutates
// its frame in the same order, so the indices line up at run time.
type compiler struct {
	layout    rowmodel.Schema
	ops       []op
	errs      Errors
	types     []ExpressionType
	typeIndex map[string]int
}

// recordType notes the type at path, replacing an earlier note for the same
// path when a literal is widened after it was first compiled.
func (c *compiler) recordType(path string, t rowmodel.LogicalType) {
	if i, ok := c.typeIndex[path]; ok {
		c.types[i].Logical = t
		return
	}
	c.typeIndex[path] = len(c.types)
	c.types = append(c.types, ExpressionType{Path: path, Logical: t})
}

// index finds a column in the working layout.
func (c *compiler) index(name string) (int, bool) {
	for i, f := range c.layout.Fields {
		if f.Name == name {
			return i, true
		}
	}
	return -1, false
}

// step dispatches on the one field the grammar guarantees is set.
func (c *compiler) step(path string, st Step) {
	switch {
	case st.Rename != nil:
		c.rename(path+".rename", st.Rename)
	case st.Drop != nil:
		c.drop(path+".drop", st.Drop)
	case st.Compute != nil:
		c.compute(path, st.Compute, st.Where)
	}
}

// rename changes names in the layout only; the frame at run time is indexed
// by position and never sees a name. Primary key entries follow the rename.
func (c *compiler) rename(path string, m map[string]string) {
	for _, from := range slices.Sorted(maps.Keys(m)) {
		to := m[from]
		i, ok := c.index(from)
		if !ok {
			c.errs.addf(fmt.Sprintf("%s[%q]", path, from), "unknown column")
			continue
		}
		if _, taken := c.index(to); taken {
			c.errs.addf(fmt.Sprintf("%s[%q]", path, from), "column %q already exists", to)
			continue
		}
		c.layout.Fields[i].Name = to
		for k, key := range c.layout.PrimaryKey {
			if key == from {
				c.layout.PrimaryKey[k] = to
			}
		}
	}
}

// drop removes columns from the layout and records one op per column. A
// primary key column cannot be dropped.
func (c *compiler) drop(path string, names []string) {
	for j, name := range names {
		i, ok := c.index(name)
		if !ok {
			c.errs.addf(fmt.Sprintf("%s[%d]", path, j), "unknown column %q", name)
			continue
		}
		if slices.Contains(c.layout.PrimaryKey, name) {
			c.errs.addf(fmt.Sprintf("%s[%d]", path, j), "cannot drop primary key column %q", name)
			continue
		}
		c.layout.Fields = slices.Delete(c.layout.Fields, i, i+1)
		c.ops = append(c.ops, dropOp{idx: i})
	}
}

// compute compiles every entry against the layout as it stood before the step and
// applies the resulting changes together, so entries in one step cannot refer
// to each other. Entries are visited in name order, which fixes where new
// columns land. Under where, an entry on an existing column must keep its
// type, because the unmatched rows keep their old values; a new column is
// null on those rows.
func (c *compiler) compute(path string, entries map[string]Expr, where *Expr) {
	o := computeOp{}
	if where != nil {
		n, t, ok := c.expr(path+".where", *where)
		if ok && t.logical != rowmodel.LogicalBool {
			c.errs.addf(path+".where", "expects %s, got %s", rowmodel.LogicalBool, t.logical)
			ok = false
		}
		if !ok {
			return
		}
		o.where = n
	}

	type pending struct {
		idx   int
		field rowmodel.Field
	}
	var changes []pending
	for _, name := range slices.Sorted(maps.Keys(entries)) {
		epath := fmt.Sprintf("%s.compute[%q]", path, name)
		n, t, ok := c.expr(epath, entries[name])
		if !ok {
			continue
		}
		idx, exists := c.index(name)
		if exists && where != nil && c.layout.Fields[idx].Logical != t.logical {
			c.errs.addf(epath, "where cannot change type: column is %s, expression is %s", c.layout.Fields[idx].Logical, t.logical)
			continue
		}
		if !exists {
			idx = -1
		}
		o.entries = append(o.entries, computeEntry{idx: idx, expr: n})
		changes = append(changes, pending{idx: idx, field: rowmodel.Field{Name: name, Nullable: true, Logical: t.logical}})
	}
	if len(o.entries) == 0 {
		return
	}
	for _, ch := range changes {
		if ch.idx < 0 {
			c.layout.Fields = append(c.layout.Fields, ch.field)
			continue
		}
		c.layout.Fields[ch.idx] = ch.field
	}
	c.ops = append(c.ops, o)
}

// expr compiles one expression into an executable node and reports its type.
// ok is false when an issue was recorded, in which case the node is nil.
// Every argument is compiled even after one fails, so the types of the parts
// that do compile are still reported.
func (c *compiler) expr(path string, e Expr) (node, argType, bool) {
	switch {
	case e.Lit != nil:
		t := literalType(e.Lit)
		c.recordType(path, t)
		return newLiteralNode(e.Lit, t), argType{logical: t, literal: true, value: e.Lit}, true
	case e.Col != "":
		i, ok := c.index(e.Col)
		if !ok {
			c.errs.addf(path, "unknown column %q", e.Col)
			return nil, argType{}, false
		}
		t := c.layout.Fields[i].Logical
		c.recordType(path, t)
		return colNode{idx: i}, argType{logical: t}, true
	}

	fn, ok := catalog[e.Fn]
	if !ok {
		c.errs.addf(path, "unknown function %q", e.Fn)
		return nil, argType{}, false
	}
	argPath := func(i int) string { return fmt.Sprintf("%s.%s[%d]", path, e.Fn, i) }
	args := make([]node, len(e.Args))
	types := make([]argType, len(e.Args))
	failed := false
	for i, a := range e.Args {
		n, t, ok := c.expr(argPath(i), a)
		if !ok {
			failed = true
			continue
		}
		args[i], types[i] = n, t
	}
	if failed {
		return nil, argType{}, false
	}
	if fn.spec.SameType {
		coerceLiterals(fn.spec, args, types)
		for i, t := range types {
			c.recordType(argPath(i), t.logical)
		}
	}
	ret, err := fn.check(types)
	if err != nil {
		at := path
		var ae *argError
		if errors.As(err, &ae) {
			at = argPath(ae.index)
		}
		c.errs.addf(at, "%v", err)
		return nil, argType{}, false
	}
	c.recordType(path, ret)
	return callNode{fn: fn, args: args}, argType{logical: ret}, true
}

// coerceLiterals widens each literal argument to the type of the first column
// argument when the value fits, so eq(int32_col, 1) reads as the author
// meant it. A literal that cannot widen is left alone for check to reject.
func coerceLiterals(spec FunctionSpec, args []node, types []argType) {
	target := rowmodel.LogicalUnknown
	for i, t := range types {
		if !t.literal && !spec.argSpec(i).Literal {
			target = t.logical
			break
		}
	}
	if target == rowmodel.LogicalUnknown {
		return
	}
	for i, t := range types {
		if !t.literal || spec.argSpec(i).Literal || t.logical == target || !coercible(t.value, target) {
			continue
		}
		args[i] = newLiteralNode(t.value, target)
		types[i].logical = target
	}
}

// coercible reports whether literal v can stand in for a value of type to
// without losing anything.
func coercible(v any, to rowmodel.LogicalType) bool {
	switch x := v.(type) {
	case int64:
		switch to {
		case rowmodel.LogicalInt16:
			return x >= -1<<15 && x < 1<<15
		case rowmodel.LogicalInt32:
			return x >= -1<<31 && x < 1<<31
		case rowmodel.LogicalInt64, rowmodel.LogicalFloat32, rowmodel.LogicalFloat64:
			return true
		}
	case float64:
		return to == rowmodel.LogicalFloat32 || to == rowmodel.LogicalFloat64
	}
	return false
}

// literalType returns the logical type of a literal Expr admits.
func literalType(v any) rowmodel.LogicalType {
	switch v.(type) {
	case string:
		return rowmodel.LogicalString
	case int64:
		return rowmodel.LogicalInt64
	case float64:
		return rowmodel.LogicalFloat64
	case bool:
		return rowmodel.LogicalBool
	}
	panic(fmt.Sprintf("transform: literal %T not admitted by Expr", v))
}
