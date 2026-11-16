package iceberg

import (
	"cmp"
	"context"
	"encoding/binary"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"
)

// pruneDeletes drops equality-delete keys outside the key column's bounds
// across the table's data files. No existing row can hold such a key, so new
// inserts with increasing keys write no deletes, and the deletes left apply to
// fewer reads and maintenance rewrites. A replayed epoch still deletes its
// keys, because the data files it committed first now cover them. Composite
// keys, other key types and files without bounds keep every delete.
func pruneDeletes(ctx context.Context, it *iceTable, keys []string, m *mutation) error {
	if len(keys) != 1 || len(m.deletes) == 0 {
		return nil
	}
	field, ok := it.schema.FindFieldByName(keys[0])
	if !ok {
		return nil
	}
	switch field.Type {
	case iceberg.PrimitiveTypes.Int32:
		return pruneByRange(ctx, it, field.ID, m, func(b []byte) (int64, bool) {
			if len(b) != 4 {
				return 0, false
			}
			return int64(int32(binary.LittleEndian.Uint32(b))), true
		}, func(a arrow.Array, i int) int64 { return int64(a.(*array.Int32).Value(i)) })
	case iceberg.PrimitiveTypes.Int64:
		return pruneByRange(ctx, it, field.ID, m, func(b []byte) (int64, bool) {
			if len(b) != 8 {
				return 0, false
			}
			return int64(binary.LittleEndian.Uint64(b)), true
		}, func(a arrow.Array, i int) int64 { return a.(*array.Int64).Value(i) })
	case iceberg.PrimitiveTypes.String:
		// Iceberg orders strings by UTF-8 bytes, as Go compares them, and a
		// truncated upper bound is rounded up.
		return pruneByRange(ctx, it, field.ID, m, func(b []byte) (string, bool) {
			return string(b), true
		}, func(a arrow.Array, i int) string { return a.(*array.String).Value(i) })
	}
	return nil
}

func pruneByRange[T cmp.Ordered](ctx context.Context, it *iceTable, fieldID int, m *mutation, decode func([]byte) (T, bool), value func(arrow.Array, int) T) error {
	r, ok, err := keyRangeOf(ctx, it.tbl, fieldID, decode)
	if err != nil || !ok {
		return err
	}
	kept := m.deletes[:0]
	for _, b := range m.deletes {
		col := b.Column(0)
		mask := make([]bool, b.NumRows())
		for i := range mask {
			mask[i] = r.contains(value(col, i))
		}
		sel, err := selectRows(ctx, b, mask, b.Schema())
		b.Release()
		if err != nil {
			for _, k := range kept {
				k.Release()
			}
			m.deletes = nil
			return err
		}
		if sel != nil {
			kept = append(kept, sel)
		}
	}
	m.deletes = kept
	return nil
}

// keyRange is the union of the data files' bounds for one column. An empty
// range means the table has no data files.
type keyRange[T cmp.Ordered] struct {
	lo, hi T
	empty  bool
}

func (r keyRange[T]) contains(v T) bool {
	return !r.empty && v >= r.lo && v <= r.hi
}

// keyRangeOf reports ok false when any data file lacks a usable bound.
func keyRangeOf[T cmp.Ordered](ctx context.Context, tbl *table.Table, fieldID int, decode func([]byte) (T, bool)) (keyRange[T], bool, error) {
	r := keyRange[T]{empty: true}
	if tbl.CurrentSnapshot() == nil {
		return r, true, nil
	}
	tasks, err := tbl.Scan().PlanFiles(ctx)
	if err != nil {
		return r, false, err
	}
	for _, task := range tasks {
		loRaw, hiRaw := task.File.LowerBoundValues()[fieldID], task.File.UpperBoundValues()[fieldID]
		if loRaw == nil || hiRaw == nil {
			return r, false, nil
		}
		lo, okLo := decode(loRaw)
		hi, okHi := decode(hiRaw)
		if !okLo || !okHi {
			return r, false, nil
		}
		if r.empty {
			r = keyRange[T]{lo: lo, hi: hi}
			continue
		}
		r.lo, r.hi = min(r.lo, lo), max(r.hi, hi)
	}
	return r, true, nil
}
