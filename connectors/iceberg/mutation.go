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

// mutation is a buffer folded by key: the filter selecting every touched key,
// and the surviving rows (each key's last non-delete version) in the table's
// Arrow schema, ready to overwrite.
type mutation struct {
	filter iceberg.BooleanExpression
	live   []arrow.RecordBatch // owned; release after use
}

func (m *mutation) release() {
	for _, b := range m.live {
		b.Release()
	}
	m.live = nil
}

// foldMutations scans the buffer once to find each key's last row, then a
// second time to keep exactly those rows that are not deletes.
func foldMutations(ctx context.Context, it *iceTable, rb *recordBuf, keys []string, target *arrow.Schema) (*mutation, error) {
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
	var expr iceberg.BooleanExpression
	bi := 0
	err := rb.stream(func(rows arrow.RecordBatch, _ []rowmodel.Operation) error {
		for i := range int(rows.NumRows()) {
			k := keyOf(rows, keyIdx, i)
			if _, seen := last[k]; !seen {
				lits := make([]any, len(keyIdx))
				for n, c := range keyIdx {
					lit, err := keyLiteral(rows.Column(c), i)
					if err != nil {
						return fmt.Errorf("primary key %q: %w", keys[n], err)
					}
					lits[n] = lit
				}
				term, err := keyExpr(it, keys, lits)
				if err != nil {
					return err
				}
				if expr == nil {
					expr = term
				} else {
					expr = iceberg.NewOr(expr, term)
				}
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

	// Pass 2: keep each key's last row unless it is a delete, then conform the
	// survivors (only they need the table's types).
	m := &mutation{filter: expr}
	bi = 0
	err = rb.stream(func(rows arrow.RecordBatch, ops []rowmodel.Operation) error {
		mask := array.NewBooleanBuilder(memory.DefaultAllocator)
		defer mask.Release()
		n := int(rows.NumRows())
		mask.Reserve(n)
		kept := 0
		for i := range n {
			p := last[keyOf(rows, keyIdx, i)]
			keep := p.batch == bi && p.row == i && (len(ops) == 0 || ops[i] != rowmodel.OpDelete)
			mask.Append(keep)
			if keep {
				kept++
			}
		}
		bi++
		if kept == 0 {
			return nil
		}
		filter := mask.NewArray()
		defer filter.Release()
		survivors, err := compute.FilterRecordBatch(ctx, rows, filter, compute.DefaultFilterOptions())
		if err != nil {
			return err
		}
		defer survivors.Release()
		live, err := conform(survivors, target)
		if err != nil {
			return err
		}
		m.live = append(m.live, live)
		return nil
	})
	if err != nil {
		m.release()
		return nil, err
	}
	return m, nil
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
