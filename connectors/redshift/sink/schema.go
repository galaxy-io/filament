package redshift

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

const (
	maxDecimalPrecision = 38
	maxIdentifierBytes  = 127
	maxVarbyteType      = "VARBYTE(16777216)"
)

type columnDefinition struct {
	name       string
	identifier string
	typ        string
	stageType  string
	logical    rowmodel.LogicalType
	sql        string
}

type internalColumn struct {
	name       string
	identifier string
}

type tableDefinition struct {
	name           string
	qualified      string
	columns        []columnDefinition
	keys           []string
	operation      internalColumn
	ordinal        internalColumn
	replacement    string
	createSQL      string
	replacementSQL string
}

// quoteIdent preserves an identifier exactly and escapes embedded quotes.
func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func qualified(identifiers ...string) string {
	quoted := make([]string, len(identifiers))
	for i, identifier := range identifiers {
		quoted[i] = quoteIdent(identifier)
	}
	return strings.Join(quoted, ".")
}

// EnsureSchema creates a typed destination and applies additive columns. A
// full replacement writes into a deterministic run-scoped table until Commit.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, model rowmodel.Schema) error {
	if s.pool == nil {
		return fmt.Errorf("redshift sink: ensure schema before open")
	}
	table, err := defineTable(s.schema, resource, model, s.modeFor(resource), s.run)
	if err != nil {
		return fmt.Errorf("redshift sink: schema for %q: %w", resource, err)
	}
	if _, err := s.pool.Exec(ctx, table.createSQL); err != nil {
		return fmt.Errorf("redshift sink: create table %s: %w", table.qualified, err)
	}
	existing, err := s.columnSet(ctx, resource)
	if err != nil {
		return fmt.Errorf("redshift sink: inspect columns on %s: %w", table.qualified, err)
	}
	for _, column := range table.columns {
		if existing[column.name] {
			continue
		}
		// Redshift supports one ADD COLUMN per ALTER and has no IF NOT EXISTS
		// for regular tables, so the information-schema snapshot gates each DDL.
		statement := "ALTER TABLE " + table.qualified + " ADD COLUMN " + column.sql
		if _, err := s.pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("redshift sink: add column %s on %s: %w", column.identifier, table.qualified, err)
		}
	}

	if table.replacement != "" {
		// Full snapshots have no resumable source checkpoint. Reopening the same
		// run starts its replacement from an empty, exact-schema table.
		if _, err := s.pool.Exec(ctx, "DROP TABLE IF EXISTS "+table.replacement); err != nil {
			return fmt.Errorf("redshift sink: reset replacement for %s: %w", table.qualified, err)
		}
		if _, err := s.pool.Exec(ctx, table.replacementSQL); err != nil {
			return fmt.Errorf("redshift sink: create replacement for %s: %w", table.qualified, err)
		}
		if _, err := s.pool.Exec(ctx, "DELETE FROM "+s.ledger+" WHERE run_id = $1 AND destination = $2", string(s.run), table.qualified); err != nil {
			return fmt.Errorf("redshift sink: reset replacement ledger for %s: %w", table.qualified, err)
		}
	}
	s.tables[resource] = table
	return nil
}

