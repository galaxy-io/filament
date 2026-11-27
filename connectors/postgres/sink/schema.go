package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// EnsureSchema creates resource's typed table from schema (Postgres column types, NOT
// NULL, primary key), empties it for full-snapshot replace semantics, and prepares the
// resource's COPY and fold statements. A pre-existing table gains any new columns
// (ADD COLUMN IF NOT EXISTS) in legacy mode. With a schema policy enabled, the
// entire diff is checked before transactional DDL or replace-mode truncation.
func (t *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if t.transactionalWrites {
		t.writeMu.Lock()
		defer t.writeMu.Unlock()
	}
	if t.transactionalWrites && t.modeFor(resource) != filament.WriteAppend && t.modeFor(resource) != filament.WriteMerge && t.modeFor(resource) != filament.WriteUpsert {
		return filament.ErrEpochMismatch
	}
	if t.pool == nil {
		return fmt.Errorf("postgres sink: ensure schema before open")
	}
	// Within write epochs, inferred columns must be installed atomically with
	// the first batch, using the active epoch transaction (and its connection).
	exec := t.pool.Exec
	if t.transactionalWrites && t.session.active != nil {
		exec = t.session.active.tx.Exec
	}
	qualified := pgx.Identifier{t.schema, resource}.Sanitize()
	checked, builtin, err := t.prepareColumnTypes(ctx, resource, schema)
	if err != nil {
		return err
	}

	cols := make([]string, len(schema.Fields)) // "name" type [NOT NULL] for DDL
	idents := make([]string, len(schema.Fields))
	types := make([]string, len(schema.Fields))
	for i, f := range schema.Fields {
		idents[i] = pgx.Identifier{f.Name}.Sanitize()
		types[i] = columnType(f, builtin[f.Native])
		if t.evolution != "" {
			types[i] = checked.Fields[i].Native
		}
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
	mode := t.modeFor(resource)
	resumable := t.resumableFor(resource)
	if t.evolution != "" {
		if err := t.reconcileSchema(ctx, checked, ddl); err != nil {
			return err
		}
	} else {
		if _, err := exec(ctx, ddl); err != nil {
			return fmt.Errorf("create %s: %w", qualified, err)
		}
		// Only explicit replace mode clears rows from earlier attempts.
		if mode == filament.WriteReplace {
			// TRUNCATE before any ADD COLUMN so a NOT NULL add lands on an empty table.
			if _, err := exec(ctx, "TRUNCATE "+qualified); err != nil {
				return fmt.Errorf("truncate %s: %w", qualified, err)
			}
		}
		for i := range schema.Fields {
			if _, err := exec(ctx, "ALTER TABLE "+qualified+" ADD COLUMN IF NOT EXISTS "+cols[i]); err != nil {
				return fmt.Errorf("add column on %s: %w", qualified, err)
			}
		}

	}

	tbl := &table{
		model:     checked,
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
		if t.evolution == "" {
			casts, err := t.destinationCasts(ctx, resource, schema.Fields, types)
			if err != nil {
				return err
			}
			tbl.castTo = casts
		}
		tbl.tempSQL = tempTableSQL(tbl.temp, idents, types, true)
		tbl.upsertSQL = upsertSQL(tbl)
		keyIdents, keyTypes := pick(idents, tbl.keyIdx), pick(types, tbl.keyIdx)
		tbl.keysTempSQL = tempTableSQL(tbl.keysTemp, keyIdents, keyTypes, false)
		tbl.deleteSQL = deleteSQL(tbl)
	}
	t.installTable(resource, tbl, schema)
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
	query := t.pool.Query
	if t.session.active != nil {
		query = t.session.active.tx.Query
	}
	rows, err := query(ctx, q, natives)
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

// destinationCasts returns, per column in fields order, the destination type
// to cast staged values to, or "" when a staged value lands as-is. A cast is
// needed only for a user-defined destination type - an enum, a domain, an
// extension type - where Postgres has no assignment cast from the staged
// form to it, so the fold fails with 42804. A pg_catalog built-in
// (varchar, numeric, timestamptz) takes the staged value through its
// assignment cast, and the same type under another spelling needs none.
// A domain whose base type does take the staged value by assignment -
// text into a domain over varchar(n), for example - keeps the uncast
// assignment path, so an over-length value errors like assignment or COPY
// would instead of being silently truncated by the explicit cast.
// Only the legacy path needs this: a schema policy blocks user-defined
// native types before any write, so a destination under one can only hold
// them in legacy mode.
func (t *Sink) destinationCasts(ctx context.Context, resource string, fields []rowmodel.Field, staged []string) ([]string, error) {
	// checkOID is the destination type's assignment target: the base type
	// for a domain, the element type for an array, unwrapped one level
	// each, so a domain over varchar and an array of enums both resolve
	// to the type the staged value is actually assigned to.
	const q = `SELECT a.attname, pg_catalog.format_type(a.atttypid, a.atttypmod),
		t.typnamespace = 'pg_catalog'::regnamespace,
		tn.nspname,
		COALESCE(NULLIF(et.typbasetype, 0), et.oid)
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_type t ON t.oid = a.atttypid
		JOIN pg_catalog.pg_namespace tn ON tn.oid = t.typnamespace
		JOIN pg_catalog.pg_type bt ON bt.oid = CASE WHEN t.typtype = 'd' THEN t.typbasetype ELSE t.oid END
		JOIN pg_catalog.pg_type et ON et.oid = CASE WHEN bt.typelem <> 0 THEN bt.typelem ELSE bt.oid END
		WHERE n.nspname = $1 AND c.relname = $2 AND a.attnum > 0 AND NOT a.attisdropped`
	query := t.pool.Query
	if t.session.active != nil {
		query = t.session.active.tx.Query
	}
	rows, err := query(ctx, q, t.schema, resource)
	if err != nil {
		return nil, fmt.Errorf("resolve destination types: %w", err)
	}
	defer rows.Close()
	type destType struct {
		typ      string
		builtin  bool
		schema   string
		checkOID uint32
	}
	byName := make(map[string]destType)
	for rows.Next() {
		var name string
		var d destType
		if err := rows.Scan(&name, &d.typ, &d.builtin, &d.schema, &d.checkOID); err != nil {
			return nil, fmt.Errorf("resolve destination types: %w", err)
		}
		byName[name] = d
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("resolve destination types: %w", err)
	}
	casts := make([]string, len(fields))
	for i, f := range fields {
		d, ok := byName[f.Name]
		if !ok || d.builtin {
			continue
		}
		assignable, err := t.assignmentCastExists(ctx, staged[i], d.checkOID)
		if err != nil {
			return nil, err
		}
		casts[i] = castFor(staged[i], qualifyCastTarget(d.schema, d.typ), d.builtin, assignable)
	}
	return casts, nil
}

// assignmentCastExists reports whether a staged value of the given type
// lands in the destination type by assignment: either it is the same type,
// or pg_cast holds an assignment or implicit cast from it. The check runs
// against the destination's unwrapped base type, so a domain over varchar
// answers for its base type. Element types are compared for arrays:
// pg_cast holds no array-to-array rows, array casts are derived from the
// element cast.
func (t *Sink) assignmentCastExists(ctx context.Context, staged string, checkOID uint32) (bool, error) {
	const q = `SELECT COALESCE(to_regtype($1) = $2::oid, false) OR EXISTS (
		SELECT 1 FROM pg_catalog.pg_cast
		WHERE castsource = to_regtype($1) AND casttarget = $2::oid AND castcontext IN ('a', 'i')
	)`
	query := t.pool.QueryRow
	if t.session.active != nil {
		query = t.session.active.tx.QueryRow
	}
	var ok bool
	if err := query(ctx, q, stagedElementType(staged), checkOID).Scan(&ok); err != nil {
		return false, fmt.Errorf("resolve assignment cast: %w", err)
	}
	return ok, nil
}

// stagedElementType strips the array suffix from a staged column type, so
// the assignment-cast check compares element types.
func stagedElementType(staged string) string {
	for strings.HasSuffix(staged, "[]") {
		staged = strings.TrimSuffix(staged, "[]")
	}
	return staged
}

// qualifyCastTarget builds the schema-qualified CAST target for a
// user-defined destination type from its schema and the format_type
// spelling, keeping any array suffix and type modifier. The schema comes
// from pg_type, not from format_type: format_type qualifies only when the
// type is not visible on the current search_path, and the merge runs under
// a different one, so an unqualified name could resolve to the staging
// temp table's row type instead of the destination type.
func qualifyCastTarget(schema, formatted string) string {
	arrays := ""
	for strings.HasSuffix(formatted, "[]") {
		arrays += "[]"
		formatted = strings.TrimSuffix(formatted, "[]")
	}
	modifier := ""
	if i := strings.IndexByte(formatted, '('); i >= 0 {
		modifier = formatted[i:]
		formatted = formatted[:i]
	}
	name := formatted
	if i := strings.LastIndexByte(formatted, '.'); i >= 0 {
		name = formatted[i+1:]
	}
	name = strings.Trim(name, `"`)
	return pgx.Identifier{schema, name}.Sanitize() + modifier + arrays
}

// castFor reports the destination type to cast a staged value of staged type
// to, or "" when the value lands as-is. The destination type is already
// schema-qualified by the caller.
func castFor(staged, dest string, builtin, assignable bool) string {
	if dest == "" || builtin || assignable || baseType(dest) == baseType(staged) {
		return ""
	}
	return dest
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
// persisted cursor). A staged column whose destination type is user-defined
// and takes no assignment cast is cast to it in the SELECT list, so enum
// and domain columns fold instead of failing with 42804. A destination
// that takes the staged value by assignment keeps the uncast path, so an
// over-length value errors instead of being silently truncated.
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
	selects := make([]string, len(tbl.idents))
	for i, id := range tbl.idents {
		selects[i] = id
		if i < len(tbl.castTo) && tbl.castTo[i] != "" {
			selects[i] = "CAST(" + id + " AS " + tbl.castTo[i] + ")"
		}
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) SELECT DISTINCT ON (%s) %s FROM %s ORDER BY %s, _ord DESC ON CONFLICT (%s) DO ",
		tbl.qualified, cols, strings.Join(keys, ", "), strings.Join(selects, ", "), tbl.temp, strings.Join(keys, ", "), strings.Join(keys, ", "))
	if len(sets) == 0 {
		return q + "NOTHING"
	}
	return q + "UPDATE SET " + strings.Join(sets, ", ")
}

// deleteSQL removes every destination row whose key is in the keys temp table.
// A key landing in a user-defined destination type is cast to it, so an enum
// or domain primary key matches instead of comparing across types.
func deleteSQL(tbl *table) string {
	keys := pick(tbl.idents, tbl.keyIdx)
	where := make([]string, len(keys))
	for n, k := range keys {
		rhs := "k." + k
		if i := tbl.keyIdx[n]; i < len(tbl.castTo) && tbl.castTo[i] != "" {
			rhs = "CAST(k." + k + " AS " + tbl.castTo[i] + ")"
		}
		where[n] = "t." + k + " = " + rhs
	}
	return fmt.Sprintf("DELETE FROM %s AS t USING %s AS k WHERE %s", tbl.qualified, tbl.keysTemp, strings.Join(where, " AND "))
}

func pick[T any](all []T, idx []int) []T {
	out := make([]T, len(idx))
	for n, i := range idx {
		out[n] = all[i]
	}
	return out
}

func (t *Sink) installTable(resource string, tbl *table, model rowmodel.Schema) {
	t.tablesMu.Lock()
	defer t.tablesMu.Unlock()
	if t.evolution != "" {
		tbl.input = arrowbatch.Schema(model)
	}
	if t.transactionalWrites && t.session.active != nil {
		epoch := t.session.active
		if epoch.previousTables == nil {
			epoch.previousTables = map[string]*table{}
		}
		if _, saved := epoch.previousTables[resource]; !saved {
			epoch.previousTables[resource] = t.tables[resource]
		}
	}
	t.tables[resource] = tbl
}

func (t *Sink) prepareColumnTypes(ctx context.Context, resource string, model rowmodel.Schema) (rowmodel.Schema, map[string]bool, error) {
	if t.evolution != "" {
		checked, err := physicalSchema(resource, model)
		return checked, nil, err
	}
	if model.Engine == engine {
		builtin, err := t.builtinTypes(ctx, model)
		return rowmodel.Schema{}, builtin, err
	}
	return rowmodel.Schema{}, nil, nil
}
