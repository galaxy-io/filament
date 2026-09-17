// Package redshift implements the Amazon Redshift sink.
package redshift

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	redshiftconnection "github.com/galaxy-io/filament/connectors/redshift/internal/connection"
)

const sinkName = "redshift"

// Sink owns the Redshift pool, S3 staging client, and typed tables prepared for
// a run. Both provisioned clusters and Serverless workgroups expose the same
// PostgreSQL-compatible endpoint consumed here.
type Sink struct {
	pool     *pgxpool.Pool
	staging  *stagingStore
	run      filament.RunID
	database string
	schema   string
	iamRole  string
	ledger   string
	policies map[string]filament.WritePolicy
	tables   map[string]tableDefinition
}

// New returns an unconfigured Redshift sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks database and staging configuration without network access.
func (*Sink) Validate(cfg filament.Config) error {
	if _, err := redshiftconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("redshift sink: connection config: %w", err)
	}
	return nil
}

// TestConnection verifies the Redshift endpoint. S3 access is tested by Open
// in the worker because the worker's IAM role commonly differs from the API's.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := redshiftconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("redshift sink: connection config: %w", err)
	}
	if err := redshiftconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("redshift sink: %w", err)
	}
	return nil
}

// Open connects to Redshift, verifies the caller-provided S3 staging bucket,
// and creates the destination schema plus the internal replay ledger.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.pool != nil {
		return fmt.Errorf("redshift sink: already open")
	}
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := redshiftconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("redshift sink: connection config: %w", err)
	}
	schema := strings.TrimSpace(cfg.String("schema"))
	if schema == "" {
		schema = strings.TrimSpace(cfg.String("schema_name"))
	}
	if schema == "" {
		schema = redshiftconnection.DefaultSchema
	}
	if err := validateIdentifier("schema", schema); err != nil {
		return fmt.Errorf("redshift sink: schema: %w", err)
	}
	prefix := strings.Trim(cfg.String("staging_prefix"), "/")
	if prefix == "" {
		prefix = redshiftconnection.DefaultStagingPrefix
	}
	if strings.ContainsAny(prefix, "\r\n") {
		return fmt.Errorf("redshift sink: staging_prefix must not contain line breaks")
	}
	prefix, err = pipelineStagingPrefix(prefix, run.PipelineID)
	if err != nil {
		return fmt.Errorf("redshift sink: S3 staging: %w", err)
	}

	stage, err := newStagingStore(ctx, resolved.StagingBucket, resolved.StagingBucketRegion, prefix)
	if err != nil {
		return fmt.Errorf("redshift sink: S3 staging: %w", err)
	}
	if err := stage.test(ctx); err != nil {
		return fmt.Errorf("redshift sink: S3 staging: %w", err)
	}

	poolCfg := resolved.DriverConfig.Copy()
	if n := run.Options.SnapshotParallelism; n > 0 && n <= math.MaxInt32 {
		poolCfg.MaxConns = max(poolCfg.MaxConns, int32(n))
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("redshift sink: open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return fmt.Errorf("redshift sink: ping: %w", err)
	}
	s.pool = pool
	s.staging = stage
	s.run = run.Run
	s.database = poolCfg.ConnConfig.Database
	s.schema = schema
	s.iamRole = resolved.AssociatedIAMRole
	s.ledger = qualified(schema, "_filament_load_batches")
	s.policies = run.WritePolicies
	s.tables = make(map[string]tableDefinition)

	if _, err := pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+quoteIdent(schema)); err != nil {
		s.release()
		return fmt.Errorf("redshift sink: create schema %s: %w", quoteIdent(schema), err)
	}
	if _, err := pool.Exec(ctx, createLedgerSQL(s.ledger)); err != nil {
		s.release()
		return fmt.Errorf("redshift sink: create replay ledger: %w", err)
	}
	return nil
}

// Commit atomically promotes each full-replacement resource, then closes the
// clients. Other modes are durable when Apply commits their transaction.
func (s *Sink) Commit(ctx context.Context) error {
	if s.pool == nil {
		return fmt.Errorf("redshift sink: commit before open")
	}
	resources := make([]string, 0, len(s.tables))
	for resource := range s.tables {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	for _, resource := range resources {
		table := s.tables[resource]
		if table.replacement == "" {
			continue
		}
		if err := s.promoteReplacement(ctx, table); err != nil {
			return fmt.Errorf("redshift sink: promote replacement for %q: %w", resource, err)
		}
	}
	s.release()
	return nil
}

// Abort removes run-scoped replacement tables and their batch-ledger entries.
// Append and keyed Apply transactions are already durable and replay-safe.
func (s *Sink) Abort(ctx context.Context) error {
	if s.pool == nil {
		return nil
	}
	defer s.release()
	cleanup := context.WithoutCancel(ctx)
	for _, table := range s.tables {
		if table.replacement == "" {
			continue
		}
		_, _ = s.pool.Exec(cleanup, "DROP TABLE IF EXISTS "+table.replacement)
		_, _ = s.pool.Exec(cleanup, "DELETE FROM "+s.ledger+" WHERE run_id = $1 AND destination = $2", string(s.run), table.qualified)
	}
	return nil
}

func (s *Sink) promoteReplacement(ctx context.Context, table tableDefinition) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := tx.Exec(ctx, "LOCK TABLE "+table.qualified); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM "+table.qualified); err != nil {
		return err
	}
	idents := columnIdentifiers(table.columns)
	statement := "INSERT INTO " + table.qualified + " (" + strings.Join(idents, ", ") + ") SELECT " +
		strings.Join(idents, ", ") + " FROM " + table.replacement
	if _, err := tx.Exec(ctx, statement); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DROP TABLE "+table.replacement); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Sink) modeFor(resource string) filament.WriteMode {
	policy, ok := s.policies[resource]
	if !ok {
		policy = s.policies[""]
	}
	return policy.Capability.Mode
}

func (s *Sink) release() {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
	s.staging = nil
	s.run = ""
	s.database = ""
	s.schema = ""
	s.iamRole = ""
	s.ledger = ""
	s.policies = nil
	s.tables = nil
}

func createLedgerSQL(name string) string {
	return "CREATE TABLE IF NOT EXISTS " + name + ` (
	token VARCHAR(64) NOT NULL,
	payload_hash VARCHAR(64) NOT NULL,
	run_id VARCHAR(512) NOT NULL,
	destination VARCHAR(512) NOT NULL,
	part INTEGER NOT NULL,
	seq DECIMAL(20,0) NOT NULL,
	rows BIGINT NOT NULL,
	applied_at TIMESTAMPTZ NOT NULL DEFAULT GETDATE(),
	PRIMARY KEY (token)
) DISTSTYLE AUTO SORTKEY AUTO ENCODE AUTO`
}

func columnIdentifiers(columns []columnDefinition) []string {
	idents := make([]string, len(columns))
	for i, column := range columns {
		idents[i] = column.identifier
	}
	return idents
}
