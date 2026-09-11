// Package postgres implements the filament.Source interface as a full-snapshot
// (ModeFull) reader for PostgreSQL.
//
// Each requested resource is a table name. A table's heap is sliced into one or
// more shards by physical ctid block range, and each shard is read window by window:
// WHERE ctid >= '(lo,0)' AND ctid < '(hi,0)' over a fixed span of blocks at a time.
// Reading by ctid (not the primary key) means one reader handles primary-key,
// composite-key, and keyless tables identically. On PostgreSQL 14+ each bounded
// window is a TID Range Scan:
// sequential heap I/O, no index needed. Both bounds matter — an open-ended ctid > x
// re-scans to end-of-table on every page, so every window is bounded on both sides.
//
// Large tables split into several shards so a single big table no longer pins one
// worker while the rest sit idle (see planShards / shard_pages). When more than one
// shard runs concurrently the reads share an exported snapshot (pg_export_snapshot),
// so every shard sees one consistent point-in-time even under concurrent writes —
// the same mechanism pg_dump -j uses.
//
// Rows are read as typed columns over pgx's binary protocol and appended straight
// into the pipeline's row writers — see columns.go — so neither the source
// database nor this process ever renders a row as text.
package postgres

import (
	"context"
	"fmt"
	"maps"
	"math"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/galaxy-io/filament"
	pgconnection "github.com/galaxy-io/filament/connectors/postgres/internal/connection"
)

const (
	// defaultPageSize bounds rows held in memory per keyset page.
	defaultPageSize = 1000

	// defaultSchema is the namespace resources resolve under when "schema" is unset.
	defaultSchema = "public"

	// defaultShardPages is the target heap-block count per shard (8 KiB blocks →
	// ~128 MiB). A table at or below this size reads as a single shard; bigger tables
	// split so they no longer bottleneck one worker. Set shard_pages=0 to disable
	// intra-table sharding (back to one shard per table).
	defaultShardPages = 16384

	// maxShardsPerTable caps the fan-out so a multi-terabyte table cannot spawn an
	// unbounded number of shards.
	maxShardsPerTable = 64

	// defaultRowsPerBlock is the rows/block estimate used when block 0 is empty (so
	// the live sample is unusable). ~60 narrow rows per 8 KiB block is a reasonable
	// middle.
	defaultRowsPerBlock = 60

	// maxWindowBlocks caps a read window so a table of unexpectedly dense (tiny) rows
	// cannot make one window pull an unbounded number of rows into memory.
	maxWindowBlocks = 256

	defaultPublication = "filament"
	defaultSlotName    = "filament"

	// The replication config values: query-based reads or the WAL stream.
	replicationStandard = "standard"
	replicationCDC      = "cdc"
)

// Source reads tables from a PostgreSQL database as a full snapshot. One instance
// is created per run: Configure opens the pool, Extract pages the tables, Teardown
// closes the pool.
type Source struct {
	pool              *pgxpool.Pool
	dsn               string
	schema            string
	pageSize          int
	shardPages        int
	readMode          string // "", "keyset" (key-ordered), "bitmap" (unordered sub-ranges), "auto" (probe)
	cursorColumns     map[string]string
	cursorLookbacks   map[string]int
	publication       string
	slotName          string
	managePublication bool
	snapshotMode      filament.SnapshotMode
}

// New returns an unconfigured source.
func New() *Source {
	return &Source{
		schema: defaultSchema, pageSize: defaultPageSize, shardPages: defaultShardPages,
		publication: defaultPublication, slotName: defaultSlotName, managePublication: true,
		snapshotMode: filament.SnapshotInitial,
	}
}

var (
	_ filament.Source                   = (*Source)(nil)
	_ filament.Discoverable             = (*Source)(nil)
	_ filament.LiveValidatable          = (*Source)(nil)
	_ filament.SchemaProvider           = (*Source)(nil)
	_ filament.Resumable                = (*Source)(nil)
	_ filament.ResumePlanner            = (*Source)(nil)
	_ filament.IncrementalPlanner       = (*Source)(nil)
	_ filament.CursorColumnProvider     = (*Source)(nil)
	_ filament.ChangeSource             = (*Source)(nil)
	_ filament.ChangeAcknowledger       = (*Source)(nil)
	_ filament.ReplicationStreamPlanner = (*Source)(nil)
	_ filament.ReplicationStreamCleaner = (*Source)(nil)
)

// Replication reports the mode the connection's config selects.
func (s *Source) Replication(cfg filament.Config) filament.ReplicationMode {
	if cfg.String("replication") == replicationCDC {
		return filament.ReplicationCDC
	}
	return filament.ReplicationStandard
}

