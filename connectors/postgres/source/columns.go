package postgres

// A table's columns: the catalog lookup, the RecordSchema built from it, and the
// rowDecoder that appends a scanned row's raw binary values straight into the
// pipeline's RowWriter, so neither the source database nor this process ever
// renders a row as text. The per-type behaviour lives in types.go; a type
// without a binary form is projected as (t.col)::text at plan time, never per row.

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// engine names this source in the schemas it reports.
const engine = "postgres"

// column is one live column of a table, from the catalog, in attribute order.
type column struct {
	name     string
	native   string // format_type spelling, e.g. "numeric(12,2)"
	oid      uint32
	typmod   int32
	nullable bool
}

// columns returns schema.table's live columns in attribute order: the set and
// order SELECT * yields.
func (s *Source) columns(ctx context.Context, schema, table string) ([]column, error) {
	const q = `
SELECT a.attname, format_type(a.atttypid, a.atttypmod), a.atttypid, a.atttypmod, a.attnotnull
FROM   pg_attribute a
JOIN   pg_class cl ON cl.oid = a.attrelid
JOIN   pg_namespace ns ON ns.oid = cl.relnamespace
WHERE  ns.nspname = $1 AND cl.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped
ORDER  BY a.attnum`
	rows, err := s.pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []column
	for rows.Next() {
		var c column
		var notNull bool
		if err := rows.Scan(&c.name, &c.native, &c.oid, &c.typmod, &notNull); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		c.nullable = !notNull
		cols = append(cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(cols) == 0 {
		return nil, fmt.Errorf("resource %q has no columns (missing table?)", table)
	}
	return cols, nil
}

// field maps a catalog column to its schema field and type behaviour. Logical
// follows the type OID, so it always agrees with how the column is decoded;
// binary reports whether the type has a binary form or is read as text.
func (c column) field() (f rowmodel.Field, t pgType, binary bool) {
	f = rowmodel.Field{Name: c.name, Nullable: c.nullable, Native: c.native}
	if c.oid == pgtype.NumericOID {
		f.Precision, f.Scale = numericPrecScale(c.typmod)
	}
	t, binary = typeFor(c.oid, f)
	f.Logical = t.logical
	return f, t, binary
}

// schemaOf assembles table's RecordSchema from its catalog columns and key names.
func schemaOf(table string, cols []column, pks []string) rowmodel.Schema {
	fields := make([]rowmodel.Field, len(cols))
	for i, c := range cols {
		fields[i], _, _ = c.field()
	}
	return rowmodel.Schema{Resource: table, Fields: fields, PrimaryKey: pks, Engine: engine}
}

// rowDecoder appends one scanned row's raw binary column values into a
// RowWriter. Built once per table, shared read-only by its shards.
type rowDecoder struct {
	schema     rowmodel.Schema
	selectList string   // column projection (no ctid); text-read types wrapped (…)::text
	types      []pgType // per column
	pkIdx      []int    // pk column positions in key order
}

// decoderFor builds table's row decoder from the catalog. pks names the key
// columns whose text forms become keyset cursors.
func (s *Source) decoderFor(ctx context.Context, table string, pks []string) (*rowDecoder, error) {
	cols, err := s.columns(ctx, s.schema, table)
	if err != nil {
		return nil, fmt.Errorf("lookup columns %q: %w", table, err)
	}
	return newRowDecoder(table, cols, pks)
}

func newRowDecoder(table string, cols []column, pks []string) (*rowDecoder, error) {
	d := &rowDecoder{
		schema: rowmodel.Schema{Resource: table, PrimaryKey: pks, Engine: engine},
		types:  make([]pgType, len(cols)),
	}
	parts := make([]string, len(cols))
	for i, c := range cols {
		f, t, binary := c.field()
		ident := "t." + pgx.Identifier{c.name}.Sanitize()
		if !binary {
			ident = "(" + ident + ")::text"
		}
		parts[i], d.types[i] = ident, t
		d.schema.Fields = append(d.schema.Fields, f)
	}
	d.selectList = strings.Join(parts, ", ")
	for _, pk := range pks {
		i := d.index(pk)
		if i < 0 {
			return nil, fmt.Errorf("decode %q: pk column %q not in catalog", table, pk)
		}
		d.pkIdx = append(d.pkIdx, i)
	}
	return d, nil
}

// index returns the column position of name, or -1.
func (d *rowDecoder) index(name string) int {
	return slices.IndexFunc(d.schema.Fields, func(f rowmodel.Field) bool { return f.Name == name })
}

// appendRow appends the row's columns in schema order; the caller ends the row.
func (d *rowDecoder) appendRow(w arrowbatch.RowWriter, raw [][]byte) error {
	if len(raw) < len(d.types) {
		return fmt.Errorf("row has %d values, want %d", len(raw), len(d.types))
	}
	for i, t := range d.types {
		if raw[i] == nil {
			w.Null()
			continue
		}
		if err := t.fromBinary(w, raw[i]); err != nil {
			return fmt.Errorf("column %q: %w", d.schema.Fields[i].Name, err)
		}
	}
	return nil
}

// texts renders the columns at idx in the text form PostgreSQL parses back for
// a cursor bound: keyset keys and incremental watermarks.
func (d *rowDecoder) texts(raw [][]byte, idx []int) ([]string, error) {
	out := make([]string, len(idx))
	for n, i := range idx {
		if raw[i] == nil {
			continue // cannot occur for a real key
		}
		b, err := d.types[i].toText(nil, raw[i])
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", d.schema.Fields[i].Name, err)
		}
		out[n] = string(b)
	}
	return out, nil
}
