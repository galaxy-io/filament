package clickhouse

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func qualified(database, table string) string {
	return quoteIdent(database) + "." + quoteIdent(table)
}

func orderBy(schema rowmodel.Schema) string {
	if len(schema.PrimaryKey) == 0 {
		return "tuple()"
	}
	ids := make([]string, len(schema.PrimaryKey))
	for i, name := range schema.PrimaryKey {
		ids[i] = quoteIdent(name)
	}
	return "(" + strings.Join(ids, ", ") + ")"
}

func schemaColumns(schema rowmodel.Schema, upsert bool, version filament.VersionPolicy) ([]string, []string, error) {
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
		defs = append(defs, id+" "+columnType(field))
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

func versionField(schema rowmodel.Schema, version filament.VersionPolicy) (string, error) {
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
		case rowmodel.LogicalTimestamp, rowmodel.LogicalTimestampTZ:
			return field.Name, nil
		default:
			return "", fmt.Errorf("cursor version field %q has unsupported ClickHouse type %q", field.Name, columnType(field))
		}
	}
	return "", fmt.Errorf("cursor version field %q is absent from schema", version.Field)
}

func createTableDDL(database, table string, schema rowmodel.Schema, upsert bool, version filament.VersionPolicy) (string, []string, error) {
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
