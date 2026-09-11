package postgres

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament/rowmodel"
)

// pkColumn is one primary-key column: its raw name (bound into the id projection) and
// Postgres type (used to cast a text resume cursor back to the column type on the
// keyset path; ignored on the ctid path).
type pkColumn struct{ name, typ string }

// lookupPrimaryKey returns the primary-key columns of schema.table in key order.
// An empty result means no primary key.
func (s *Source) lookupPrimaryKey(ctx context.Context, schema, table string) ([]pkColumn, error) {
	const q = `
SELECT a.attname, format_type(a.atttypid, a.atttypmod)
FROM   pg_index i
JOIN   pg_class cl ON cl.oid = i.indrelid
JOIN   pg_namespace ns ON ns.oid = cl.relnamespace
JOIN   pg_attribute a ON a.attrelid = cl.oid AND a.attnum = ANY(i.indkey)
WHERE  ns.nspname = $1 AND cl.relname = $2 AND i.indisprimary
ORDER  BY array_position(i.indkey, a.attnum)`
	rows, err := s.pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pks []pkColumn
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		pks = append(pks, pkColumn{name: name, typ: typ})
	}
	return pks, rows.Err()
}

// Schema returns the column schema of one table: every live column in attribute
// order with its Postgres type (format_type, kept verbatim in Native for a same-engine
// round-trip and classified into a LogicalType), plus the primary key. It implements
// filament.SchemaProvider so a Schematized sink can build matching typed tables.
func (s *Source) Schema(ctx context.Context, resource string) (rowmodel.Schema, error) {
	cols, err := s.columns(ctx, s.schema, resource)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	pks, err := s.lookupPrimaryKey(ctx, s.schema, resource)
	if err != nil {
		return rowmodel.Schema{}, fmt.Errorf("lookup pk: %w", err)
	}
	return schemaOf(resource, cols, pkNames(pks)), nil
}
