// Package snowflake implements the Snowflake sink.
package snowflake

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

const sinkName = "snowflake"

// Sink owns the Snowflake sessions and the typed tables prepared for a run.
type Sink struct {
	db       *sql.DB
	run      filament.RunID
	database string
	schema   string
	policies map[string]filament.WritePolicy

	// tables is populated during the engine's sequential EnsureSchema pass and
	// only read once concurrent Apply calls begin, so it needs no lock.
	tables map[string]tableDefinition
}

// New returns an unconfigured Snowflake sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Spec describes the sink's connection configuration and write capabilities.
func (*Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Snowflake",
		Description:  "Cloud-native data platform for scalable analytics, elastic compute, and secure data sharing.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-snowflake-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-snowflake-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(connectionFields(), filament.ConfigField{
			Name: "schema", Type: filament.FieldString, Default: defaultSchema,
			Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name.",
		})},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			EncodedIntegrity:   true,
			PreferredBatchRows: 100_000,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalAppend,
				filament.IngestionIncrementalUpsert,
				filament.IngestionIncrementalDelete,
				filament.IngestionCDCMerge,
				filament.IngestionCDCAppend,
			),
		},
	}
}

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks connection syntax and credentials without accessing the network.
func (*Sink) Validate(cfg filament.Config) error {
	if _, err := resolveConnection(cfg); err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	return nil
}

// TestConnection authenticates with Snowflake using a short-lived database handle.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := resolveConnection(cfg)
	if err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	db := resolved.open()
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("snowflake sink: ping: %w", err)
	}
	return nil
}

// Open establishes the run's Snowflake session and ensures its destination
// schema exists. Resource tables are created later by EnsureSchema.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.db != nil {
		return fmt.Errorf("snowflake sink: already open")
	}
	resolved, err := resolveConnection(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	database := resolved.driverConfig.Database
	schema := resolved.driverConfig.Schema
	ddl, err := createSchemaDDL(database, schema)
	if err != nil {
		return fmt.Errorf("snowflake sink: schema: %w", err)
	}
	// Every destination is fully qualified and writes stage through @~, so
	// neither a current database nor schema is needed. Leaving both unset also
	// preserves case-sensitive quoted names such as a normalized "local_pg";
	// connection session parameters otherwise resolve them as unquoted names.
	resolved.driverConfig.Database = ""
	resolved.driverConfig.Schema = ""
	db := resolved.open()
	_, err = db.ExecContext(ctx, ddl)
	if err != nil {
		_ = db.Close()
		return fmt.Errorf("snowflake sink: create schema %s: %w", qualified(database, schema), err)
	}

	s.db = db
	s.run = run.Run
	s.database = database
	s.schema = schema
	s.policies = run.WritePolicies
	s.tables = make(map[string]tableDefinition)
	return nil
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

// Apply validates a batch and routes append/replace directly into the target;
// key-based modes fold through a session-local table first.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if s.db == nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: write before open")
	}
	if batch == nil || batch.Rows() == nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: write requires a record batch")
	}
	table, ok := s.tables[batch.Resource]
	if !ok {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: no schema ensured for resource %q", batch.Resource)
	}
	mode := opts.Policy.Capability.Mode
	if expected := s.modeFor(batch.Resource); expected != "" && mode != expected {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: apply policy %q does not match resource policy %q", mode, expected)
	}
	policy := opts.Policy
	if mode == filament.WriteReplace {
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
	}
	if err := policy.ValidateBatch(batch.Resource, batch); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: %w", err)
	}
	switch mode {
	case filament.WriteAppend, filament.WriteReplace:
		return s.write(ctx, table, batch)
	case filament.WriteUpsert, filament.WriteDelete, filament.WriteMerge:
		if table.mergeSQL == "" {
			return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: write policy %q requires a primary key for resource %q", mode, batch.Resource)
		}
		return s.writeFold(ctx, table, batch)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("snowflake sink: write policy %q is not implemented", mode)
	}
}

// Commit closes the session. Schema DDL is committed by Snowflake when issued.
func (s *Sink) Commit(context.Context) error {
	s.release()
	return nil
}

// Abort removes partial full-replacement data and closes the session. Other
// modes are replay-safe or must retain pre-existing destination rows.
func (s *Sink) Abort(ctx context.Context) error {
	defer s.release()
	if s.db == nil {
		return nil
	}
	cleanup := context.WithoutCancel(ctx)
	for _, table := range s.tables {
		if table.replace {
			_, _ = s.db.ExecContext(cleanup, "TRUNCATE TABLE "+table.qualified) //nolint:gosec // identifier is quoted
		}
	}
	return nil
}

func (s *Sink) modeFor(resource string) filament.WriteMode {
	policy, ok := s.policies[resource]
	if !ok {
		policy = s.policies[""]
	}
	return policy.Capability.Mode
}

func (s *Sink) release() {
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	s.policies = nil
	s.tables = nil
}
