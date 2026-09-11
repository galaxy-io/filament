// Package snowflake implements the Snowflake sink.
package snowflake

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/galaxy-io/filament"
	snowflakeconnection "github.com/galaxy-io/filament/connectors/snowflake/internal/connection"
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

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks connection syntax and credentials without accessing the network.
func (*Sink) Validate(cfg filament.Config) error {
	if _, err := snowflakeconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	return nil
}

// TestConnection authenticates with Snowflake using a short-lived database handle.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := snowflakeconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	if err := snowflakeconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("snowflake sink: %w", err)
	}
	return nil
}

// Open establishes the run's Snowflake session and ensures its destination
// schema exists. Resource tables are created later by EnsureSchema.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.db != nil {
		return fmt.Errorf("snowflake sink: already open")
	}
	resolved, err := snowflakeconnection.Resolve(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	database := resolved.DriverConfig.Database
	schema := resolved.DriverConfig.Schema
	ddl, err := createSchemaDDL(database, schema)
	if err != nil {
		return fmt.Errorf("snowflake sink: schema: %w", err)
	}
	// Every destination is fully qualified and writes stage through @~, so
	// neither a current database nor schema is needed. Leaving both unset also
	// preserves case-sensitive quoted names such as a normalized "local_pg";
	// connection session parameters otherwise resolve them as unquoted names.
	resolved.DriverConfig.Database = ""
	resolved.DriverConfig.Schema = ""
	db := snowflakeconnection.Open(ctx, resolved)
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
