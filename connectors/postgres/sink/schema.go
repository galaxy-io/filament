package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// EnsureSchema creates resource's typed table from schema (Postgres column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and prepares the
// resource's COPY and fold statements. A pre-existing table gains any new columns
// (ADD COLUMN IF NOT EXISTS); an incompatible existing column type surfaces later as
// a COPY error (full type-change handling is deferred to schema evolution).
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if t.pool == nil {
		return fmt.Errorf("postgres sink: ensure schema before open")
	}
	qualified := pgx.Identifier{t.schema, resource}.Sanitize()
	var builtin map[string]bool
	if schema.Engine == engine {
		var err error
		if builtin, err = t.builtinTypes(ctx, schema); err != nil {
			return err
		}
	}

	cols := make([]string, len(schema.Fields)) // "name" type [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields))
	types := make([]string, len(schema.Fields))
	for i, f := range schema.Fields {
		idents[i] = pgx.Identifier{f.Name}.Sanitize()
		types[i] = columnType(f, builtin[f.Native])
		cols[i] = idents[i] + " " + types[i]
		if !f.Nullable {
			cols[i] += " NOT NULL"
		}
	}
	ddl := "CREATE TABLE IF NOT EXISTS " + qualified + " (\n\t" + strings.Join(cols, ",\n\t")
	if len(schema.PrimaryKey) > 0 {
		pk := make([]string, len(schema.PrimaryKey))
		for i, c := range schema.PrimaryKey {
			pk[i] = pgx.Identifier{c}.Sanitize()
		}
		ddl += ",\n\tPRIMARY KEY (" + strings.Join(pk, ", ") + ")"
	}
	ddl += "\n)"
	if _, err := t.pool.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("create %s: %w", qualified, err)
	}
	// A resumable table upserts by key and must preserve any partial load from a prior
	// attempt, so it skips the full-snapshot TRUNCATE (kept only when we cannot dedup:
	// a non-resumable table, or a keyless table that re-reads whole on resume).
	mode := t.modeFor(resource)
	resumable := t.resumableFor(resource)
	if mode == filament.WriteReplace {
		// TRUNCATE before any ADD COLUMN so a NOT NULL add lands on an empty table.
		if _, err := t.pool.Exec(ctx, "TRUNCATE "+qualified); err != nil {
			return fmt.Errorf("truncate %s: %w", qualified, err)
		}
	}
	for i := range schema.Fields {
		if _, err := t.pool.Exec(ctx, "ALTER TABLE "+qualified+" ADD COLUMN IF NOT EXISTS "+cols[i]); err != nil {
			return fmt.Errorf("add column on %s: %w", qualified, err)
		}
	}

	tbl := &table{
		qualified: qualified,
		idents:    idents,
		types:     types,
		resumable: resumable || mode == filament.WriteAppend,
		copier:    map[*arrow.Schema]*copier{},
		keys:      map[*arrow.Schema]*copier{},
	}
	for _, k := range schema.PrimaryKey {
		for i, f := range schema.Fields {
			if f.Name == k {
				tbl.keyIdx = append(tbl.keyIdx, i)
			}
		}
	}
	if len(tbl.keyIdx) > 0 {
		tbl.temp = pgx.Identifier{"_filament_" + resource}.Sanitize()
		tbl.keysTemp = pgx.Identifier{"_filament_" + resource + "_keys"}.Sanitize()
		tbl.tempSQL = tempTableSQL(tbl.temp, idents, types, true)
		tbl.upsertSQL = upsertSQL(tbl)
		keyIdents, keyTypes := pick(idents, tbl.keyIdx), pick(types, tbl.keyIdx)
		tbl.keysTempSQL = tempTableSQL(tbl.keysTemp, keyIdents, keyTypes, false)
		tbl.deleteSQL = deleteSQL(tbl.qualified, tbl.keysTemp, keyIdents)
	}
	t.tables[resource] = tbl
	return nil
}

// engine is the source engine whose native type spellings this sink reuses.
const engine = "postgres"

