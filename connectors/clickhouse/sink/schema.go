package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

const (
	mergeTreeEngine          = "MergeTree"
	replacingMergeTreeEngine = "ReplacingMergeTree"
	sharedEnginePrefix       = "Shared"
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

// EnsureSchema creates or evolves one resource table before records arrive.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	if s.conn == nil {
		return fmt.Errorf("clickhouse sink: ensure schema before open")
	}
	mode := s.modeFor(resource)
	upsert := mode == filament.WriteUpsert
	version := filament.VersionPolicy{}
	if policy, ok := s.policies[resource]; ok {
		version = policy.Version
	} else if policy, ok := s.policies[""]; ok {
		version = policy.Version
	}
	if upsert && version.Strategy == "" {
		version.Strategy = filament.VersionInsertOrder
	}
	stage := ""
	writeTable := resource
	if mode == filament.WriteReplace {
		stage = stageTableName(s.run, resource)
		writeTable = stage
		if err := s.conn.Exec(ctx, "DROP TABLE IF EXISTS "+qualified(s.database, stage)); err != nil {
			return fmt.Errorf("clickhouse sink: drop stale stage for %q: %w", resource, err)
		}
	}

	ddl, idents, err := createTableDDL(s.database, writeTable, schema, upsert, version)
	if err != nil {
		return fmt.Errorf("clickhouse sink: schema for %q: %w", resource, err)
	}
	if err := s.conn.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("clickhouse sink: create table for %q: %w", resource, err)
	}
	if stage == "" {
		if err := s.validateTable(ctx, resource, schema, upsert, version); err != nil {
			return err
		}
		defs, _, err := schemaColumns(schema, upsert, version)
		if err != nil {
			return err
		}
		for _, def := range defs {
			if err := s.conn.Exec(ctx, "ALTER TABLE "+qualified(s.database, resource)+" ADD COLUMN IF NOT EXISTS "+def); err != nil {
				return fmt.Errorf("clickhouse sink: add column on %q: %w", resource, err)
			}
		}
	}

	writeTo := qualified(s.database, writeTable)
	s.tables[resource] = &table{
		name: resource, qualified: qualified(s.database, resource), writeTo: writeTo,
		stage: stage, insertSQL: "INSERT INTO " + writeTo + " (" + strings.Join(idents, ", ") + ")",
	}
	return nil
}

func (s *Sink) validateTable(ctx context.Context, resource string, schema rowmodel.Schema, upsert bool, version filament.VersionPolicy) error {
	var engine, engineFull string
	if err := s.conn.QueryRow(ctx,
		"SELECT engine, engine_full FROM system.tables WHERE database = ? AND name = ?", s.database, resource).Scan(&engine, &engineFull); err != nil {
		return fmt.Errorf("clickhouse sink: inspect table %q: %w", resource, err)
	}
	want := mergeTreeEngine
	if upsert {
		want = replacingMergeTreeEngine
	}
	if !matchesTableEngine(engine, want) {
		return fmt.Errorf("clickhouse sink: table %q uses engine %s, need %s for write policy %q", resource, engine, want, s.modeFor(resource))
	}
	if !upsert {
		return nil
	}
	versionField, err := versionField(schema, version)
	if err != nil {
		return fmt.Errorf("clickhouse sink: table %q version policy: %w", resource, err)
	}
	if !matchesReplacingVersion(engineFull, versionField) {
		wantVersion := "insertion order"
		if versionField != "" {
			wantVersion = versionField
		}
		return fmt.Errorf("clickhouse sink: table %q must use %s as its ReplacingMergeTree version", resource, wantVersion)
	}
	rows, err := s.conn.Query(ctx,
		"SELECT name FROM system.columns WHERE database = ? AND table = ? AND is_in_sorting_key ORDER BY name", s.database, resource)
	if err != nil {
		return fmt.Errorf("clickhouse sink: inspect sorting key for %q: %w", resource, err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return fmt.Errorf("clickhouse sink: scan sorting key for %q: %w", resource, err)
		}
		got = append(got, name)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("clickhouse sink: sorting key for %q: %w", resource, err)
	}
	wantKeys := append([]string(nil), schema.PrimaryKey...)
	sort.Strings(wantKeys)
	if strings.Join(got, "\x00") != strings.Join(wantKeys, "\x00") {
		return fmt.Errorf("clickhouse sink: table %q sorting key %v must exactly match source primary key %v", resource, got, schema.PrimaryKey)
	}
	return nil
}

func matchesReplacingVersion(engineFull, field string) bool {
	compact := strings.NewReplacer(" ", "", "`", "").Replace(engineFull)
	compact = strings.TrimPrefix(compact, sharedEnginePrefix)
	rest, ok := strings.CutPrefix(compact, replacingMergeTreeEngine)
	if !ok {
		return false
	}
	if field != "" {
		return strings.HasPrefix(rest, "("+field+")")
	}
	// ClickHouse may render the parameterless engine with or without (). Reject
	// any non-empty argument so cursor-versioned tables cannot be mistaken for
	// insertion-order tables.
	return !strings.HasPrefix(rest, "(") || strings.HasPrefix(rest, "()")
}

func matchesTableEngine(got, want string) bool {
	return got == want || got == sharedEnginePrefix+want
}
