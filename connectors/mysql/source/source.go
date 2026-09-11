// Package mysql implements the filament.Source interface as a full-snapshot,
// incremental-watermark, and CDC reader for MySQL (8.0+).
//
// Each requested resource is a table name. InnoDB stores rows clustered by primary
// key, so key order is physical order: a keyset read (WHERE pk > cursor ORDER BY pk
// LIMIT page) over the clustered index is sequential I/O. That removes the need for
// a physical read tier.
//
// Large tables split into several primary-key range shards so a single big table doesnt
// pin one worker while the rest sit idle (see planKeyset / shard_pages). An
// integer leading key splits by arithmetic over min/max; any other leading key splits
// by sampled equal-count boundaries.
//
// Rows are read as the driver's text-protocol values and parsed straight into
// the pipeline's row writers by type — see types.go — so no row is ever rendered
// as JSON on either side.
package mysql

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"math"

	"github.com/galaxy-io/filament"
	mysqlconnection "github.com/galaxy-io/filament/connectors/mysql/internal/connection"
)

const (
	// defaultPageSize bounds rows held in memory per keyset page.
	defaultPageSize = 1000

	// defaultShardPages is the target InnoDB page count per shard (16 KiB pages →
	// ~256 MiB). A table at or below this size reads as a single shard; bigger tables
	// split so they no longer bottleneck one worker. Set shard_pages=0 to disable
	// intra-table sharding (back to one shard per table).
	defaultShardPages = 16384

	// maxShardsPerTable caps the fan-out so a multi-terabyte table cannot spawn an
	// unbounded number of shards.
	maxShardsPerTable = 64
)

// Source reads tables from a MySQL database as a full snapshot. One instance is
// created per run: Configure opens the pool, Extract pages the tables, Teardown
// closes the pool.
type Source struct {
	db         *sql.DB
	dsn        string // driver DSN the pool was opened with; the CDC bootstrap opens its lock session from it
	database   string
	pageSize   int
	shardPages int

	// Per-resource incremental settings populated by PlanIncremental.
	cursorColumns   map[string]string
	cursorLookbacks map[string]int

	// Replication-client identity and endpoint for the CDC path (see cdc.go),
	// captured from the DSN at Configure.
	serverID     uint32
	snapshotMode filament.SnapshotMode
	binlogHost   string
	binlogPort   uint16
	binlogUser   string
	binlogPass   string
	binlogTLS    *tls.Config
}

// New returns an unconfigured source.
func New() *Source {
	return &Source{pageSize: defaultPageSize, shardPages: defaultShardPages}
}

// querier is the subset of database/sql used by the page loop, satisfied by *sql.DB,
// *sql.Conn, and *sql.Tx. PrepareContext lets the keyset loop prepare its page
// statement once per shard: database/sql otherwise turns every parameterized
// QueryContext into prepare → execute → close (three round trips per page).
type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
}

var (
	_ filament.Source               = (*Source)(nil)
	_ filament.Discoverable         = (*Source)(nil)
	_ filament.LiveValidatable      = (*Source)(nil)
	_ filament.SchemaProvider       = (*Source)(nil)
	_ filament.CursorColumnProvider = (*Source)(nil)
	_ filament.IncrementalPlanner   = (*Source)(nil)
	_ filament.Resumable            = (*Source)(nil)
	_ filament.ResumePlanner        = (*Source)(nil)
)

// Replication reports the mode the connection's config selects.
func (s *Source) Replication(cfg filament.Config) filament.ReplicationMode {
	if cfg.String("replication") == string(filament.ReplicationCDC) {
		return filament.ReplicationCDC
	}
	return filament.ReplicationStandard
}

// Validate rejects an invalid DSN or incomplete individual connection fields.
func (s *Source) Validate(cfg filament.Config) error {
	resolved, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql source: connection config: %w", err)
	}
	if cfg.String("database") == "" && resolved.DriverConfig.DBName == "" {
		return fmt.Errorf("mysql source: connection has no database and \"database\" is unset")
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the database.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql source: connection config: %w", err)
	}
	if database := cfg.String("database"); database != "" {
		resolved.DriverConfig.DBName = database
	}
	if err := mysqlconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("mysql source: %w", err)
	}
	return nil
}

// Configure reads dsn/database/page_size/shard_pages/max_conns and opens a connection
// pool. Set max_conns to at least the run's parallelism so concurrent shard reads
// each get a connection instead of queueing.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := mysqlconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("mysql source: connection config: %w", err)
	}
	mc := resolved.DriverConfig
	s.database = mc.DBName
	if v := cfg.String("database"); v != "" {
		s.database = v
		mc.DBName = v
	}
	if cfg.Has("page_size") {
		if n := cfg.Int("page_size"); n > 0 {
			s.pageSize = n
		}
	}
	if cfg.Has("shard_pages") {
		// 0 is a valid value here (disable sharding), so honor it as-is.
		s.shardPages = cfg.Int("shard_pages")
	}
	s.serverID = defaultServerID
	if cfg.Has("server_id") {
		if n := cfg.Int("server_id"); n > 0 && n <= math.MaxUint32 {
			s.serverID = uint32(n)
		}
	}
	s.snapshotMode = filament.SnapshotInitial
	if v := cfg.String("snapshot_mode"); v != "" {
		s.snapshotMode = filament.SnapshotMode(v)
	}
	if mc.Net == "tcp" {
		host, port, err := splitHostPort(mc.Addr)
		if err != nil {
			return fmt.Errorf("mysql source: replication address: %w", err)
		}
		s.binlogHost, s.binlogPort = host, port
	} else if s.Replication(cfg) == filament.ReplicationCDC {
		return fmt.Errorf("mysql source: CDC requires a TCP connection, got network %q", mc.Net)
	}
	s.binlogUser, s.binlogPass = mc.User, mc.Passwd
	s.binlogTLS = nil
	if mc.TLS != nil {
		s.binlogTLS = mc.TLS.Clone()
	}

	// Timestamp columns are read as text in the session zone; pin it so they
	// decode as UTC, the same instant the binlog reports.
	if mc.Params == nil {
		mc.Params = map[string]string{}
	}
	mc.Params["time_zone"] = "'+00:00'"
	// The decoder intentionally consumes MySQL's text protocol so DATE,
	// DATETIME, and TIMESTAMP all share the same zero-value and microsecond
	// handling. A user-supplied parseTime=true DSN would otherwise make the
	// driver re-render those values as RFC3339 before scanning into RawBytes.
	mc.ParseTime = false

	s.dsn = mc.FormatDSN()
	db, err := mysqlconnection.Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("mysql source: open: %w", err)
	}
	if cfg.Has("max_conns") {
		if n := cfg.Int("max_conns"); n > 0 {
			db.SetMaxOpenConns(n)
		}
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("mysql source: ping: %w", err)
	}
	s.db = db
	return nil
}

// Teardown closes the pool.
func (s *Source) Teardown(context.Context) error {
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}
	s.binlogTLS = nil
	return nil
}
