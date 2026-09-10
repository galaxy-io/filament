package snowflake

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament/rowmodel"
)

const maxNumberPrecision = 38

type columnDefinition struct {
	identifier string
	typ        string
	sql        string
}

type tableDefinition struct {
	qualified string
	columns   []columnDefinition
	createSQL string
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

// defineTable validates a portable schema and renders the additive DDL used by
// EnsureSchema. Primary keys are metadata in standard Snowflake tables; future
// merge support will also use the source schema's key list directly.
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
		columns[i] = columnDefinition{identifier: identifier, typ: typ, sql: definition}
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
	return tableDefinition{
		qualified: name,
		columns:   columns,
		createSQL: "CREATE TABLE IF NOT EXISTS " + name + " (\n\t" + strings.Join(definitions, ",\n\t") + "\n)",
	}, nil
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
