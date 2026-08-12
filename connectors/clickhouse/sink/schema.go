package clickhouse

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/galaxy-io/filament"
)

var decimalType = regexp.MustCompile(`(?i)^(?:decimal|numeric)\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)$`)

// clickhouseColumnType maps Filament's portable schema onto conservative
// ClickHouse types. JSON, arrays, and unknown source-native types land as their
// exact JSON text in String until the portable schema carries enough nested type
// information to create a lossless ClickHouse JSON/Array declaration.
func clickhouseColumnType(f filament.SchemaField) string {
	var typ string
	switch f.Logical {
	case filament.LogicalBool:
		typ = "Bool"
	case filament.LogicalInt16:
		typ = "Int16"
	case filament.LogicalInt32:
		typ = "Int32"
	case filament.LogicalInt64:
		typ = "Int64"
	case filament.LogicalFloat32:
		typ = "Float32"
	case filament.LogicalFloat64:
		typ = "Float64"
	case filament.LogicalDecimal:
		typ = decimalFromNative(f.Native)
	case filament.LogicalString, filament.LogicalBytes, filament.LogicalTime,
		filament.LogicalJSON, filament.LogicalArray, filament.LogicalUnknown:
		typ = "String"
	case filament.LogicalDate:
		typ = "Date32"
	case filament.LogicalTimestamp:
		typ = "DateTime64(6)"
	case filament.LogicalTimestampTZ:
		typ = "DateTime64(6, 'UTC')"
	case filament.LogicalUUID:
		typ = "UUID"
	default:
		typ = "String"
	}
	if f.Nullable {
		return "Nullable(" + typ + ")"
	}
	return typ
}

func decimalFromNative(native string) string {
	m := decimalType.FindStringSubmatch(strings.TrimSpace(native))
	if len(m) != 3 {
		return "Decimal(38, 9)"
	}
	precision, _ := strconv.Atoi(m[1])
	scale, _ := strconv.Atoi(m[2])
	// ClickHouse Decimal supports precision 1..76 and scale <= precision.
	if precision < 1 || precision > 76 || scale < 0 || scale > precision {
		return "Decimal(38, 9)"
	}
	return fmt.Sprintf("Decimal(%d, %d)", precision, scale)
}

func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func qualified(database, table string) string {
	return quoteIdent(database) + "." + quoteIdent(table)
}

func orderBy(schema filament.RecordSchema) string {
	if len(schema.PrimaryKey) == 0 {
		return "tuple()"
	}
	ids := make([]string, len(schema.PrimaryKey))
	for i, name := range schema.PrimaryKey {
		ids[i] = quoteIdent(name)
	}
	return "(" + strings.Join(ids, ", ") + ")"
}

func schemaColumns(schema filament.RecordSchema, upsert bool, version filament.VersionPolicy) ([]string, []string, error) {
	if len(schema.Fields) == 0 {
		return nil, nil, fmt.Errorf("schema has no fields")
	}
	fieldSet := make(map[string]bool, len(schema.Fields))
	defs := make([]string, 0, len(schema.Fields)+1)
	idents := make([]string, 0, len(schema.Fields)+1)
	for _, field := range schema.Fields {
		if field.Name == "" {
			return nil, nil, fmt.Errorf("schema contains an empty field name")
		}
		if fieldSet[field.Name] {
			return nil, nil, fmt.Errorf("schema contains duplicate field %q", field.Name)
		}
		fieldSet[field.Name] = true
		id := quoteIdent(field.Name)
		defs = append(defs, id+" "+clickhouseColumnType(field))
		idents = append(idents, id)
	}
	for _, key := range schema.PrimaryKey {
		if !fieldSet[key] {
			return nil, nil, fmt.Errorf("primary-key field %q is absent from schema", key)
		}
	}
	if upsert {
		if len(schema.PrimaryKey) == 0 {
			return nil, nil, fmt.Errorf("upsert requires a primary key")
		}
		if _, err := versionField(schema, version); err != nil {
			return nil, nil, err
		}
	}
	return defs, idents, nil
}

func versionField(schema filament.RecordSchema, version filament.VersionPolicy) (string, error) {
	if version.Strategy == "" || version.Strategy == filament.VersionInsertOrder {
		return "", nil
	}
	if version.Strategy != filament.VersionCursor {
		return "", fmt.Errorf("unsupported version strategy %q", version.Strategy)
	}
	for _, field := range schema.Fields {
		if field.Name != version.Field {
			continue
		}
		if field.Nullable {
			return "", fmt.Errorf("cursor version field %q must be NOT NULL", field.Name)
		}
		switch field.Logical {
		case filament.LogicalTimestamp, filament.LogicalTimestampTZ:
			return field.Name, nil
		default:
			return "", fmt.Errorf("cursor version field %q has unsupported ClickHouse type %q", field.Name, clickhouseColumnType(field))
		}
	}
	return "", fmt.Errorf("cursor version field %q is absent from schema", version.Field)
}

func createTableDDL(database, table string, schema filament.RecordSchema, upsert bool, version filament.VersionPolicy) (string, []string, error) {
	defs, idents, err := schemaColumns(schema, upsert, version)
	if err != nil {
		return "", nil, err
	}
	engine := mergeTreeEngine + "()"
	if upsert {
		field, err := versionField(schema, version)
		if err != nil {
			return "", nil, err
		}
		engine = replacingMergeTreeEngine + "()"
		if field != "" {
			engine = replacingMergeTreeEngine + "(" + quoteIdent(field) + ")"
		}
	}
	ddl := "CREATE TABLE IF NOT EXISTS " + qualified(database, table) + " (\n\t" +
		strings.Join(defs, ",\n\t") + "\n) ENGINE = " + engine + " ORDER BY " + orderBy(schema)
	return ddl, idents, nil
}
