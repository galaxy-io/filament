package motherduck

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/rowmodel"
)

type table struct {
	name      string
	qualified string
	writeName string
	writeTo   string
	stage     string
	mode      filament.WriteMode
	mergeSQL  string
}

func quoteIdent(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func qualifySchema(database, schema string) string {
	if database == "" {
		return quoteIdent(schema)
	}
	return quoteIdent(database) + "." + quoteIdent(schema)
}

func qualified(database, schema, table string) string {
	return qualifySchema(database, schema) + "." + quoteIdent(table)
}

func schemaColumns(schema rowmodel.Schema) ([]string, map[string]struct{}, error) {
	if len(schema.Fields) == 0 {
		return nil, nil, fmt.Errorf("schema has no fields")
	}
	definitions := make([]string, len(schema.Fields))
	fields := make(map[string]struct{}, len(schema.Fields))
	for i, field := range schema.Fields {
		if field.Name == "" {
			return nil, nil, fmt.Errorf("schema contains an empty field name")
		}
		if _, exists := fields[field.Name]; exists {
			return nil, nil, fmt.Errorf("schema contains duplicate field %q", field.Name)
		}
		fields[field.Name] = struct{}{}
		definitions[i] = quoteIdent(field.Name) + " " + columnType(field)
		if !field.Nullable {
			definitions[i] += " NOT NULL"
		}
	}
	return definitions, fields, nil
}

func createTableDDL(name string, schema rowmodel.Schema, keys []string) (string, error) {
	definitions, fields, err := schemaColumns(schema)
	if err != nil {
		return "", err
	}
	if len(keys) > 0 {
		keyIdentifiers := make([]string, len(keys))
		seen := make(map[string]struct{}, len(keys))
		for i, key := range keys {
			if _, exists := fields[key]; !exists {
				return "", fmt.Errorf("primary-key field %q is absent from schema", key)
			}
			if _, exists := seen[key]; exists {
				return "", fmt.Errorf("primary key contains duplicate field %q", key)
			}
			seen[key] = struct{}{}
			keyIdentifiers[i] = quoteIdent(key)
		}
		definitions = append(definitions, "PRIMARY KEY ("+strings.Join(keyIdentifiers, ", ")+")")
	}
	return "CREATE TABLE IF NOT EXISTS " + name + " (\n\t" + strings.Join(definitions, ",\n\t") + "\n)", nil
}

// addColumnDDLs deliberately omits NOT NULL. DuckDB rejects ADD COLUMN with a
// constraint; new tables still receive the source nullability in createTableDDL.
func addColumnDDLs(name string, schema rowmodel.Schema) ([]string, error) {
	_, _, err := schemaColumns(schema)
	if err != nil {
		return nil, err
	}
	statements := make([]string, len(schema.Fields))
	for i, field := range schema.Fields {
		statements[i] = "ALTER TABLE " + name + " ADD COLUMN IF NOT EXISTS " + quoteIdent(field.Name) + " " + columnType(field)
	}
	return statements, nil
}

// upsertSQL merges the stage into the target by primary key. Columns are named
// on both sides because the live table may carry columns the stage lacks or
// hold them in a different order.
func upsertSQL(target, stage string, columns, keys []string) string {
	keySet := make(map[string]struct{}, len(keys))
	keyIdentifiers := make([]string, len(keys))
	for i, key := range keys {
		keySet[key] = struct{}{}
		keyIdentifiers[i] = quoteIdent(key)
	}
	identifiers := make([]string, len(columns))
	sets := make([]string, 0, len(columns)-len(keys))
	for i, column := range columns {
		identifiers[i] = quoteIdent(column)
		if _, key := keySet[column]; key {
			continue
		}
		sets = append(sets, identifiers[i]+" = excluded."+identifiers[i])
	}
	list := strings.Join(identifiers, ", ")
	statement := "INSERT INTO " + target + " (" + list + ") SELECT " + list + " FROM " + stage +
		" ON CONFLICT (" + strings.Join(keyIdentifiers, ", ") + ") DO "
	if len(sets) == 0 {
		return statement + "NOTHING"
	}
	return statement + "UPDATE SET " + strings.Join(sets, ", ")
}

// EnsureSchema creates one destination or run-scoped staging table for resource.
func (s *Sink) EnsureSchema(ctx context.Context, resource string, schema rowmodel.Schema) error {
	s.schemaMu.Lock()
	defer s.schemaMu.Unlock()
	if s.db == nil {
		return fmt.Errorf("%s sink: ensure schema before open", s.Name())
	}
	if resource == "" {
		return fmt.Errorf("%s sink: resource name is empty", s.Name())
	}
	policy := s.policyFor(resource)
	conn, err := s.take(ctx)
	if err != nil {
		return err
	}
	defer s.put(conn)

	var tbl *table
	switch policy.Capability.Mode {
	case filament.WriteAppend:
		tbl, err = s.ensureAppendTable(ctx, conn, resource, schema)
	case filament.WriteReplace:
		tbl, err = s.ensureReplaceTable(ctx, conn, resource, schema)
	case filament.WriteUpsert:
		tbl, err = s.ensureUpsertTable(ctx, conn, resource, schema, policy.Keys)
	default:
		return fmt.Errorf("%s sink: write policy %q is not implemented", s.Name(), policy.Capability.Mode)
	}
	if err != nil {
		return err
	}
	s.tablesMu.Lock()
	s.tables[resource] = tbl
	s.tablesMu.Unlock()
	return nil
}

func (s *Sink) ensureAppendTable(ctx context.Context, conn *pooledConnection, resource string, schema rowmodel.Schema) (*table, error) {
	target := qualified(s.database, s.schema, resource)
	if err := s.ensureLiveTable(ctx, conn, resource, target, schema, nil); err != nil {
		return nil, err
	}
	return &table{name: resource, qualified: target, writeName: resource, writeTo: target, mode: filament.WriteAppend}, nil
}

func (s *Sink) ensureReplaceTable(ctx context.Context, conn *pooledConnection, resource string, schema rowmodel.Schema) (*table, error) {
	target := qualified(s.database, s.schema, resource)
	stage, writeTo, err := s.createStageTable(ctx, conn, resource, schema)
	if err != nil {
		return nil, err
	}
	return &table{
		name: resource, qualified: target, writeName: stage, writeTo: writeTo,
		stage: stage, mode: filament.WriteReplace,
	}, nil
}

func (s *Sink) ensureUpsertTable(
	ctx context.Context,
	conn *pooledConnection,
	resource string,
	schema rowmodel.Schema,
	policyKeys []string,
) (*table, error) {
	keys := append([]string(nil), policyKeys...)
	if len(keys) == 0 {
		return nil, fmt.Errorf("%s sink: upsert requires a primary key for resource %q", s.Name(), resource)
	}
	target := qualified(s.database, s.schema, resource)
	if err := s.ensureLiveTable(ctx, conn, resource, target, schema, keys); err != nil {
		return nil, err
	}
	if err := s.verifyPrimaryKey(ctx, conn, resource, keys); err != nil {
		return nil, err
	}
	stage, writeTo, err := s.createStageTable(ctx, conn, resource, schema)
	if err != nil {
		return nil, err
	}
	return &table{
		name: resource, qualified: target, writeName: stage, writeTo: writeTo,
		stage: stage, mode: filament.WriteUpsert,
		mergeSQL: upsertSQL(target, writeTo, unquoteColumns(schema), keys),
	}, nil
}

func (s *Sink) ensureLiveTable(
	ctx context.Context,
	conn *pooledConnection,
	resource, target string,
	schema rowmodel.Schema,
	keys []string,
) error {
	ddl, err := createTableDDL(target, schema, keys)
	if err != nil {
		return fmt.Errorf("%s sink: schema for %q: %w", s.Name(), resource, err)
	}
	if err := conn.exec(ctx, ddl); err != nil {
		return fmt.Errorf("%s sink: create table for %q: %w", s.Name(), resource, err)
	}
	if err := addColumns(ctx, conn, target, schema); err != nil {
		return fmt.Errorf("%s sink: add columns for %q: %w", s.Name(), resource, err)
	}
	return nil
}

func (s *Sink) createStageTable(
	ctx context.Context,
	conn *pooledConnection,
	resource string,
	schema rowmodel.Schema,
) (string, string, error) {
	stage := stageTableName(resource, s.run)
	writeTo := qualified(s.database, s.schema, stage)
	ddl, err := createTableDDL(writeTo, schema, nil)
	if err != nil {
		return "", "", fmt.Errorf("%s sink: stage schema for %q: %w", s.Name(), resource, err)
	}
	if err := conn.exec(ctx, "DROP TABLE IF EXISTS "+writeTo); err != nil {
		return "", "", fmt.Errorf("%s sink: drop stale stage for %q: %w", s.Name(), resource, err)
	}
	if err := conn.exec(ctx, ddl); err != nil {
		return "", "", fmt.Errorf("%s sink: create stage for %q: %w", s.Name(), resource, err)
	}
	return stage, writeTo, nil
}

// verifyPrimaryKey rejects an existing destination whose primary key differs
// from the policy keys, such as a table an earlier append run created without
// one. The merge would fail at commit otherwise, so the run is refused up front
// instead of migrated.
func (s *Sink) verifyPrimaryKey(ctx context.Context, conn *pooledConnection, resource string, keys []string) error {
	query := "SELECT constraint_column_names FROM duckdb_constraints() WHERE constraint_type = 'PRIMARY KEY'" +
		" AND schema_name = " + quoteLiteral(s.schema) + " AND table_name = " + quoteLiteral(resource)
	if s.database != "" {
		query += " AND database_name = " + quoteLiteral(s.database)
	}
	rows, err := conn.conn.(driver.QueryerContext).QueryContext(ctx, query, nil)
	if err != nil {
		return fmt.Errorf("%s sink: inspect primary key for %q: %w", s.Name(), resource, err)
	}
	defer func() { _ = rows.Close() }()
	var got []string
	values := make([]driver.Value, 1)
	switch err := rows.Next(values); {
	case errors.Is(err, io.EOF):
		return fmt.Errorf("%s sink: table %q has no primary key; upsert needs primary key %v", s.Name(), resource, keys)
	case err != nil:
		return fmt.Errorf("%s sink: inspect primary key for %q: %w", s.Name(), resource, err)
	}
	names, _ := values[0].([]any)
	for _, name := range names {
		got = append(got, fmt.Sprint(name))
	}
	want := append([]string(nil), keys...)
	sort.Strings(got)
	sort.Strings(want)
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		return fmt.Errorf("%s sink: table %q primary key %v must match the write policy keys %v", s.Name(), resource, got, keys)
	}
	return nil
}

func quoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func addColumns(ctx context.Context, conn *pooledConnection, table string, schema rowmodel.Schema) error {
	statements, err := addColumnDDLs(table, schema)
	if err != nil {
		return err
	}
	for _, statement := range statements {
		if err := conn.exec(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func unquoteColumns(schema rowmodel.Schema) []string {
	columns := make([]string, len(schema.Fields))
	for i, field := range schema.Fields {
		columns[i] = field.Name
	}
	return columns
}