// builtinTypes reports which of a same-engine schema's native type spellings
// are Postgres built-ins (pg_catalog, arrays included). Only those keep their
// spelling in the destination; a user-defined type — an enum, a domain, an
// extension type — exists only in the source database, so its column lands as
// text.
func (t *Sink) builtinTypes(ctx context.Context, schema rowmodel.Schema) (map[string]bool, error) {
	natives := make([]string, 0, len(schema.Fields))
	for _, f := range schema.Fields {
		if f.Native != "" {
			natives = append(natives, f.Native)
		}
	}
	const q = `SELECT t, coalesce((SELECT typnamespace = 'pg_catalog'::regnamespace FROM pg_type WHERE oid = to_regtype(t)), false)
FROM unnest($1::text[]) AS t`
	rows, err := t.pool.Query(ctx, q, natives)
	if err != nil {
		return nil, fmt.Errorf("resolve types: %w", err)
	}
	defer rows.Close()
	builtin := make(map[string]bool, len(natives))
	for rows.Next() {
		var native string
		var ok bool
		if err := rows.Scan(&native, &ok); err != nil {
			return nil, fmt.Errorf("resolve types: %w", err)
		}
		builtin[native] = ok
	}
	return builtin, rows.Err()
}

// columnType picks a destination column type: the source's own spelling for a
// built-in type on the same engine (keepNative), else a portable mapping from
// the logical type.
func columnType(f rowmodel.Field, keepNative bool) string {
	if keepNative && f.Native != "" {
		return f.Native
	}
	switch f.Logical {
	case rowmodel.LogicalBool:
		return "boolean"
	case rowmodel.LogicalInt16:
		return "smallint"
	case rowmodel.LogicalInt32:
		return "integer"
	case rowmodel.LogicalInt64:
		return "bigint"
	case rowmodel.LogicalFloat32:
		return "real"
	case rowmodel.LogicalFloat64:
		return "double precision"
	case rowmodel.LogicalDecimal:
		if f.Precision > 0 {
			return fmt.Sprintf("numeric(%d,%d)", f.Precision, f.Scale)
		}
		return "numeric"
	case rowmodel.LogicalBytes:
		return "bytea"
	case rowmodel.LogicalDate:
		return "date"
	case rowmodel.LogicalTime:
		return "time"
	case rowmodel.LogicalTimestamp:
		return "timestamp"
	case rowmodel.LogicalTimestampTZ:
		return "timestamptz"
	case rowmodel.LogicalJSON:
		return "jsonb"
	case rowmodel.LogicalUUID:
		return "uuid"
	default:
		return "text"
	}
}

// tempTableSQL creates the per-connection scratch table a key-based fold loads
// into: the given columns, nullable, plus _ord (row order within the batch) when
// ordered. It is emptied before every load; a merge folds several runs through
// it inside one transaction.
func tempTableSQL(name string, idents, types []string, ordered bool) string {
	cols := make([]string, len(idents), len(idents)+1)
	for i := range idents {
		cols[i] = idents[i] + " " + types[i]
	}
	if ordered {
		cols = append(cols, "_ord integer")
	}
	return "CREATE TEMP TABLE IF NOT EXISTS " + name + " (" + strings.Join(cols, ", ") + "); TRUNCATE " + name
}

// upsertSQL folds the temp table into the destination: the last row per key wins
// within the batch, then INSERT … ON CONFLICT DO UPDATE by primary key makes the
// write idempotent (an at-least-once resume re-delivers rows past the last
// persisted cursor).
func upsertSQL(tbl *table) string {
	keys := pick(tbl.idents, tbl.keyIdx)
	keySet := map[string]bool{}
	for _, k := range keys {
		keySet[k] = true
	}
	var sets []string
	for _, id := range tbl.idents {
		if !keySet[id] {
			sets = append(sets, id+" = excluded."+id)
		}
	}
	cols := strings.Join(tbl.idents, ", ")
	q := fmt.Sprintf("INSERT INTO %s (%s) SELECT DISTINCT ON (%s) %s FROM %s ORDER BY %s, _ord DESC ON CONFLICT (%s) DO ",
		tbl.qualified, cols, strings.Join(keys, ", "), cols, tbl.temp, strings.Join(keys, ", "), strings.Join(keys, ", "))
	if len(sets) == 0 {
		return q + "NOTHING"
	}
	return q + "UPDATE SET " + strings.Join(sets, ", ")
}

// deleteSQL removes every destination row whose key is in the keys temp table.
func deleteSQL(qualified, keysTemp string, keyIdents []string) string {
	where := make([]string, len(keyIdents))
	for i, k := range keyIdents {
		where[i] = "t." + k + " = k." + k
	}
	return fmt.Sprintf("DELETE FROM %s AS t USING %s AS k WHERE %s", qualified, keysTemp, strings.Join(where, " AND "))
}

func pick[T any](all []T, idx []int) []T {
	out := make([]T, len(idx))
	for n, i := range idx {
		out[n] = all[i]
	}
	return out
}
