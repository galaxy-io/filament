package mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// EnsureSchema creates resource's typed table from schema (MySQL column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and prepares
// the resource's load and delete statements. A pre-existing table gains any new
// columns; an incompatible existing column type surfaces later as a load error
// (full type-change handling is deferred to schema evolution).
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if t.db == nil {
		return fmt.Errorf("mysql sink: ensure schema before open")
	}
	qualified := quoteIdent(t.database) + "." + quoteIdent(resource)
	same := schema.Engine == engine

	cols := make([]string, len(schema.Fields))   // `name` type [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields)) // quoted names
	types := make([]string, len(schema.Fields))
	for i, f := range schema.Fields {
		idents[i] = quoteIdent(f.Name)
		types[i] = columnType(f, same)
		cols[i] = idents[i] + " " + types[i]
		if !f.Nullable {
			cols[i] += " NOT NULL"
		}
	}

	ddl := "CREATE TABLE IF NOT EXISTS " + qualified + " (\n\t" + strings.Join(cols, ",\n\t") //nolint:gosec // identifiers backtick-quoted via quoteIdent; no user values
	if len(schema.PrimaryKey) > 0 {
		pk := make([]string, len(schema.PrimaryKey))
		for i, c := range schema.PrimaryKey {
			pk[i] = quoteIdent(c)
		}
		ddl += ",\n\tPRIMARY KEY (" + strings.Join(pk, ", ") + ")"
	}
	ddl += "\n)"
	if _, err := t.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("create %s: %w", qualified, err)
	}
	// A resumable table upserts by key and must preserve any partial load from a prior
	// attempt, so it skips the full-snapshot TRUNCATE (kept only when we cannot dedup:
	// a non-resumable table, or a keyless table that re-reads whole on resume).
	mode := t.modeFor(resource)
	resumable := t.resumableFor(resource)
	if mode == filament.WriteReplace {
		// TRUNCATE before any ADD COLUMN so a NOT NULL add lands on an empty table.
		if _, err := t.db.ExecContext(ctx, "TRUNCATE "+qualified); err != nil {
			return fmt.Errorf("truncate %s: %w", qualified, err)
		}
	}
	// MySQL has no ADD COLUMN IF NOT EXISTS; add only the columns the live table lacks.
	existing, err := t.columnSet(ctx, resource)
	if err != nil {
		return fmt.Errorf("columns %s: %w", qualified, err)
	}
	for i, f := range schema.Fields {
		if existing[f.Name] {
			continue
		}
		if _, err := t.db.ExecContext(ctx, "ALTER TABLE "+qualified+" ADD COLUMN "+cols[i]); err != nil {
			return fmt.Errorf("add column on %s: %w", qualified, err)
		}
	}

	// The batches' Arrow schema is a pure function of the RecordSchema, so the
	// renderers are built here once rather than per batch.
	as := arrowbatch.Schema(schema)
	all := make([]int, len(schema.Fields))
	for i := range all {
		all[i] = i
	}
	tbl := &table{qualified: qualified, idents: idents, resumable: resumable || mode == filament.WriteAppend, rows: newLoader(as, all), json: make([]bool, len(types))}
	for i, typ := range types {
		tbl.json[i] = strings.EqualFold(strings.TrimSpace(typ), "json")
	}
	for _, k := range schema.PrimaryKey {
		for i, f := range schema.Fields {
			if f.Name == k {
				tbl.keyIdx = append(tbl.keyIdx, i)
			}
		}
	}
	if len(tbl.keyIdx) > 0 {
		tbl.keys = newLoader(as, tbl.keyIdx)
		tbl.keysTemp = quoteIdent("_filament_" + resource + "_keys")
		keyDefs := make([]string, len(tbl.keyIdx))
		on := make([]string, len(tbl.keyIdx))
		for n, i := range tbl.keyIdx {
			tbl.keyIdents = append(tbl.keyIdents, idents[i])
			keyDefs[n] = idents[i] + " " + types[i]
			on[n] = "t." + idents[i] + " = k." + idents[i]
		}
		tbl.keysTempSQL = "CREATE TEMPORARY TABLE IF NOT EXISTS " + tbl.keysTemp + " (" + strings.Join(keyDefs, ", ") + ")"
		tbl.deleteSQL = "DELETE t FROM " + qualified + " t JOIN " + tbl.keysTemp + " k ON " + strings.Join(on, " AND ")
	}
	t.tables[resource] = tbl
	return nil
}

// columnSet returns the live column names of the destination table.
func (t *Sink) columnSet(ctx context.Context, resource string) (map[string]bool, error) {
	const q = `SELECT COLUMN_NAME FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`
	rows, err := t.db.QueryContext(ctx, q, t.database, resource)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = true
	}
	return out, rows.Err()
}
