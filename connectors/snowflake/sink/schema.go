package snowflake

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

const maxNumberPrecision = 38

type columnDefinition struct {
	name       string
	identifier string
	typ        string
	logical    rowmodel.LogicalType
	sql        string
}

type tableDefinition struct {
	qualified     string
	columns       []columnDefinition
	keys          []string
	operation     internalColumn
	ordinal       internalColumn
	temporary     string
	createSQL     string
	createTempSQL string
	mergeSQL      string
	replace       bool
}

type internalColumn struct {
	name       string
	identifier string
}

// quoteIdent preserves an identifier exactly as supplied and escapes embedded
// quotes according to Snowflake's delimited-identifier syntax.
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

func createSchemaDDL(database, schema string) (string, error) {
	if strings.TrimSpace(database) == "" {
		return "", fmt.Errorf("database name is empty")
	}
	if strings.TrimSpace(schema) == "" {
		return "", fmt.Errorf("schema name is empty")
	}
	return "CREATE SCHEMA IF NOT EXISTS " + qualified(database, schema), nil
}

// EnsureSchema creates a typed table and adds newly discovered columns. It
// deliberately does not drop, rename, or alter existing columns.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if s.db == nil {
		return fmt.Errorf("snowflake sink: ensure schema before open")
	}
	table, err := defineTable(s.database, s.schema, resource, schema)
	if err != nil {
		return fmt.Errorf("snowflake sink: schema for %q: %w", resource, err)
	}
	if _, err := s.db.ExecContext(ctx, table.createSQL); err != nil {
		return fmt.Errorf("snowflake sink: create table %s: %w", table.qualified, err)
	}
	if s.modeFor(resource) == filament.WriteReplace {
		if _, err := s.db.ExecContext(ctx, "TRUNCATE TABLE "+table.qualified); err != nil { //nolint:gosec // identifier is quoted
			return fmt.Errorf("snowflake sink: truncate table %s: %w", table.qualified, err)
		}
		table.replace = true
	}
	existing, err := s.columnSet(ctx, resource)
	if err != nil {
		return fmt.Errorf("snowflake sink: inspect columns on %s: %w", table.qualified, err)
	}
	for _, column := range table.columns {
		if existing[column.name] {
			continue
		}
		//nolint:gosec // Identifiers are quoted and type spellings come only from columnType.
		ddl := "ALTER TABLE " + table.qualified +
			" ADD COLUMN IF NOT EXISTS " + column.sql
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("snowflake sink: add column %s on %s: %w", column.identifier, table.qualified, err)
		}
	}
	s.tables[resource] = table
	return nil
}

