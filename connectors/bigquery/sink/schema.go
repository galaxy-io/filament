package bigquery

import (
	"context"
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

const (
	maxDecimalPrecision      = 38
	maxNumericScale          = 9
	maxNumericIntegralDigits = 29
	maxBigNumericScale       = 38
	maxBigNumericIntegral    = 38
)

var datasetPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type tableDefinition struct {
	name        string
	qualified   string
	columns     []columnDefinition
	keys        []string
	operation   internalColumn
	ordinal     internalColumn
	replacement string
	createSQL   string
	alterSQL    string
}

type columnDefinition struct {
	name       string
	identifier string
	typ        string
	logical    rowmodel.LogicalType
	sql        string
}

type internalColumn struct {
	name       string
	identifier string
}

// quoteIdent preserves an identifier exactly as supplied using GoogleSQL's
// backtick-quoted identifier syntax.
func quoteIdent(identifier string) string {
	escaped := strings.NewReplacer(
		`\`, `\\`,
		"`", `\`+"`",
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
	).Replace(identifier)
	return "`" + escaped + "`"
}

func qualified(identifiers ...string) string {
	return quoteIdent(strings.Join(identifiers, "."))
}

func createDatasetDDL(project, dataset, location string) (string, error) {
	if strings.TrimSpace(project) == "" {
		return "", fmt.Errorf("project ID is empty")
	}
	if !validDataset(dataset) {
		return "", fmt.Errorf("dataset must contain 1 to 1024 letters, numbers, or underscores")
	}
	ddl := "CREATE SCHEMA IF NOT EXISTS " + qualified(project, dataset)
	if location != "" {
		ddl += " OPTIONS(location = '" + location + "')"
	}
	return ddl, nil
}

// EnsureSchema creates a typed table and adds newly discovered columns. It
// deliberately does not drop, rename, or alter existing columns.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if s.client == nil {
		return fmt.Errorf("bigquery sink: ensure schema before open")
	}
	table, err := defineTable(s.project, s.dataset, resource, schema)
	if err != nil {
		return fmt.Errorf("bigquery sink: schema for %q: %w", resource, err)
	}
	if err := s.execute(ctx, table.createSQL); err != nil {
		return fmt.Errorf("bigquery sink: create table %s: %w", table.qualified, err)
	}
	if err := s.execute(ctx, table.alterSQL); err != nil {
		return fmt.Errorf("bigquery sink: add columns on %s: %w", table.qualified, err)
	}
	if s.modeFor(resource) == filament.WriteReplace {
		table.replacement = stagingTableID(table.qualified, s.run, "replace")
		if err := s.execute(ctx, createTypedStageSQL(s.project, s.dataset, table, table.replacement)); err != nil {
			return fmt.Errorf("bigquery sink: create replacement table for %s: %w", table.qualified, err)
		}
		s.trackStage(table.replacement)
	}
	s.tables[resource] = table
	return nil
}

// defineTable validates a portable schema and renders additive BigQuery DDL.
// Primary keys are informational constraints and will also drive keyed writes.
func defineTable(project, dataset, table string, model rowmodel.Schema) (tableDefinition, error) {
	if strings.TrimSpace(project) == "" {
		return tableDefinition{}, fmt.Errorf("project ID is empty")
	}
	if !validDataset(dataset) {
		return tableDefinition{}, fmt.Errorf("dataset must contain 1 to 1024 letters, numbers, or underscores")
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
	additions := make([]string, len(model.Fields))
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
		columns[i] = columnDefinition{
			name: field.Name, identifier: identifier, typ: typ,
			logical: field.Logical, sql: definition,
		}
		definitions[i] = definition
		additions[i] = "ADD COLUMN IF NOT EXISTS " + identifier + " " + typ
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
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(keys, ", ")+") NOT ENFORCED")
	}

	name := qualified(project, dataset, table)
	operation := uniqueInternalColumn(model.Fields, "_filament_internal_operation")
	ordinal := uniqueInternalColumn(model.Fields, "_filament_internal_ordinal")
	return tableDefinition{
		name:      table,
		qualified: name,
		columns:   columns,
		keys:      keys,
		operation: operation,
		ordinal:   ordinal,
		createSQL: "CREATE TABLE IF NOT EXISTS " + name + " (\n\t" + strings.Join(definitions, ",\n\t") + "\n)",
		alterSQL:  "ALTER TABLE " + name + "\n\t" + strings.Join(additions, ",\n\t"),
	}, nil
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

func stagingTableID(destination string, run filament.RunID, purpose string) string {
	hash := sha256.Sum256([]byte(destination + "\x00" + string(run) + "\x00" + purpose))
	return fmt.Sprintf("_filament_%s_%x", purpose, hash[:8])
}

func createTypedStageSQL(project, dataset string, table tableDefinition, stage string) string {
	definitions := make([]string, len(table.columns))
	for i, column := range table.columns {
		definitions[i] = column.sql
	}
	return "CREATE TABLE IF NOT EXISTS " + qualified(project, dataset, stage) + " (\n\t" + strings.Join(definitions, ",\n\t") +
		"\n) OPTIONS(expiration_timestamp = TIMESTAMP_ADD(CURRENT_TIMESTAMP(), INTERVAL 7 DAY))"
}

func insertTableSQL(table tableDefinition, destination, source string) string {
	targets := make([]string, len(table.columns))
	values := make([]string, len(table.columns))
	for i, column := range table.columns {
		targets[i] = column.identifier
		values[i] = sourceValue(column, "s")
	}
	return "INSERT INTO " + destination + " (" + strings.Join(targets, ", ") + ") SELECT " +
		strings.Join(values, ", ") + " FROM " + source + " AS s"
}

func replaceTableSQL(table tableDefinition, source string) string {
	targets := make([]string, len(table.columns))
	values := make([]string, len(table.columns))
	for i, column := range table.columns {
		targets[i] = column.identifier
		values[i] = "s." + column.identifier
	}
	return "BEGIN TRANSACTION;\nTRUNCATE TABLE " + table.qualified + ";\nINSERT INTO " + table.qualified +
		" (" + strings.Join(targets, ", ") + ") SELECT " + strings.Join(values, ", ") + " FROM " + source +
		" AS s;\nCOMMIT TRANSACTION"
}

// mergeTableSQL folds repeated keys in one batch to their last operation, then
// applies the resulting inserts, updates, and deletes atomically.
func mergeTableSQL(table tableDefinition, source string) string {
	columns := make([]string, len(table.columns))
	projected := make([]string, 0, len(table.columns)+2)
	keySet := make(map[string]bool, len(table.keys))
	for _, key := range table.keys {
		keySet[key] = true
	}
	updates := make([]string, 0, len(table.columns))
	inserts := make([]string, len(table.columns))
	for i, column := range table.columns {
		columns[i] = column.identifier
		projected = append(projected, sourceValue(column, "r")+" AS "+column.identifier)
		inserts[i] = "s." + column.identifier
		if !keySet[column.identifier] {
			updates = append(updates, column.identifier+" = s."+column.identifier)
		}
	}
	projected = append(projected, "r."+table.operation.identifier, "r."+table.ordinal.identifier)

	joins := make([]string, len(table.keys))
	for i, key := range table.keys {
		joins[i] = "(t." + key + " = s." + key + " OR (t." + key + " IS NULL AND s." + key + " IS NULL))"
	}
	sourceQuery := "SELECT * FROM (SELECT " + strings.Join(projected, ", ") + " FROM " + source +
		" AS r) AS projected QUALIFY ROW_NUMBER() OVER (PARTITION BY " + strings.Join(table.keys, ", ") +
		" ORDER BY " + table.ordinal.identifier + " DESC) = 1"
	statement := "MERGE INTO " + table.qualified + " AS t USING (" + sourceQuery + ") AS s ON " +
		strings.Join(joins, " AND ") + fmt.Sprintf(" WHEN MATCHED AND s.%s = %d THEN DELETE", table.operation.identifier, rowmodel.OpDelete)
	if len(updates) > 0 {
		statement += " WHEN MATCHED THEN UPDATE SET " + strings.Join(updates, ", ")
	}
	return statement + fmt.Sprintf(" WHEN NOT MATCHED AND s.%s <> %d THEN INSERT (%s) VALUES (%s)",
		table.operation.identifier, rowmodel.OpDelete, strings.Join(columns, ", "), strings.Join(inserts, ", "))
}

func sourceValue(column columnDefinition, alias string) string {
	value := alias + "." + column.identifier
	switch column.logical {
	case rowmodel.LogicalJSON:
		return "PARSE_JSON(" + value + ")"
	case rowmodel.LogicalTimestamp:
		return "DATETIME(" + value + ", 'UTC')"
	default:
		return value
	}
}

func validDataset(dataset string) bool {
	return len(dataset) <= 1024 && datasetPattern.MatchString(dataset)
}

// columnType maps portable logical types to stable BigQuery types. Values that
// lack enough metadata for a lossless native representation land as STRING.
func columnType(field rowmodel.Field) string {
	switch field.Logical {
	case rowmodel.LogicalBool:
		return "BOOL"
	case rowmodel.LogicalInt16, rowmodel.LogicalInt32, rowmodel.LogicalInt64:
		return "INT64"
	case rowmodel.LogicalFloat32, rowmodel.LogicalFloat64:
		return "FLOAT64"
	case rowmodel.LogicalDecimal:
		return decimalType(field.Precision, field.Scale)
	case rowmodel.LogicalBytes:
		return "BYTES"
	case rowmodel.LogicalDate:
		return "DATE"
	case rowmodel.LogicalTime:
		return "TIME"
	case rowmodel.LogicalTimestamp:
		return "DATETIME"
	case rowmodel.LogicalTimestampTZ:
		return "TIMESTAMP"
	case rowmodel.LogicalJSON:
		return "JSON"
	default:
		return "STRING"
	}
}

func decimalType(precision, scale int) string {
	if precision < 1 || precision > maxDecimalPrecision || scale < 0 || scale > precision {
		return "STRING"
	}
	if scale <= maxNumericScale && precision-scale <= maxNumericIntegralDigits {
		return fmt.Sprintf("NUMERIC(%d,%d)", precision, scale)
	}
	if scale <= maxBigNumericScale && precision-scale <= maxBigNumericIntegral {
		return fmt.Sprintf("BIGNUMERIC(%d,%d)", precision, scale)
	}
	return "STRING"
}
