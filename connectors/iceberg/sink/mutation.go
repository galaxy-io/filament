package iceberg

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/compute"
	"github.com/apache/arrow-go/v18/arrow/memory"
	iceberg "github.com/apache/iceberg-go"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament/rowmodel"
)

// mutation is a buffer folded by key: the surviving rows (each key's last
// non-delete version) in the table's Arrow schema, and every touched key either
// as a copy-on-write filter or as merge-on-read equality-delete rows.
type mutation struct {
	touched int
	filter  iceberg.BooleanExpression // copy-on-write only
	live    []arrow.RecordBatch       // owned; release after use
	deletes []arrow.RecordBatch       // merge-on-read only; owned
}

func (m *mutation) release() {
	for _, b := range m.live {
		b.Release()
	}
	for _, b := range m.deletes {
		b.Release()
	}
	m.live, m.deletes = nil, nil
}

// foldMutations scans the buffer once to find each key's last row, then a
// second time to keep exactly those rows that are not deletes. A non-nil
// deleteTarget folds for merge-on-read, collecting one key row per touched key
// in that schema instead of building a filter.
func foldMutations(ctx context.Context, it *iceTable, rb *recordBuf, keys []string, target, deleteTarget *arrow.Schema) (*mutation, error) {
	keyIdx := make([]int, len(keys))
	for n, k := range keys {
		idx := rb.schema.FieldIndices(k)
		if len(idx) == 0 {
			return nil, fmt.Errorf("primary key %q is not in the batch schema", k)
		}
		keyIdx[n] = idx[0]
	}

	// Pass 1: last (batch, row) per key, and one literal tuple per distinct key.
	type pos struct{ batch, row int }
	last := map[string]pos{}
	var tuples [][]any
	bi := 0
	err := rb.stream(func(rows arrow.RecordBatch, _ []rowmodel.Operation) error {
		for i := range int(rows.NumRows()) {
			k := keyOf(rows, keyIdx, i)
			if _, seen := last[k]; !seen && deleteTarget == nil {
				lits := make([]any, len(keyIdx))
				for n, c := range keyIdx {
					lit, err := keyLiteral(rows.Column(c), i)
					if err != nil {
						return fmt.Errorf("primary key %q: %w", keys[n], err)
					}
					lits[n] = lit
				}
				tuples = append(tuples, lits)
			}
			last[k] = pos{bi, i}
		}
		bi++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(last) == 0 {
		return &mutation{}, nil
	}
	var expr iceberg.BooleanExpression
	if deleteTarget == nil {
		if expr, err = keysExpr(it, keys, tuples); err != nil {
			return nil, err
		}
	}

	// Pass 2: keep each key's last row unless it is a delete, then conform the
	// survivors (only they need the table's types).
	m := &mutation{touched: len(last), filter: expr}
	bi = 0
	err = rb.stream(func(rows arrow.RecordBatch, ops []rowmodel.Operation) error {
		n := int(rows.NumRows())
		lastRow := make([]bool, n)
		keep := make([]bool, n)
		for i := range n {
			p := last[keyOf(rows, keyIdx, i)]
			lastRow[i] = p.batch == bi && p.row == i
			keep[i] = lastRow[i] && (len(ops) == 0 || ops[i] != rowmodel.OpDelete)
		}
		bi++
		live, err := selectRows(ctx, rows, keep, target)
		if err != nil {
			return err
		}
		if live != nil {
			m.live = append(m.live, live)
		}
		if deleteTarget == nil {
			return nil
		}
		// Inserts get a delete too, so a replayed epoch replaces its rows
		// instead of duplicating them.
		del, err := selectRows(ctx, rows, lastRow, deleteTarget)
		if err != nil {
			return err
		}
		if del != nil {
			m.deletes = append(m.deletes, del)
		}
		return nil
	})
	if err != nil {
		m.release()
		return nil, err
	}
	return m, nil
}

// selectRows returns the masked rows conformed to target, or nil when the mask
// selects none.
func selectRows(ctx context.Context, rows arrow.RecordBatch, mask []bool, target *arrow.Schema) (arrow.RecordBatch, error) {
	b := array.NewBooleanBuilder(memory.DefaultAllocator)
	defer b.Release()
	kept := 0
	for _, keep := range mask {
		b.Append(keep)
		if keep {
			kept++
		}
	}
	if kept == 0 {
		return nil, nil
	}
	filter := b.NewArray()
	defer filter.Release()
	selected, err := compute.FilterRecordBatch(ctx, rows, filter, compute.DefaultFilterOptions())
	if err != nil {
		return nil, err
	}
	defer selected.Release()
	return conform(selected, target)
}

// keyOf renders row i's primary key as one string, for last-write-wins folding.
// Columns are joined with 0x1f, which cannot appear in a key value's text form.
func keyOf(rows arrow.RecordBatch, keyIdx []int, i int) string {
	if len(keyIdx) == 1 {
		return rows.Column(keyIdx[0]).ValueStr(i)
	}
	var b strings.Builder
	for n, k := range keyIdx {
		if n > 0 {
			b.WriteByte(0x1f)
		}
		b.WriteString(rows.Column(k).ValueStr(i))
	}
	return b.String()
}

// keysExpr builds the filter selecting exactly the given distinct key tuples.
// A single-column key is one set predicate. A composite key picks the column
// with the most distinct values as the set column and groups the tuples by
// their other columns, giving one term per group:
//
//	other = values AND set_column IN (...)
//
// Each term matches exactly its group's tuples, so the OR of terms matches the
// changed keys and nothing else. Keys such as (tenant_id, id) or
// (order_id, line_number) collapse to a few terms; only keys whose every column
// is high-cardinality approach one term per tuple. Terms are joined in a
// balanced tree so nesting depth stays logarithmic.
func keysExpr(it *iceTable, keys []string, tuples [][]any) (iceberg.BooleanExpression, error) {
	if len(keys) == 1 {
		values := make([]any, len(tuples))
		for i, tuple := range tuples {
			values[i] = tuple[0]
		}
		return keyInExpr(it, keys[0], values)
	}

	setCol, most := 0, -1
	for c := range keys {
		distinct := make(map[string]struct{}, len(tuples))
		for _, tuple := range tuples {
			distinct[literalKey(tuple[c])] = struct{}{}
		}
		if len(distinct) > most {
			setCol, most = c, len(distinct)
		}
	}
	otherKeys := make([]string, 0, len(keys)-1)
	for c, key := range keys {
		if c != setCol {
			otherKeys = append(otherKeys, key)
		}
	}

	type group struct {
		others []any
		values []any
	}
	var groups []*group
	byOthers := map[string]*group{}
	for _, tuple := range tuples {
		others := make([]any, 0, len(keys)-1)
		var id strings.Builder
		for c, value := range tuple {
			if c == setCol {
				continue
			}
			others = append(others, value)
			id.WriteString(literalKey(value))
			id.WriteByte(0x1f)
		}
		g := byOthers[id.String()]
		if g == nil {
			g = &group{others: others}
			byOthers[id.String()] = g
			groups = append(groups, g)
		}
		g.values = append(g.values, tuple[setCol])
	}

	terms := make([]iceberg.BooleanExpression, len(groups))
	for i, g := range groups {
		equal, err := keyExpr(it, otherKeys, g.others)
		if err != nil {
			return nil, err
		}
		in, err := keyInExpr(it, keys[setCol], g.values)
		if err != nil {
			return nil, err
		}
		terms[i] = iceberg.NewAnd(equal, in)
	}
	return balancedOr(terms), nil
}

// balancedOr joins terms in a balanced tree rather than a left-deep chain.
func balancedOr(terms []iceberg.BooleanExpression) iceberg.BooleanExpression {
	if len(terms) == 1 {
		return terms[0]
	}
	mid := len(terms) / 2
	return iceberg.NewOr(balancedOr(terms[:mid]), balancedOr(terms[mid:]))
}

// literalKey renders a key literal for grouping. Values come from keyLiteral,
// so the type prefix keeps 1 and "1" apart and []byte compares by content.
func literalKey(value any) string {
	if raw, ok := value.([]byte); ok {
		return "[]byte:" + string(raw)
	}
	return fmt.Sprintf("%T:%v", value, value)
}

// keyInExpr builds one set-membership predicate over a single key column's
// distinct values. Engines test it with a hash lookup per row, where an OR of
// equalities costs one comparison per key per row.
func keyInExpr(it *iceTable, key string, values []any) (iceberg.BooleanExpression, error) {
	field, ok := it.schema.FindFieldByName(key)
	if !ok {
		return nil, fmt.Errorf("primary key %q is not in iceberg schema", key)
	}
	isUUID := field.Type.Equals(iceberg.PrimitiveTypes.UUID)
	lits := make([]iceberg.Literal, len(values))
	for i, value := range values {
		if isUUID {
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("primary key %q: expected uuid text, got %T", key, value)
			}
			u, err := uuid.Parse(text)
			if err != nil {
				return nil, fmt.Errorf("primary key %q: %w", key, err)
			}
			value = u
		}
		lit, err := newKeyLiteral(key, value)
		if err != nil {
			return nil, err
		}
		lits[i] = lit
	}
	return iceberg.SetPredicate(iceberg.OpIn, iceberg.Reference(key), lits), nil
}

// keyExpr builds the equality over one row's key columns; a uuid key column in
// the table binds its literal as a uuid.
func keyExpr(it *iceTable, keys []string, lits []any) (iceberg.BooleanExpression, error) {
	var expr iceberg.BooleanExpression
	for n, key := range keys {
		field, ok := it.schema.FindFieldByName(key)
		if !ok {
			return nil, fmt.Errorf("primary key %q is not in iceberg schema", key)
		}
		lit := lits[n]
		if field.Type.Equals(iceberg.PrimitiveTypes.UUID) {
			s, ok := lit.(string)
			if !ok {
				return nil, fmt.Errorf("primary key %q: expected uuid text, got %T", key, lit)
			}
			u, err := uuid.Parse(s)
			if err != nil {
				return nil, fmt.Errorf("primary key %q: %w", key, err)
			}
			lit = u
		}
		term, err := equalExpr(key, lit)
		if err != nil {
			return nil, err
		}
		if expr == nil {
			expr = term
		} else {
			expr = iceberg.NewAnd(expr, term)
		}
	}
	if expr == nil {
		return nil, fmt.Errorf("empty primary key for iceberg table")
	}
	return expr, nil
}
