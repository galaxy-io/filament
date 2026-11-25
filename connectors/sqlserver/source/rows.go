package sqlserver

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"

	mssql "github.com/microsoft/go-mssqldb"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// querier runs a query on a pool, a dedicated connection or a transaction.
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// rowReader is the mssql.RawRowHandler every read goes through. It turns each
// result row's TDS values into one row of the writer, with no per-value
// allocation, and stamps each row with the metadata the pipeline checkpoints.
//
// A result set holds the schema's fields in order, after skip leading columns
// that are not part of the row: a change-table read puts its LSN, sequence and
// operation columns there and derives the row's metadata from them through
// meta. Snapshot and incremental reads have none and carry the key instead.
type rowReader struct {
	schema rowmodel.Schema
	open   func() (arrowbatch.RowWriter, error)
	limit  int // stop after this many rows; 0 = all

	skip int
	// meta derives a row's metadata from its skip columns. It takes and
	// returns the Meta by value so the per-row Meta stays on the stack.
	meta func(row *mssql.RawRow, m rowmodel.Meta) (rowmodel.Meta, error)

	// keyNames are the fields whose values make up each row's Meta.Key.
	keyNames []string

	w     arrowbatch.RowWriter
	decs  []decodeFunc
	n     int
	utf8  []byte
	key   keyTracker
	ready bool
}

// errLimit ends a read that reached its row limit. It is not a failure.
var errLimit = errors.New("sqlserver: row limit reached")

func (r *rowReader) Columns(cols []mssql.RawColumn) error {
	if r.ready {
		return fmt.Errorf("query for %q returned more than one result set", r.schema.Resource)
	}
	fields := r.schema.Fields
	if len(cols)-r.skip != len(fields) {
		return fmt.Errorf("query for %q returned %d columns, the schema has %d", r.schema.Resource, len(cols)-r.skip, len(fields))
	}
	r.decs = make([]decodeFunc, len(fields))
	for i := range fields {
		col := &cols[r.skip+i]
		dec, err := decoderFor(col, fields[i])
		if err != nil {
			return err
		}
		r.decs[i] = dec
	}
	if err := r.key.init(cols, r.skip, fields, r.keyNames); err != nil {
		return err
	}
	if r.w == nil {
		w, err := r.open()
		if err != nil {
			return err
		}
		r.w = w
	}
	r.ready = true
	return nil
}

func (r *rowReader) Row(row *mssql.RawRow) error {
	if r.limit > 0 && r.n >= r.limit {
		return errLimit
	}
	for i, dec := range r.decs {
		v, ok := row.Value(r.skip + i)
		if !ok {
			r.w.Null()
			continue
		}
		if err := dec(r, r.w, v); err != nil {
			return fmt.Errorf("column %q: %w", r.schema.Fields[i].Name, err)
		}
	}
	var m rowmodel.Meta
	if r.key.cols != nil {
		key, err := r.key.update(row)
		if err != nil {
			return err
		}
		m.Key = key
	}
	if r.meta != nil {
		var err error
		if m, err = r.meta(row, m); err != nil {
			return err
		}
	}
	if err := r.w.EndRow(m); err != nil {
		return err
	}
	r.n++
	return nil
}

// keyTracker renders each row's key as checkpoint text. Every row's Meta.Key
// shares one slice, which EndRow borrows: the builder copies the key of each
// chunk's last row. A key column whose bytes repeat from the previous row keeps
// its string, so a composite key re-renders only the columns that changed.
type keyTracker struct {
	cols []int // result column of each key value
	fmts []keyFormatter
	prev [][]byte
	vals []string
}

func (k *keyTracker) init(cols []mssql.RawColumn, skip int, fields []rowmodel.Field, names []string) error {
	if len(names) == 0 {
		return nil
	}
	k.cols = make([]int, len(names))
	k.fmts = make([]keyFormatter, len(names))
	k.prev = make([][]byte, len(names))
	k.vals = make([]string, len(names))
	for n, name := range names {
		idx := -1
		for i := range fields {
			if fields[i].Name == name {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("key column %q is not in the result", name)
		}
		f, err := keyFormatterFor(&cols[skip+idx])
		if err != nil {
			return err
		}
		k.cols[n], k.fmts[n] = skip+idx, f
	}
	return nil
}

func (k *keyTracker) update(row *mssql.RawRow) ([]string, error) {
	for n, col := range k.cols {
		v, ok := row.Value(col)
		if !ok {
			return nil, fmt.Errorf("key column %d is NULL", n)
		}
		if k.prev[n] != nil && bytes.Equal(k.prev[n], v) {
			continue
		}
		s, err := k.fmts[n](v)
		if err != nil {
			return nil, err
		}
		k.vals[n] = s
		k.prev[n] = append(k.prev[n][:0], v...)
	}
	return k.vals, nil
}

// readRows runs query with args and streams its rows through r.
func readRows(ctx context.Context, q querier, query string, args []any, r *rowReader) error {
	rows, err := q.QueryContext(ctx, query, append(args, mssql.RawRows{Handler: r})...)
	if err != nil {
		return err
	}
	for rows.Next() { //nolint:revive // rows are delivered to the handler
	}
	err = rows.Err()
	closeErr := rows.Close()
	if errors.Is(err, errLimit) {
		return nil
	}
	return errors.Join(err, closeErr)
}

// keyCapture is a handler that keeps the checkpoint text of every row's
// columns. Planning queries use it to read key values, such as a high
// watermark or shard boundaries, in the same form the read path writes them.
type keyCapture struct {
	fmts []keyFormatter
	rows [][]string
}

func (c *keyCapture) Columns(cols []mssql.RawColumn) error {
	c.fmts = make([]keyFormatter, len(cols))
	for i := range cols {
		f, err := keyFormatterFor(&cols[i])
		if err != nil {
			return err
		}
		c.fmts[i] = f
	}
	return nil
}

func (c *keyCapture) Row(row *mssql.RawRow) error {
	vals := make([]string, len(c.fmts))
	for i, f := range c.fmts {
		v, ok := row.Value(i)
		if !ok {
			return fmt.Errorf("key value %d is NULL", i)
		}
		s, err := f(v)
		if err != nil {
			return err
		}
		vals[i] = s
	}
	c.rows = append(c.rows, vals)
	return nil
}

// queryKeys returns every row of a query as checkpoint text.
func queryKeys(ctx context.Context, q querier, query string, args ...any) ([][]string, error) {
	c := &keyCapture{}
	rows, err := q.QueryContext(ctx, query, append(args, mssql.RawRows{Handler: c})...)
	if err != nil {
		return nil, err
	}
	for rows.Next() { //nolint:revive // rows are delivered to the handler
	}
	err = rows.Err()
	return c.rows, errors.Join(err, rows.Close())
}
