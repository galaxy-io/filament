// Package snowflake implements the Snowflake sink.
package snowflake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

const sinkName = "snowflake"

var errWritesUnavailable = errors.New("snowflake sink: writes are not implemented")

// Sink owns the Snowflake session and materializes typed destination tables.
// Row loading is added in a subsequent implementation tranche.
type Sink struct {
	db       *sql.DB
	database string
	schema   string
}

// New returns an unconfigured Snowflake sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Spec describes the sink's connection configuration. It intentionally
// advertises no write policies until the snapshot implementation is available.
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
			Schematized: true,
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
	db := resolved.open()
	if _, err := db.ExecContext(ctx, ddl); err != nil {
		_ = db.Close()
		return fmt.Errorf("snowflake sink: create schema %s: %w", qualified(database, schema), err)
	}
	s.db = db
	s.database = database
	s.schema = schema
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
	for _, column := range table.columns {
		//nolint:gosec // Identifiers are quoted and type spellings come only from columnType.
		ddl := "ALTER TABLE " + table.qualified +
			" ADD COLUMN IF NOT EXISTS " + column.sql
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("snowflake sink: add column %s on %s: %w", column.identifier, table.qualified, err)
		}
	}
	return nil
}

// Apply is unavailable until the Snowflake write implementation is added.
func (*Sink) Apply(context.Context, *arrowbatch.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, errWritesUnavailable
}

// Commit closes the session. Schema DDL is committed by Snowflake when issued.
func (s *Sink) Commit(context.Context) error {
	s.release()
	return nil
}

// Abort closes the session. Additive schema changes are intentionally retained.
func (s *Sink) Abort(context.Context) error {
	s.release()
	return nil
}

func (s *Sink) release() {
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
}
