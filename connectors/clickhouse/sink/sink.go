// Package clickhouse implements a typed ClickHouse sink.
package clickhouse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync/atomic"

	chdriver "github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"github.com/galaxy-io/filament"
	clickhouseconnection "github.com/galaxy-io/filament/connectors/clickhouse/internal/connection"
)

type connection interface {
	Exec(context.Context, string, ...any) error
	Query(context.Context, string, ...any) (chdriver.Rows, error)
	QueryRow(context.Context, string, ...any) chdriver.Row
	PrepareBatch(context.Context, string, ...chdriver.PrepareBatchOption) (chdriver.Batch, error)
	Ping(context.Context) error
	Close() error
}

// Sink writes typed batches to MergeTree tables and implements key-based upsert
// with ReplacingMergeTree. Snapshot replacement writes into run-scoped staging
// tables and swaps each resource into place only on Commit.
type Sink struct {
	conn     connection
	database string
	run      filament.RunID
	written  atomic.Int64
	policies map[string]filament.WritePolicy
	tables   map[string]*table
}

// modeFor returns the write mode governing one resource: its bound policy, or
// the run-wide policy for runs with no explicit resource list.
func (s *Sink) modeFor(resource string) filament.WriteMode {
	if p, ok := s.policies[resource]; ok {
		return p.Capability.Mode
	}
	return s.policies[""].Capability.Mode
}

type table struct {
	name      string
	qualified string
	writeTo   string
	stage     string
	insertSQL string
}

// New returns an unconfigured ClickHouse sink. Open establishes its connection.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "clickhouse" }

// Validate checks connection syntax without opening a network connection.
func (s *Sink) Validate(cfg filament.Config) error {
	if _, err := clickhouseconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("clickhouse sink: connection config: %w", err)
	}
	return nil
}

// TestConnection pings ClickHouse through a short-lived connection without
// creating the configured destination database.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := clickhouseconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("clickhouse sink: connection config: %w", err)
	}
	if err := clickhouseconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("clickhouse sink: %w", err)
	}
	return nil
}

// Open connects to ClickHouse and prepares per-run state.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := clickhouseconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("clickhouse sink: connection config: %w", err)
	}
	s.database = cfg.String("database")
	if s.database == "" {
		s.database = clickhouseconnection.DefaultDatabase
	}
	// Connect through the always-present default database so a pipeline can create
	// its destination database on first use.
	opts := resolved.DriverConfig
	opts.Auth.Database = clickhouseconnection.DefaultDatabase
	if n := run.Options.SnapshotParallelism; n > 0 {
		opts.MaxOpenConns = max(opts.MaxOpenConns, n)
	}
	conn, err := clickhouseconnection.Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("clickhouse sink: open: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return fmt.Errorf("clickhouse sink: ping over %s to %s: %w", opts.Protocol, opts.Addr[0], err)
	}
	if err := conn.Exec(ctx, "CREATE DATABASE IF NOT EXISTS "+quoteIdent(s.database)); err != nil {
		_ = conn.Close()
		return fmt.Errorf("clickhouse sink: create database %q: %w", s.database, err)
	}

	s.conn = conn
	s.run = run.Run
	s.written.Store(0)
	s.policies = run.WritePolicies
	s.tables = map[string]*table{}
	return nil
}

// Commit promotes any snapshot-replacement stages and closes the connection.
func (s *Sink) Commit(ctx context.Context) error {
	defer s.release()
	if s.conn == nil {
		return nil
	}
	for _, tbl := range s.tables {
		if tbl.stage == "" {
			continue
		}
		var exists uint8
		if err := s.conn.QueryRow(ctx, existsTableQuery(tbl.qualified)).Scan(&exists); err != nil {
			return fmt.Errorf("clickhouse sink: inspect destination %q: %w", tbl.name, err)
		}
		if exists == 0 {
			if err := s.conn.Exec(ctx, "RENAME TABLE "+tbl.writeTo+" TO "+tbl.qualified); err != nil {
				return fmt.Errorf("clickhouse sink: promote %q: %w", tbl.name, err)
			}
			continue
		}
		if err := s.conn.Exec(ctx, "EXCHANGE TABLES "+tbl.qualified+" AND "+tbl.writeTo); err != nil {
			return fmt.Errorf("clickhouse sink: replace %q: %w", tbl.name, err)
		}
		if err := s.conn.Exec(ctx, "DROP TABLE "+tbl.writeTo); err != nil {
			return fmt.Errorf("clickhouse sink: drop replaced table %q: %w", tbl.name, err)
		}
	}
	return nil
}

func existsTableQuery(qualifiedTable string) string {
	return "EXISTS TABLE " + qualifiedTable
}

// Abort drops uncommitted snapshot stages and closes the connection.
func (s *Sink) Abort(ctx context.Context) error {
	defer s.release()
	if s.conn == nil {
		return nil
	}
	cleanup := context.WithoutCancel(ctx)
	var first error
	for _, tbl := range s.tables {
		if tbl.stage == "" {
			continue
		}
		if err := s.conn.Exec(cleanup, "DROP TABLE IF EXISTS "+tbl.writeTo); err != nil && first == nil {
			first = fmt.Errorf("clickhouse sink: drop stage for %q: %w", tbl.name, err)
		}
	}
	return first
}

func (s *Sink) release() {
	if s.conn != nil {
		_ = s.conn.Close()
		s.conn = nil
	}
}

func stageTableName(run filament.RunID, resource string) string {
	sum := sha256.Sum256([]byte(string(run) + "\x00" + resource))
	return "__filament_stage_" + hex.EncodeToString(sum[:8])
}