func (s *Sink) columnSet(ctx context.Context, resource string) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2`, s.schema, resource)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		columns[name] = true
	}
	return columns, rows.Err()
}

func defineTable(schema, table string, model rowmodel.Schema, mode filament.WriteMode, run filament.RunID) (tableDefinition, error) {
	if err := validateIdentifier("schema", schema); err != nil {
		return tableDefinition{}, err
	}
	if err := validateIdentifier("table", table); err != nil {
		return tableDefinition{}, err
	}
	if len(model.Fields) == 0 {
		return tableDefinition{}, fmt.Errorf("schema has no fields")
	}

	fields := make(map[string]rowmodel.Field, len(model.Fields))
	columns := make([]columnDefinition, len(model.Fields))
	definitions := make([]string, len(model.Fields))
	for i, field := range model.Fields {
		if err := validateIdentifier("field", field.Name); err != nil {
			return tableDefinition{}, err
		}
		if _, exists := fields[field.Name]; exists {
			return tableDefinition{}, fmt.Errorf("schema contains duplicate field %q", field.Name)
		}
		fields[field.Name] = field
		typ, stageType := columnTypes(field)
		identifier := quoteIdent(field.Name)
		definition := identifier + " " + typ
		if !field.Nullable {
			definition += " NOT NULL"
		}
		columns[i] = columnDefinition{
			name: field.Name, identifier: identifier, typ: typ,
			stageType: stageType, logical: field.Logical, sql: definition,
		}
		definitions[i] = definition
	}

	keys := make([]string, len(model.PrimaryKey))
	seenKeys := make(map[string]bool, len(model.PrimaryKey))
	for i, key := range model.PrimaryKey {
		field, exists := fields[key]
		if !exists {
			return tableDefinition{}, fmt.Errorf("primary-key field %q is absent from schema", key)
		}
		if seenKeys[key] {
			return tableDefinition{}, fmt.Errorf("primary key contains duplicate field %q", key)
		}
		if field.Logical == rowmodel.LogicalJSON {
			return tableDefinition{}, fmt.Errorf("primary-key field %q maps to unsupported Redshift SUPER type", key)
		}
		seenKeys[key] = true
		keys[i] = quoteIdent(key)
	}
	if len(keys) > 0 {
		// Keep the key inline with CREATE TABLE IF NOT EXISTS. On a rerun,
		// Redshift leaves the existing table and constraint untouched instead
		// of attempting to create the primary key again.
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(keys, ", ")+")")
	}

	name := qualified(schema, table)
	operation := uniqueInternalColumn(model.Fields, "_filament_internal_operation")
	ordinal := uniqueInternalColumn(model.Fields, "_filament_internal_ordinal")
	definition := tableDefinition{
		name: table, qualified: name, columns: columns, keys: keys,
		operation: operation, ordinal: ordinal,
		createSQL: createTableSQL(name, definitions),
	}
	if mode == filament.WriteReplace {
		hash := sha256.Sum256([]byte(name + "\x00" + string(run)))
		definition.replacement = qualified(schema, fmt.Sprintf("_filament_replace_%x", hash[:10]))
		replacementDefinitions := make([]string, len(columns))
		for i, column := range columns {
			replacementDefinitions[i] = column.sql
		}
		definition.replacementSQL = createTableSQL(definition.replacement, replacementDefinitions)
	}
	return definition, nil
}

func createTableSQL(name string, definitions []string) string {
	return "CREATE TABLE IF NOT EXISTS " + name + " (\n\t" + strings.Join(definitions, ",\n\t") +
		"\n) DISTSTYLE AUTO SORTKEY AUTO ENCODE AUTO"
}

func validateIdentifier(kind, identifier string) error {
	if strings.TrimSpace(identifier) == "" {
		return fmt.Errorf("%s name is empty", kind)
	}
	if len([]byte(identifier)) > maxIdentifierBytes {
		return fmt.Errorf("%s name %q exceeds Redshift's %d-byte identifier limit", kind, identifier, maxIdentifierBytes)
	}
	return nil
}

func uniqueInternalColumn(fields []rowmodel.Field, base string) internalColumn {
	used := make(map[string]bool, len(fields))
	for _, field := range fields {
		used[field.Name] = true
	}
	name := base
	for used[name] {
		name += "_"
	}
	return internalColumn{name: name, identifier: quoteIdent(name)}
}

func columnTypes(field rowmodel.Field) (target, stage string) {
	switch field.Logical {
	case rowmodel.LogicalBool:
		return "BOOLEAN", "BOOLEAN"
	case rowmodel.LogicalInt16:
		return "SMALLINT", "SMALLINT"
	case rowmodel.LogicalInt32:
		return "INTEGER", "INTEGER"
	case rowmodel.LogicalInt64:
		return "BIGINT", "BIGINT"
	case rowmodel.LogicalFloat32:
		return "REAL", "REAL"
	case rowmodel.LogicalFloat64:
		return "DOUBLE PRECISION", "DOUBLE PRECISION"
	case rowmodel.LogicalDecimal:
		if field.Precision > 0 && field.Precision <= maxDecimalPrecision && field.Scale >= 0 && field.Scale <= field.Precision {
			typ := fmt.Sprintf("DECIMAL(%d,%d)", field.Precision, field.Scale)
			return typ, typ
		}
		return "VARCHAR(MAX)", "VARCHAR(MAX)"
	case rowmodel.LogicalBytes:
		return maxVarbyteType, maxVarbyteType
	case rowmodel.LogicalDate:
		return "DATE", "DATE"
	case rowmodel.LogicalTime:
		return "TIME", "TIME"
	case rowmodel.LogicalTimestamp:
		return "TIMESTAMP", "TIMESTAMP"
	case rowmodel.LogicalTimestampTZ:
		return "TIMESTAMPTZ", "TIMESTAMPTZ"
	case rowmodel.LogicalJSON:
		// Parquet carries Filament JSON as UTF-8 text. COPY it into VARCHAR,
		// then parse once while projecting into the typed destination.
		return "SUPER", "VARCHAR(MAX)"
	case rowmodel.LogicalUUID:
		return "VARCHAR(36)", "VARCHAR(36)"
	default:
		return "VARCHAR(MAX)", "VARCHAR(MAX)"
	}
}
