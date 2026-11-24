package sqlserver

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

// engine names this source in the schemas it reports.
const engine = "sqlserver"

// table is a table's catalog description, read once per run and shared by its
// shards.
type table struct {
	name       string
	qualified  string // [schema].[table]
	schema     rowmodel.Schema
	selectList string
	cols       []column
	pks        []column // key order
}

// tableMeta reads table's columns in ordinal order and its primary key. CLR
// types report their own name; alias types report the system type under them,
// also when the login cannot see the alias type itself. A CDC source leaves
// out computed columns: change tables hold NULL for them, which an update
// would write over the real value, so snapshot and change rows both omit
// them. A computed key column cannot be left out without merging rows the key
// tells apart, so CDC rejects such a table.
func (s *Source) tableMeta(ctx context.Context, name string) (*table, error) {
	const q = `
SELECT c.name,
	CASE WHEN t.is_assembly_type = 1 THEN t.name ELSE bt.name END,
	c.max_length, c.precision, c.scale, c.is_nullable,
	COALESCE(ic.key_ordinal, 0), COALESCE(c.collation_name, N''), c.is_computed
FROM sys.columns c
LEFT JOIN sys.types t ON t.user_type_id = c.user_type_id
LEFT JOIN sys.types bt ON bt.user_type_id = c.system_type_id
LEFT JOIN sys.indexes i ON i.object_id = c.object_id AND i.is_primary_key = 1
LEFT JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id
	AND ic.column_id = c.column_id AND ic.key_ordinal > 0
WHERE c.object_id = OBJECT_ID(@p1)
ORDER BY c.column_id`
	qualified := quoteIdent(s.schema) + "." + quoteIdent(name)
	rows, err := s.db.QueryContext(ctx, q, qualified)
	if err != nil {
		return nil, fmt.Errorf("columns %q: %w", name, err)
	}
	defer func() { _ = rows.Close() }()
	t := &table{name: name, qualified: qualified}
	for rows.Next() {
		var c column
		var typeName sql.NullString
		var maxLength int16
		var precision, scale uint8
		var computed bool
		if err := rows.Scan(&c.name, &typeName, &maxLength, &precision, &scale, &c.nullable, &c.pkOrder, &c.collation, &computed); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		if computed && s.cdc {
			if c.pkOrder > 0 {
				return nil, fmt.Errorf("%s primary key column %q is computed; CDC change tables hold NULL for computed columns, so the table cannot be read with CDC", qualified, c.name)
			}
			continue
		}
		if !typeName.Valid {
			// A CLR type the login has no permission on has no system type
			// to fall back to.
			return nil, fmt.Errorf("column %q of %q has a type this login cannot see; grant it REFERENCES on the type", c.name, name)
		}
		c.typeName = typeName.String
		c.maxLength, c.precision, c.scale = int(maxLength), int(precision), int(scale)
		t.cols = append(t.cols, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(t.cols) == 0 {
		return nil, fmt.Errorf("resource %q has no columns (missing table, or no permission to see it?)", name)
	}
	for ord := 1; ; ord++ {
		found := false
		for _, c := range t.cols {
			if c.pkOrder == ord {
				t.pks, found = append(t.pks, c), true
			}
		}
		if !found {
			break
		}
	}

	t.schema = rowmodel.Schema{Resource: name, PrimaryKey: columnNames(t.pks), Engine: engine}
	parts := make([]string, len(t.cols))
	for i, c := range t.cols {
		st := typeFor(c)
		ident := "t." + quoteIdent(c.name)
		parts[i] = ident
		if st.selectExpr != nil {
			parts[i] = st.selectExpr(ident) + " AS " + quoteIdent(c.name)
		}
		t.schema.Fields = append(t.schema.Fields, rowmodel.Field{
			Name: c.name, Nullable: c.nullable, Logical: st.logical, Native: c.decl(),
			Precision: st.precision, Scale: st.scale,
		})
	}
	t.selectList = strings.Join(parts, ", ")
	return t, nil
}

// Schema returns one table's row schema and primary key. It implements
// filament.SchemaProvider so a schematized sink can create matching tables.
func (s *Source) Schema(ctx context.Context, resource string) (rowmodel.Schema, error) {
	t, err := s.tableMeta(ctx, resource)
	if err != nil {
		return rowmodel.Schema{}, err
	}
	return t.schema, nil
}

// Discover lists the base tables of the configured schema with their primary
// keys and row counts from partition metadata.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.db == nil {
		return filament.DiscoverResult{}, fmt.Errorf("sqlserver source: discover before configure")
	}
	// One row per primary key column, in key order, or one row with a NULL
	// column for a table without a key. The key is assembled here rather
	// than with STRING_AGG ... WITHIN GROUP, which needs compatibility level
	// 110.
	const q = `
SELECT t.name, c.name,
	COALESCE((
		SELECT SUM(p.rows) FROM sys.partitions p
		WHERE p.object_id = t.object_id AND p.index_id IN (0, 1)
	), 0)
FROM sys.tables t
JOIN sys.schemas s ON s.schema_id = t.schema_id
LEFT JOIN sys.indexes i ON i.object_id = t.object_id AND i.is_primary_key = 1
LEFT JOIN sys.index_columns ic ON ic.object_id = i.object_id AND ic.index_id = i.index_id AND ic.key_ordinal > 0
LEFT JOIN sys.columns c ON c.object_id = ic.object_id AND c.column_id = ic.column_id
WHERE s.name = @p1 AND t.is_ms_shipped = 0
ORDER BY t.name, ic.key_ordinal`
	rows, err := s.db.QueryContext(ctx, q, s.schema)
	if err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("sqlserver source: discover tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var resources []filament.Resource
	for rows.Next() {
		var name string
		var key sql.NullString
		var estimated int64
		if err := rows.Scan(&name, &key, &estimated); err != nil {
			return filament.DiscoverResult{}, fmt.Errorf("sqlserver source: scan table: %w", err)
		}
		if n := len(resources); n == 0 || resources[n-1].Name != name {
			resources = append(resources, filament.Resource{Name: name, Selectable: true, Estimated: estimated})
		}
		if key.Valid {
			r := &resources[len(resources)-1]
			r.PrimaryKey = append(r.PrimaryKey, key.String)
		}
	}
	if err := rows.Err(); err != nil {
		return filament.DiscoverResult{}, fmt.Errorf("sqlserver source: discover rows: %w", err)
	}
	if s.cdc {
		// CDC reads a change by its table's key, so a keyless table cannot
		// be selected; a full refresh reads it whole.
		for i := range resources {
			resources[i].Selectable = len(resources[i].PrimaryKey) > 0
		}
	}
	return filament.DiscoverResult{Resources: resources}, nil
}

// tableRows estimates a table's row count from partition metadata, which any
// login that can see the table may read.
func (s *Source) tableRows(ctx context.Context, qualified string) (int64, error) {
	const q = `SELECT COALESCE(SUM(rows), 0) FROM sys.partitions WHERE object_id = OBJECT_ID(@p1) AND index_id IN (0, 1)`
	var n int64
	err := s.db.QueryRowContext(ctx, q, qualified).Scan(&n)
	return n, err
}

// quoteIdent renders s as a bracket-quoted identifier.
func quoteIdent(s string) string {
	return "[" + strings.ReplaceAll(s, "]", "]]") + "]"
}

func columnNames(cols []column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.name
	}
	return out
}

// columnDecls returns the key declarations of cols, as a checkpoint records
// them.
func columnDecls(cols []column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.keyDecl()
	}
	return out
}
