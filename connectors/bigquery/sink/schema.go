package bigquery

import (
	"context"
	"fmt"
	"regexp"
	"strings"

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
	qualified string
	createSQL string
	alterSQL  string
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
	return tableDefinition{
		qualified: name,
		createSQL: "CREATE TABLE IF NOT EXISTS " + name + " (\n\t" + strings.Join(definitions, ",\n\t") + "\n)",
		alterSQL:  "ALTER TABLE " + name + "\n\t" + strings.Join(additions, ",\n\t"),
	}, nil
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
