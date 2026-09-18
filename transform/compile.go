package transform

import (
	"fmt"
	"maps"
	"slices"

	"github.com/galaxy-io/filament/rowmodel"
)

// Compile resolves the definition's steps for in.Resource against in and
// returns the plan that applies them. Every column reference and function
// call is checked here, so Apply never meets a name or type it has not seen.
// A resource the definition does not mention compiles to an identity plan.
func Compile(def *Definition, in rowmodel.Schema) (*Plan, error) {
	c := compiler{layout: in.Clone()}
	if res, ok := def.Resources[in.Resource]; ok {
		for i, st := range res.Steps {
			c.step(fmt.Sprintf("resources[%q].steps[%d]", in.Resource, i), st)
		}
	}
	if err := c.errs.asError(); err != nil {
		return nil, err
	}
	return newPlan(in, c.layout, c.ops), nil
}

// compiler walks the steps while carrying the schema as each step leaves it.
// Ops record column indices against that working layout, and Apply mutates
// its frame in the same order, so the indices line up at run time.
type compiler struct {
	layout rowmodel.Schema
	ops    []op
	errs   Errors
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
// columns land. Under where, an entry must target an existing column and
// keep its type, because the unmatched rows keep their old values.
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
		if !exists && where != nil {
			c.errs.addf(epath, "where requires an existing column; %q is new", name)
			continue
		}
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
func (c *compiler) expr(path string, e Expr) (node, argType, bool) {
	switch {
	case e.Lit != nil:
		return newLiteralNode(e.Lit), argType{logical: literalType(e.Lit), literal: true, value: e.Lit}, true
	case e.Col != "":
		i, ok := c.index(e.Col)
		if !ok {
			c.errs.addf(path, "unknown column %q", e.Col)
			return nil, argType{}, false
		}
		return colNode{idx: i}, argType{logical: c.layout.Fields[i].Logical}, true
	}

	fn, ok := catalog[e.Fn]
	if !ok {
		c.errs.addf(path, "unknown function %q", e.Fn)
		return nil, argType{}, false
	}
	args := make([]node, len(e.Args))
	types := make([]argType, len(e.Args))
	for i, a := range e.Args {
		n, t, ok := c.expr(fmt.Sprintf("%s.%s[%d]", path, e.Fn, i), a)
		if !ok {
			return nil, argType{}, false
		}
		args[i], types[i] = n, t
	}
	ret, err := fn.check(types)
	if err != nil {
		c.errs.addf(path, "%v", err)
		return nil, argType{}, false
	}
	return callNode{fn: fn, args: args}, argType{logical: ret}, true
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