// PlanReplicationStream gives one pipeline route its own logical replication
// slot while keeping the publication shared at the connection level.
func (s *Source) PlanReplicationStream(request filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	if request.ReplicationStreamID == "" {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("postgres source: replication stream ID is required")
	}
	consumerName := "filament_" + strings.ReplaceAll(request.ReplicationStreamID, "-", "")
	if !validReplicationName(consumerName) {
		return filament.ReplicationStreamPlan{}, fmt.Errorf("postgres source: generated slot name %q is invalid", consumerName)
	}
	publication := request.Config.String("publication")
	if publication == "" {
		publication = defaultPublication
	}
	schema := request.Config.String("schema")
	if schema == "" {
		schema = defaultSchema
	}
	return filament.ReplicationStreamPlan{
		ConsumerName: consumerName,
		ConsumerConfig: map[string]any{
			"kind": "postgres_lsn", "publication": publication,
		},
		ContinuityConfig: map[string]any{
			"schema": schema, "publication": publication,
		},
	}, nil
}

// BindReplicationStream injects the resolved generation's slot into a run
// without mutating the stored connection or pipeline configuration.
func (s *Source) BindReplicationStream(config map[string]any, stream filament.ReplicationStream) (map[string]any, error) {
	if stream.ConsumerName == "" || !validReplicationName(stream.ConsumerName) {
		return nil, fmt.Errorf("postgres source: replication stream has invalid slot name %q", stream.ConsumerName)
	}
	out := maps.Clone(config)
	if out == nil {
		out = map[string]any{}
	}
	out["slot_name"] = stream.ConsumerName
	return out, nil
}

// CleanupReplicationStream drops a retired logical slot after its successor
// has committed. Missing slots are already clean and therefore succeed.
func (s *Source) CleanupReplicationStream(ctx context.Context, stream filament.ReplicationStream) error {
	if s.pool == nil {
		return fmt.Errorf("postgres source: cleanup replication stream before configure")
	}
	if !validReplicationName(stream.ConsumerName) {
		return fmt.Errorf("postgres source: replication stream has invalid slot name %q", stream.ConsumerName)
	}
	if _, err := s.pool.Exec(ctx, `SELECT pg_drop_replication_slot(slot_name)
		FROM pg_replication_slots
		WHERE slot_name=$1`, stream.ConsumerName); err != nil {
		return fmt.Errorf("postgres cdc: drop retired slot %q: %w", stream.ConsumerName, err)
	}
	return nil
}

// Validate rejects an invalid URL or incomplete individual connection fields.
func (s *Source) Validate(cfg filament.Config) error {
	if _, err := pgconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("postgres source: connection config: %w", err)
	}
	for field, fallback := range map[string]string{"publication": defaultPublication, "slot_name": defaultSlotName} {
		value := cfg.String(field)
		if value == "" {
			value = fallback
		}
		if !validReplicationName(value) {
			return fmt.Errorf("postgres source: %s %q must be a lowercase PostgreSQL identifier", field, value)
		}
	}
	return nil
}

// TestConnection opens a short-lived pool and pings the database.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	resolved, err := pgconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("postgres source: connection config: %w", err)
	}
	if err := pgconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("postgres source: %w", err)
	}
	return nil
}

// Configure reads dsn/schema/page_size/shard_pages/max_conns and opens a connection
// pool. Set max_conns to at least the run's parallelism + 1 (the snapshot
// coordinator holds one connection) so concurrent shard reads each get a connection
// instead of queueing; unset keeps pgx's default (max(4, NumCPU)).
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	if v := cfg.String("schema"); v != "" {
		s.schema = v
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
	if v := cfg.String("scan_strategy"); v != "" {
		s.readMode = v
	}
	if v := cfg.String("publication"); v != "" {
		s.publication = v
	}
	// slot_name is runtime-only. ReplicationStreamPlanner binds the durable
	// stream's generated slot after user-facing connection/pipeline validation.
	if v := cfg.String("slot_name"); v != "" {
		s.slotName = v
	}
	if cfg.Has("manage_publication") {
		s.managePublication = cfg.Bool("manage_publication")
	}
	if v := cfg.String("snapshot_mode"); v != "" {
		s.snapshotMode = filament.SnapshotMode(v)
	}
	resolved, err := pgconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("postgres source: connection config: %w", err)
	}
	s.dsn = resolved.DSN
	poolCfg := resolved.DriverConfig
	if cfg.Has("max_conns") {
		if n := cfg.Int("max_conns"); n > 0 && n <= math.MaxInt32 {
			poolCfg.MaxConns = int32(n)
		}
	}
	pool, err := pgconnection.Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("postgres source: open pool: %w", err)
	}
	s.pool = pool
	return nil
}

// Teardown closes the pool.
func (s *Source) Teardown(context.Context) error {
	if s.pool != nil {
		s.pool.Close()
		s.pool = nil
	}
	return nil
}