func (s *Sink) columnSet(ctx context.Context, resource string) (map[string]bool, error) {
	//nolint:gosec // The database and information-schema identifiers are quoted.
	query := "SELECT COLUMN_NAME FROM " + qualified(s.database, "INFORMATION_SCHEMA", "COLUMNS") +
		" WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?"
	rows, err := s.db.QueryContext(ctx, query, s.schema, resource)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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

// defineTable validates a portable schema and renders the DDL used by
// EnsureSchema. Primary keys are informational constraints in Snowflake and
// also drive merge-key selection for keyed write modes.
func defineTable(database, schema, table string, model rowmodel.Schema) (tableDefinition, error) {
	if strings.TrimSpace(database) == "" {
		return tableDefinition{}, fmt.Errorf("database name is empty")
	}
	if strings.TrimSpace(schema) == "" {
		return tableDefinition{}, fmt.Errorf("schema name is empty")
	}
	if strings.TrimSpace(table) == "" {
		return tableDefinition{}, fmt.Errorf("table name is empty")
	}
	if len(model.Fields) == 0 {
		return tableDefinition{}, fmt.Errorf("schema has no fields")
	}

	fields := make(map[string]bool, len(model.Fields))
	columns := make([]columnDefinition, len(model.Fields))
	definitions := make([]string, len(model.Fields))
	for i, field := range model.Fields {
		if field.Name == "" {
			return tableDefinition{}, fmt.Errorf("schema contains an empty field name")
		}
		if fields[field.Name] {
			return tableDefinition{}, fmt.Errorf("schema contains duplicate field %q", field.Name)
		}
		fields[field.Name] = true
		identifier := quoteIdent(field.Name)
		typ := columnType(field)
		definition := identifier + " " + typ
		if !field.Nullable {
			definition += " NOT NULL"
		}
		columns[i] = columnDefinition{name: field.Name, identifier: identifier, typ: typ, logical: field.Logical, sql: definition}
		definitions[i] = definition
	}

	keys := make([]string, len(model.PrimaryKey))
	seenKeys := make(map[string]bool, len(model.PrimaryKey))
	for i, key := range model.PrimaryKey {
		if !fields[key] {
			return tableDefinition{}, fmt.Errorf("primary-key field %q is absent from schema", key)
		}
		if seenKeys[key] {
			return tableDefinition{}, fmt.Errorf("primary key contains duplicate field %q", key)
		}
		seenKeys[key] = true
		keys[i] = quoteIdent(key)
	}
	if len(keys) > 0 {
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(keys, ", ")+")")
	}

	name := qualified(database, schema, table)
	operation := uniqueInternalColumn(model.Fields, "_filament_internal_operation")
	ordinal := uniqueInternalColumn(model.Fields, "_filament_internal_ordinal")
	definition := tableDefinition{
		qualified: name,
		columns:   columns,
		keys:      keys,
		operation: operation,
		ordinal:   ordinal,
		createSQL: "CREATE TABLE IF NOT EXISTS " + name + " (\n\t" + strings.Join(definitions, ",\n\t") + "\n)",
	}
	if len(keys) > 0 {
		tableHash := sha256.Sum256([]byte(name))
		definition.temporary = qualified(database, schema, fmt.Sprintf("_filament_load_%x", tableHash[:8]))
		definition.createTempSQL = createTempTableSQL(definition)
		definition.mergeSQL = mergeTableSQL(definition)
	}
	return definition, nil
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

func createTempTableSQL(table tableDefinition) string {
	definitions := make([]string, 0, len(table.columns)+2)
	for _, column := range table.columns {
		definitions = append(definitions, column.identifier+" "+column.typ)
	}
	definitions = append(definitions,
		table.operation.identifier+" NUMBER(38,0) NOT NULL",
		table.ordinal.identifier+" NUMBER(38,0) NOT NULL",
	)
	return "CREATE OR REPLACE TEMPORARY TABLE " + table.temporary + " (" + strings.Join(definitions, ", ") + ")"
}

// mergeTableSQL reduces a batch to its final operation per primary key, then
// applies inserts, updates, and deletes in one atomic Snowflake statement.
func mergeTableSQL(table tableDefinition) string {
	columns := make([]string, len(table.columns))
	keySet := make(map[string]bool, len(table.keys))
	for _, key := range table.keys {
		keySet[key] = true
	}
	updates := make([]string, 0, len(table.columns))
	inserts := make([]string, len(table.columns))
	for i, column := range table.columns {
		columns[i] = column.identifier
		inserts[i] = "s." + column.identifier
		if !keySet[column.identifier] {
			updates = append(updates, column.identifier+" = s."+column.identifier)
		}
	}
	joins := make([]string, len(table.keys))
	for i, key := range table.keys {
		joins[i] = "EQUAL_NULL(t." + key + ", s." + key + ")"
	}

	sourceColumns := append(append([]string(nil), columns...), table.operation.identifier, table.ordinal.identifier)
	statement := "MERGE INTO " + table.qualified + " AS t USING (SELECT " + strings.Join(sourceColumns, ", ") +
		" FROM " + table.temporary + " QUALIFY ROW_NUMBER() OVER (PARTITION BY " + strings.Join(table.keys, ", ") +
		" ORDER BY " + table.ordinal.identifier + " DESC) = 1) AS s ON " + strings.Join(joins, " AND ") +
		fmt.Sprintf(" WHEN MATCHED AND s.%s = %d THEN DELETE", table.operation.identifier, rowmodel.OpDelete)
	if len(updates) > 0 {
		statement += " WHEN MATCHED THEN UPDATE SET " + strings.Join(updates, ", ")
	}
	return statement + fmt.Sprintf(" WHEN NOT MATCHED AND s.%s <> %d THEN INSERT (%s) VALUES (%s)",
		table.operation.identifier, rowmodel.OpDelete, strings.Join(columns, ", "), strings.Join(inserts, ", "))
}

// columnType maps portable logical types to stable Snowflake types. Types that
// cannot be represented losslessly without more source metadata land as TEXT.
func columnType(field rowmodel.Field) string {
	switch field.Logical {
	case rowmodel.LogicalBool:
		return "BOOLEAN"
	case rowmodel.LogicalInt16, rowmodel.LogicalInt32, rowmodel.LogicalInt64:
		return "NUMBER(38,0)"
	case rowmodel.LogicalFloat32, rowmodel.LogicalFloat64:
		return "FLOAT"
	case rowmodel.LogicalDecimal:
		if field.Precision > 0 && field.Precision <= maxNumberPrecision &&
			field.Scale >= 0 && field.Scale <= field.Precision {
			return fmt.Sprintf("NUMBER(%d,%d)", field.Precision, field.Scale)
		}
		return "TEXT"
	case rowmodel.LogicalBytes:
		return "BINARY"
	case rowmodel.LogicalDate:
		return "DATE"
	case rowmodel.LogicalTime:
		return "TIME(6)"
	case rowmodel.LogicalTimestamp:
		return "TIMESTAMP_NTZ(6)"
	case rowmodel.LogicalTimestampTZ:
		return "TIMESTAMP_TZ(6)"
	case rowmodel.LogicalJSON:
		return "VARIANT"
	default:
		return "TEXT"
	}
}
