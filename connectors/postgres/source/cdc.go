package postgres

// PostgreSQL change-data-capture uses a persistent logical replication slot and
// the built-in pgoutput plugin. A resource without a stream checkpoint is first
// read in full from a snapshot pinned by the slot; WAL is retained from the
// snapshot's consistent point while that scan runs. Each extraction then becomes
// a bounded catch-up cycle: it captures pg_current_wal_lsn(), replays committed
// changes from the oldest resource checkpoint through that watermark, emits a
// stream marker for every resource, and returns. The next engine cycle resumes
// from those markers without taking another snapshot.

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

const standbyStatusInterval = 10 * time.Second

// slotReleaseWait bounds how long a cycle waits for a peer to let go of the
// slot: the previous cycle's walsender still exiting after Close, or its
// pg_replication_slot_advance still decoding toward the acknowledged cursor.
const (
	slotReleaseWait = 10 * time.Second
	slotReleasePoll = 100 * time.Millisecond
)

// errSlotHeld marks a slot operation refused because another backend holds
// the slot right now; untilSlotReleased retries such operations.
var errSlotHeld = errors.New("slot is held by another backend")

// slotHeldByPeer reports whether a server error is PostgreSQL refusing to
// acquire an active slot (SQLSTATE 55006, object_in_use).
func slotHeldByPeer(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "55006"
}

// untilSlotReleased runs op until it stops reporting errSlotHeld or
// slotReleaseWait elapses, returning op's last error.
func untilSlotReleased(ctx context.Context, op func() error) error {
	deadline := time.Now().Add(slotReleaseWait)
	for {
		err := op()
		if !errors.Is(err, errSlotHeld) || time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(slotReleasePoll):
		}
	}
}

var replicationNameRE = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func validReplicationName(name string) bool { return replicationNameRE.MatchString(name) }

// cdcRelation is one relation the stream describes, bound to the resource's
// schema: cols maps each schema field to its tuple column (-1 = not sent, so
// null), parsers read each field's text form, keys are the tuple columns of
// the primary key.
type cdcRelation struct {
	message *pglogrepl.RelationMessage
	tracked bool
	schema  rowmodel.Schema
	cols    []int
	parsers []func(arrowbatch.RowWriter, []byte) error
	keys    []int
}

// cdcTable is one tracked resource: its primary key, catalog schema, and the row
// writer changes are appended into (opened on first use).
type cdcTable struct {
	pks    []string
	schema rowmodel.Schema
	writer arrowbatch.RowWriter
}

type pgCDCRun struct {
	sink      arrowbatch.Inlet
	schema    string
	resources []string
	tracked   map[string]*cdcTable
	relations map[uint32]*cdcRelation
	seq       uint64
	emitted   int
	limit     int
	limited   bool
	inTxn     bool
	lastLSN   pglogrepl.LSN
	floors    map[string]pglogrepl.LSN
}

// writer returns the resource's row writer, opening it on first use.
func (r *pgCDCRun) writer(resource string) (arrowbatch.RowWriter, error) {
	t := r.tracked[resource]
	if t.writer == nil {
		w, err := r.sink.Builder(resource, 0, t.schema)
		if err != nil {
			return nil, err
		}
		t.writer = w
	}
	return t.writer, nil
}

type replicationSlotState struct {
	start      pglogrepl.LSN
	snapshotID string
}

// ExtractChanges replays pgoutput row changes for the selected tables. The
// publication is created/expanded by default; set manage_publication=false when
// database administration owns it instead. A slot is never dropped automatically:
// its stable identity is what makes a later catch-up lossless.
//
//nolint:gocyclo,funlen // Complex replication protocol handling; refactor deferred.
func (s *Source) ExtractChanges(ctx context.Context, sink arrowbatch.Inlet, opts filament.ChangeExtractOpts) error {
	if s.pool == nil || s.dsn == "" {
		return fmt.Errorf("postgres source: extract changes before configure")
	}
	if len(opts.Resources) == 0 {
		return nil
	}
	tracked := make(map[string]*cdcTable, len(opts.Resources))
	for _, resource := range opts.Resources {
		schema, err := s.Schema(ctx, resource)
		if err != nil {
			return fmt.Errorf("postgres cdc: schema %q: %w", resource, err)
		}
		if len(schema.PrimaryKey) == 0 {
			return fmt.Errorf("postgres cdc: resource %q has no primary key", resource)
		}
		tracked[resource] = &cdcTable{pks: schema.PrimaryKey, schema: schema}
	}
	if err := s.ensurePublication(ctx, opts.Resources); err != nil {
		return err
	}
	run := &pgCDCRun{
		sink: sink, schema: s.schema, resources: opts.Resources, tracked: tracked,
		relations: map[uint32]*cdcRelation{}, limit: opts.Limit,
	}

	repl, err := s.replicationConn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = repl.Close(context.WithoutCancel(ctx)) }()

	slot, err := s.ensureReplicationSlot(ctx, repl)
	if err != nil {
		return err
	}
	start, seq, haveCheckpoint, err := startPostgresLSN(opts.Checkpoints)
	if err != nil {
		return err
	}
	if !haveCheckpoint {
		start = slot.start
	} else if start < slot.start {
		return fmt.Errorf("postgres cdc: checkpoint %s is older than slot %q confirmed position %s; WAL was consumed by another client", start, s.slotName, slot.start)
	}
	run.floors, err = postgresResourceFloors(opts.Checkpoints)
	if err != nil {
		return err
	}

	// A missing per-resource checkpoint means this resource has never completed
	// its bootstrap. Plan the scan before opening the pool-backed transaction so
	// max_conns=1 remains usable. The slot is already retaining every concurrent
	// change. A newly-created slot supplies the exact exported snapshot at its
	// consistent point; an existing slot (for example after an interrupted first
	// attempt) supplies the older replay position while a fresh snapshot is read.
	//
	// Under SnapshotNone no baseline is read: the resource's floor is the
	// slot's consistent point (new slot) or the current WAL position (existing
	// slot), and its stream starts there.
	bootstrapResources := resourcesWithoutCheckpoints(opts.Resources, opts.Checkpoints)
	var bootstrapShards []shard
	var bootstrapSnapshot *snapshot
	bootstrapFloor := slot.start
	switch {
	case len(bootstrapResources) == 0:
	case s.snapshotMode == filament.SnapshotNone:
		if slot.snapshotID == "" {
			bootstrapFloor, err = s.currentLSN(ctx)
			if err != nil {
				return err
			}
		}
		for _, resource := range bootstrapResources {
			run.floors[resource] = bootstrapFloor
		}
	default:
		bootstrapShards, err = s.planShards(ctx, bootstrapResources, 1)
		if err != nil {
			return fmt.Errorf("postgres cdc: plan initial snapshot: %w", err)
		}
		if slot.snapshotID != "" {
			bootstrapSnapshot, err = s.importSnapshot(ctx, slot.snapshotID)
		} else {
			// Capture a conservative floor before opening the fresh snapshot.
			// Replaying a transaction that races this boundary is safe; choosing a
			// later floor could skip a transaction the snapshot cannot see.
			bootstrapFloor, err = s.currentLSN(ctx)
			if err != nil {
				return err
			}
			bootstrapSnapshot, err = s.openSnapshot(ctx)
		}
		if err != nil {
			return fmt.Errorf("postgres cdc: open initial snapshot: %w", err)
		}
		defer func() {
			if bootstrapSnapshot != nil {
				bootstrapSnapshot.close(context.WithoutCancel(ctx))
			}
		}()
		for _, resource := range bootstrapResources {
			run.floors[resource] = bootstrapFloor
		}
	}

	// Complete the baseline before capturing its catch-up target. pg_current_wal_lsn
	// is not constrained by transaction snapshot visibility, so querying it through
	// the held snapshot transaction sees commits made while the scan ran without
	// requiring a second pool connection. Those commits are retained by the slot
	// and replayed in this same cycle.
	var watermark pglogrepl.LSN
	if bootstrapSnapshot != nil {
		if err := s.extractInitialSnapshot(ctx, run, bootstrapSnapshot.tx, bootstrapShards); err != nil {
			return err
		}
		watermark, err = currentLSNFrom(ctx, bootstrapSnapshot.tx)
		bootstrapSnapshot.close(context.WithoutCancel(ctx))
		bootstrapSnapshot = nil
	} else {
		watermark, err = s.currentLSN(ctx)
	}
	if err != nil {
		return err
	}
	if start > watermark {
		return fmt.Errorf("postgres cdc: checkpoint %s is ahead of server watermark %s (timeline changed?)", start, watermark)
	}

	if err := pglogrepl.StartReplication(ctx, repl, s.slotName, start, pglogrepl.StartReplicationOptions{
		PluginArgs: []string{"proto_version '1'", "publication_names '" + s.publication + "'"},
	}); err != nil {
		return fmt.Errorf("postgres cdc: start slot %q at %s: %w", s.slotName, start, err)
	}
	// Standby replies confirm the slot's own position, a no-op that keeps the
	// walsender satisfied. The slot advances in exactly one place,
	// AcknowledgeChanges, once the runner has proven the cursors durable.
	if err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{WALWritePosition: slot.start}); err != nil {
		return fmt.Errorf("postgres cdc: acknowledge start %s: %w", start, err)
	}

	run.seq = seq
	if start >= watermark {
		return run.pushStreamMarks(start)
	}

	nextStatus := time.Now().Add(standbyStatusInterval)
	for {
		receiveCtx, cancel := context.WithDeadline(ctx, nextStatus)
		raw, recvErr := repl.ReceiveMessage(receiveCtx)
		cancel()
		if recvErr != nil {
			if pgconn.Timeout(recvErr) {
				if err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{WALWritePosition: slot.start}); err != nil {
					return fmt.Errorf("postgres cdc: standby status: %w", err)
				}
				nextStatus = time.Now().Add(standbyStatusInterval)
				continue
			}
			return fmt.Errorf("postgres cdc: receive WAL at %s: %w", run.lastLSN, recvErr)
		}
		switch msg := raw.(type) {
		case *pgproto3.ErrorResponse:
			return fmt.Errorf("postgres cdc: WAL stream: %w", pgconn.ErrorResponseToPgError(msg))
		case *pgproto3.CopyData:
			if len(msg.Data) == 0 {
				continue
			}
			switch msg.Data[0] {
			case pglogrepl.PrimaryKeepaliveMessageByteID:
				keepalive, err := pglogrepl.ParsePrimaryKeepaliveMessage(msg.Data[1:])
				if err != nil {
					return fmt.Errorf("postgres cdc: keepalive: %w", err)
				}
				if keepalive.ReplyRequested {
					if err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{WALWritePosition: slot.start}); err != nil {
						return fmt.Errorf("postgres cdc: keepalive reply: %w", err)
					}
					nextStatus = time.Now().Add(standbyStatusInterval)
				}
				if !run.inTxn && keepalive.ServerWALEnd >= watermark {
					mark := maxLSN(watermark, run.lastLSN)
					return run.pushStreamMarks(mark)
				}
			case pglogrepl.XLogDataByteID:
				xld, err := pglogrepl.ParseXLogData(msg.Data[1:])
				if err != nil {
					return fmt.Errorf("postgres cdc: xlog data: %w", err)
				}
				run.lastLSN = xld.WALStart
				committed, err := run.process(xld.WALData, xld.WALStart)
				if err != nil {
					return err
				}
				if committed != 0 {
					run.lastLSN = committed
				}
				if run.limited && committed != 0 {
					return run.pushStreamMarks(committed)
				}
				if committed >= watermark {
					return run.pushStreamMarks(committed)
				}
			}
		}
	}
}

func (s *Source) ensurePublication(ctx context.Context, resources []string) error {
	var walLevel string
	if err := s.pool.QueryRow(ctx, "SHOW wal_level").Scan(&walLevel); err != nil {
		return fmt.Errorf("postgres cdc: read wal_level: %w", err)
	}
	if walLevel != "logical" {
		return fmt.Errorf("postgres cdc: wal_level is %q, want logical", walLevel)
	}

	var exists, allTables, inserts, updates, deletes bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname=$1),
		COALESCE((SELECT puballtables FROM pg_publication WHERE pubname=$1), false),
		COALESCE((SELECT pubinsert FROM pg_publication WHERE pubname=$1), false),
		COALESCE((SELECT pubupdate FROM pg_publication WHERE pubname=$1), false),
		COALESCE((SELECT pubdelete FROM pg_publication WHERE pubname=$1), false)`, s.publication).
		Scan(&exists, &allTables, &inserts, &updates, &deletes)
	if err != nil {
		return fmt.Errorf("postgres cdc: inspect publication %q: %w", s.publication, err)
	}
	columnLists, err := s.publicationColumnLists(ctx, resources)
	if err != nil {
		return err
	}
	if !exists {
		if !s.managePublication {
			return fmt.Errorf("postgres cdc: publication %q does not exist", s.publication)
		}
		clauses := make([]string, len(resources))
		for i, resource := range resources {
			clauses[i] = s.publicationTableClause(resource, columnLists[resource])
		}
		stmt := "CREATE PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " FOR TABLE " + strings.Join(clauses, ", ") + " WITH (publish = 'insert, update, delete')"
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("postgres cdc: create publication %q: %w", s.publication, err)
		}
		return nil
	}
	if !inserts || !updates || !deletes {
		if !s.managePublication {
			return fmt.Errorf("postgres cdc: publication %q must publish inserts, updates, and deletes", s.publication)
		}
		stmt := "ALTER PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " SET (publish = 'insert, update, delete')"
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("postgres cdc: configure publication operations: %w", err)
		}
	}
	if allTables {
		return nil
	}

	published, err := s.publishedTables(ctx)
	if err != nil {
		return err
	}
	var missing, replace, retired []string
	for _, resource := range resources {
		entry, ok := published[resource]
		if !ok {
			missing = append(missing, s.publicationTableClause(resource, columnLists[resource]))
			continue
		}
		needsReplace, err := s.reconcilePublishedColumns(resource, columnLists[resource], entry)
		if err != nil {
			return err
		}
		if needsReplace {
			replace = append(replace, resource)
		}
	}
	for table := range published {
		if !slices.Contains(resources, table) {
			retired = append(retired, table)
		}
	}
	if len(missing) == 0 && len(replace) == 0 && (len(retired) == 0 || !s.managePublication) {
		return nil
	}
	if !s.managePublication {
		return fmt.Errorf("postgres cdc: publication %q is missing selected tables: %s", s.publication, strings.Join(missing, ", "))
	}
	return s.applyPublicationTables(ctx, replace, retired, missing, columnLists)
}

// applyPublicationTables rewrites the listed tables' column lists, drops the
// tables the stream no longer selects and adds the missing clauses in one
// transaction, so a commit between the drop and the add is never decoded
// against a publication that lacks the table.
func (s *Source) applyPublicationTables(ctx context.Context, replace, retired, missing []string, columnLists map[string][]string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("postgres cdc: update publication: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	for _, resource := range replace {
		stmt := "ALTER PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " DROP TABLE " + pgx.Identifier{s.schema, resource}.Sanitize()
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("postgres cdc: reset publication columns for %s: %w", resource, err)
		}
		missing = append(missing, s.publicationTableClause(resource, columnLists[resource]))
	}
	for _, resource := range retired {
		stmt := "ALTER PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " DROP TABLE " + pgx.Identifier{s.schema, resource}.Sanitize()
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("postgres cdc: drop retired publication table %s: %w", resource, err)
		}
	}
	if len(missing) > 0 {
		stmt := "ALTER PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " ADD TABLE " + strings.Join(missing, ", ")
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("postgres cdc: add publication tables: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("postgres cdc: update publication: %w", err)
	}
	return nil
}

// reconcilePublishedColumns reports whether a published table's column list
// must be rewritten to the one this run wants. A managed publication belongs
// to this stream alone, so its lists are exactly the selection. An unmanaged
// one is never rewritten and must already publish every column the run reads.
func (s *Source) reconcilePublishedColumns(resource string, desired []string, entry publishedTable) (bool, error) {
	if !s.managePublication {
		return false, s.checkPublishedColumns(resource, entry)
	}
	return entry.filtered != (desired != nil) || !sameColumnSet(entry.columns, desired), nil
}

// checkPublishedColumns rejects a publication column list that would hide a
// column this run reads. The decoder would otherwise null the unpublished
// columns.
func (s *Source) checkPublishedColumns(resource string, entry publishedTable) error {
	if !entry.filtered {
		return nil
	}
	selected, ok := s.selected[resource]
	if !ok {
		return fmt.Errorf("postgres cdc: publication %q limits the columns of %s; select the published columns or publish every column", s.publication, resource)
	}
	if !subsetOf(selected, entry.columns) {
		return fmt.Errorf("postgres cdc: publication %q does not publish every selected column of %s", s.publication, resource)
	}
	return nil
}

// publicationColumnLists returns the column list each selected table publishes
// with, or nil for every column. A publication column list needs Postgres 15
// and must cover the table's replica identity, or Postgres rejects the
// application's own updates and deletes. A table with REPLICA IDENTITY FULL,
// or an identity index the selection does not cover, publishes every column
// and the decoder drops the unselected ones.
func (s *Source) publicationColumnLists(ctx context.Context, resources []string) (map[string][]string, error) {
	lists := make(map[string][]string, len(s.selected))
	if len(s.selected) == 0 || !s.managePublication {
		return lists, nil
	}
	version, err := s.serverVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version < 150000 {
		return lists, nil
	}
	for _, resource := range resources {
		selected, ok := s.selected[resource]
		if !ok {
			continue
		}
		var identity string
		var identityColumns []string
		if err := s.pool.QueryRow(ctx, `SELECT c.relreplident, COALESCE((
			SELECT array_agg(a.attname ORDER BY k.ord)
			FROM pg_index i
			JOIN LATERAL unnest(i.indkey) WITH ORDINALITY AS k(attnum, ord) ON true
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = k.attnum
			WHERE i.indrelid = c.oid AND i.indisreplident), '{}')::text[]
			FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace WHERE n.nspname = $1 AND c.relname = $2`, s.schema, resource).Scan(&identity, &identityColumns); err != nil {
			return nil, fmt.Errorf("postgres cdc: replica identity of %s: %w", resource, err)
		}
		if identity == "f" || (identity == "i" && !subsetOf(identityColumns, selected)) {
			continue
		}
		lists[resource] = selected
	}
	return lists, nil
}

func (s *Source) serverVersion(ctx context.Context) (int, error) {
	var raw string
	if err := s.pool.QueryRow(ctx, "SHOW server_version_num").Scan(&raw); err != nil {
		return 0, fmt.Errorf("postgres cdc: read server version: %w", err)
	}
	version, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("postgres cdc: parse server version %q: %w", raw, err)
	}
	return version, nil
}

// publicationTableClause renders one table for CREATE or ALTER PUBLICATION,
// with its column list when it publishes a subset.
func (s *Source) publicationTableClause(resource string, columns []string) string {
	clause := pgx.Identifier{s.schema, resource}.Sanitize()
	if len(columns) == 0 {
		return clause
	}
	names := make([]string, len(columns))
	for i, name := range columns {
		names[i] = pgx.Identifier{name}.Sanitize()
	}
	return clause + " (" + strings.Join(names, ", ") + ")"
}

// publishedTable is one table of the publication: whether it carries a column
// list and, when it does, the published columns.
type publishedTable struct {
	filtered bool
	columns  []string
}

func (s *Source) publishedTables(ctx context.Context) (map[string]publishedTable, error) {
	version, err := s.serverVersion(ctx)
	if err != nil {
		return nil, err
	}
	query := `SELECT tablename, false, NULL::text[] FROM pg_publication_tables WHERE pubname=$1 AND schemaname=$2`
	if version >= 150000 {
		query = `SELECT pt.tablename, pr.prattrs IS NOT NULL, pt.attnames::text[]
FROM pg_publication_tables pt
JOIN pg_publication p ON p.pubname = pt.pubname
JOIN pg_namespace n ON n.nspname = pt.schemaname
JOIN pg_class c ON c.relnamespace = n.oid AND c.relname = pt.tablename
LEFT JOIN pg_publication_rel pr ON pr.prpubid = p.oid AND pr.prrelid = c.oid
WHERE pt.pubname=$1 AND pt.schemaname=$2`
	}
	rows, err := s.pool.Query(ctx, query, s.publication, s.schema)
	if err != nil {
		return nil, fmt.Errorf("postgres cdc: publication tables: %w", err)
	}
	defer rows.Close()
	published := map[string]publishedTable{}
	for rows.Next() {
		var table string
		var entry publishedTable
		if err := rows.Scan(&table, &entry.filtered, &entry.columns); err != nil {
			return nil, err
		}
		if !entry.filtered {
			entry.columns = nil
		}
		published[table] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres cdc: publication tables: %w", err)
	}
	return published, nil
}

func sameColumnSet(a, b []string) bool {
	return len(a) == len(b) && subsetOf(a, b)
}

func subsetOf(inner, outer []string) bool {
	for _, name := range inner {
		if !slices.Contains(outer, name) {
			return false
		}
	}
	return true
}

func (s *Source) replicationConn(ctx context.Context) (*pgconn.PgConn, error) {
	cfg, err := pgconn.ParseConfig(s.dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres cdc: parse replication dsn: %w", err)
	}
	if cfg.RuntimeParams == nil {
		cfg.RuntimeParams = map[string]string{}
	}
	cfg.RuntimeParams["replication"] = "database"
	// pgoutput renders tuples with the walsender's output functions; the parsers
	// in types.go read ISO dates and hex bytea, so pin both for the session.
	cfg.RuntimeParams["DateStyle"] = "ISO, MDY"
	cfg.RuntimeParams["bytea_output"] = "hex"
	conn, err := pgconn.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres cdc: connect replication protocol: %w", err)
	}
	return conn, nil
}

func (s *Source) ensureReplicationSlot(ctx context.Context, conn *pgconn.PgConn) (replicationSlotState, error) {
	var plugin, database string
	var confirmed, restart *string
	err := untilSlotReleased(ctx, func() error {
		var active bool
		err := s.pool.QueryRow(ctx, `SELECT plugin, database, active, confirmed_flush_lsn::text, restart_lsn::text
			FROM pg_replication_slots WHERE slot_name=$1`, s.slotName).Scan(&plugin, &database, &active, &confirmed, &restart)
		if err != nil {
			return err
		}
		if active {
			return fmt.Errorf("postgres cdc: slot %q is already active; use a unique slot per pipeline (%w)", s.slotName, errSlotHeld)
		}
		return nil
	})
	if errors.Is(err, errSlotHeld) {
		return replicationSlotState{}, err
	}
	if err != nil && err != pgx.ErrNoRows {
		return replicationSlotState{}, fmt.Errorf("postgres cdc: inspect slot %q: %w", s.slotName, err)
	}
	if err == pgx.ErrNoRows {
		created, err := pglogrepl.CreateReplicationSlot(ctx, conn, s.slotName, "pgoutput", pglogrepl.CreateReplicationSlotOptions{SnapshotAction: "EXPORT_SNAPSHOT"})
		if err != nil {
			return replicationSlotState{}, fmt.Errorf("postgres cdc: create slot %q: %w", s.slotName, err)
		}
		lsn, err := pglogrepl.ParseLSN(created.ConsistentPoint)
		if err != nil {
			return replicationSlotState{}, fmt.Errorf("postgres cdc: slot consistent point %q: %w", created.ConsistentPoint, err)
		}
		return replicationSlotState{start: lsn, snapshotID: created.SnapshotName}, nil
	}
	if plugin != "pgoutput" {
		return replicationSlotState{}, fmt.Errorf("postgres cdc: slot %q uses plugin %q, want pgoutput", s.slotName, plugin)
	}
	var currentDB string
	if err := s.pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
		return replicationSlotState{}, err
	}
	if database != currentDB {
		return replicationSlotState{}, fmt.Errorf("postgres cdc: slot %q belongs to database %q, want %q", s.slotName, database, currentDB)
	}
	position := confirmed
	if position == nil || *position == "" {
		position = restart
	}
	if position == nil || *position == "" {
		return replicationSlotState{}, fmt.Errorf("postgres cdc: slot %q has no restart position", s.slotName)
	}
	lsn, err := pglogrepl.ParseLSN(*position)
	if err != nil {
		return replicationSlotState{}, fmt.Errorf("postgres cdc: slot position %q: %w", *position, err)
	}
	return replicationSlotState{start: lsn}, nil
}

// AcknowledgeChanges advances the logical slot to the oldest checkpoint the
// runner has verified durable after sink commit. ExtractChanges cannot do this
// itself: at extraction return time the sink and tracker may still fail. The
// standby replies sent while streaming only confirm a flush position; advancing
// decodes the retained WAL so restart_lsn moves and the server can recycle it.
func (s *Source) AcknowledgeChanges(ctx context.Context, checkpoints map[string]filament.Checkpoint) error {
	target, _, found, err := startPostgresLSN(checkpoints)
	if err != nil || !found {
		return err
	}
	// GREATEST keeps the call monotonic: a target behind confirmed_flush_lsn
	// is a no-op rather than an error.
	err = untilSlotReleased(ctx, func() error {
		var endLSN string
		err := s.pool.QueryRow(ctx, `SELECT (pg_replication_slot_advance(slot_name, GREATEST(confirmed_flush_lsn, $2::pg_lsn))).end_lsn::text
			FROM pg_replication_slots WHERE slot_name=$1`, s.slotName, target.String()).Scan(&endLSN)
		if slotHeldByPeer(err) {
			return fmt.Errorf("%w: %w", errSlotHeld, err)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("postgres cdc: advance slot %q to %s: %w", s.slotName, target, err)
	}
	return nil
}

// importSnapshot opens a read-only transaction over the snapshot exported by
// CREATE_REPLICATION_SLOT. The replication connection remains open until this
// transaction is closed, as PostgreSQL requires for exported snapshots.
func (s *Source) importSnapshot(ctx context.Context, id string) (*snapshot, error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire snapshot conn: %w", err)
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		conn.Release()
		return nil, fmt.Errorf("begin snapshot tx: %w", err)
	}
	if _, err := tx.Exec(ctx, "SET TRANSACTION SNAPSHOT "+quoteLiteral(id)); err != nil {
		_ = tx.Rollback(ctx)
		conn.Release()
		return nil, fmt.Errorf("import slot snapshot: %w", err)
	}
	return &snapshot{conn: conn, tx: tx, id: id}, nil
}

// extractInitialSnapshot emits every row for resources that do not yet have a
// durable stream checkpoint. Snapshot rows are inserts, which the CDC merge
// policy applies idempotently on retry. Limit deliberately does not apply: a
// partial snapshot followed by an advanced LSN would permanently skip rows.
func (s *Source) extractInitialSnapshot(ctx context.Context, run *pgCDCRun, tx pgx.Tx, shards []shard) error {
	for _, sh := range shards {
		w, err := run.writer(sh.table)
		if err == nil {
			_, err = s.readBlocks(ctx, w, tx, sh, "", nil, rowmodel.Meta{}, 0)
		}
		if err != nil {
			return fmt.Errorf("postgres cdc: initial snapshot %q: %w", sh.table, err)
		}
	}
	return nil
}

func resourcesWithoutCheckpoints(resources []string, cps map[string]filament.Checkpoint) []string {
	missing := make([]string, 0, len(resources))
	for _, resource := range resources {
		if cps == nil || cps[resource] == nil {
			missing = append(missing, resource)
		}
	}
	return missing
}

func (s *Source) currentLSN(ctx context.Context) (pglogrepl.LSN, error) {
	return currentLSNFrom(ctx, s.pool)
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func currentLSNFrom(ctx context.Context, q rowQuerier) (pglogrepl.LSN, error) {
	var raw string
	if err := q.QueryRow(ctx, "SELECT pg_current_wal_lsn()::text").Scan(&raw); err != nil {
		return 0, fmt.Errorf("postgres cdc: current WAL position: %w", err)
	}
	lsn, err := pglogrepl.ParseLSN(raw)
	if err != nil {
		return 0, fmt.Errorf("postgres cdc: current WAL position %q: %w", raw, err)
	}
	return lsn, nil
}

func startPostgresLSN(cps map[string]filament.Checkpoint) (pglogrepl.LSN, uint64, bool, error) {
	var start pglogrepl.LSN
	var maxSeq uint64
	found := false
	for resource, cp := range cps {
		if cp == nil {
			continue
		}
		raw, seq, ok := checkpoint.ParseStream(cp)
		if !ok {
			return 0, 0, false, fmt.Errorf("postgres cdc: checkpoint for %q is not a stream cursor", resource)
		}
		lsn, err := pglogrepl.ParseLSN(raw)
		if err != nil {
			return 0, 0, false, fmt.Errorf("postgres cdc: checkpoint for %q has invalid LSN %q: %w", resource, raw, err)
		}
		if !found || lsn < start {
			start = lsn
			found = true
		}
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	return start, maxSeq, found, nil
}

func postgresResourceFloors(cps map[string]filament.Checkpoint) (map[string]pglogrepl.LSN, error) {
	floors := make(map[string]pglogrepl.LSN, len(cps))
	for resource, cp := range cps {
		if cp == nil {
			continue
		}
		raw, _, ok := checkpoint.ParseStream(cp)
		if !ok {
			return nil, fmt.Errorf("postgres cdc: checkpoint for %q is not a stream cursor", resource)
		}
		lsn, err := pglogrepl.ParseLSN(raw)
		if err != nil {
			return nil, fmt.Errorf("postgres cdc: checkpoint for %q has invalid LSN %q: %w", resource, raw, err)
		}
		floors[resource] = lsn
	}
	return floors, nil
}

func (r *pgCDCRun) process(data []byte, walStart pglogrepl.LSN) (pglogrepl.LSN, error) {
	msg, err := pglogrepl.Parse(data)
	if err != nil {
		return 0, fmt.Errorf("postgres cdc: decode pgoutput at %s: %w", walStart, err)
	}
	switch msg := msg.(type) {
	case *pglogrepl.BeginMessage:
		r.inTxn = true
	case *pglogrepl.CommitMessage:
		r.inTxn = false
		return msg.TransactionEndLSN, nil
	case *pglogrepl.RelationMessage:
		r.rememberRelation(msg)
	case *pglogrepl.InsertMessage:
		rel, ok := r.relation(msg.RelationID)
		if !ok {
			return 0, nil
		}
		if err := r.push(rel, msg.Tuple, nil, rowmodel.OpInsert, walStart); err != nil {
			return 0, err
		}
	case *pglogrepl.UpdateMessage:
		rel, ok := r.relation(msg.RelationID)
		if !ok {
			return 0, nil
		}
		op := rowmodel.OpUpdate
		if msg.OldTuple != nil {
			// A key change is a delete of the old row and an insert of the new one.
			oldKey, err := rel.keyOf(msg.OldTuple)
			if err != nil {
				return 0, err
			}
			newKey, err := rel.keyOf(msg.NewTuple)
			if err != nil {
				return 0, err
			}
			if !slices.Equal(oldKey, newKey) {
				if err := r.push(rel, msg.OldTuple, nil, rowmodel.OpDelete, walStart); err != nil {
					return 0, err
				}
				op = rowmodel.OpInsert
			}
		}
		if err := r.push(rel, msg.NewTuple, msg.OldTuple, op, walStart); err != nil {
			return 0, err
		}
	case *pglogrepl.DeleteMessage:
		rel, ok := r.relation(msg.RelationID)
		if !ok {
			return 0, nil
		}
		if err := r.push(rel, msg.OldTuple, nil, rowmodel.OpDelete, walStart); err != nil {
			return 0, err
		}
	case *pglogrepl.TruncateMessage:
		for _, id := range msg.RelationIDs {
			if rel, ok := r.relations[id]; ok && rel.tracked {
				return 0, fmt.Errorf("postgres cdc: TRUNCATE of %s.%s cannot be represented as row deletes", rel.message.Namespace, rel.message.RelationName)
			}
		}
	}
	return 0, nil
}

func (r *pgCDCRun) rememberRelation(msg *pglogrepl.RelationMessage) {
	rel := &cdcRelation{message: msg}
	t, tracked := r.tracked[msg.RelationName]
	rel.tracked = tracked && msg.Namespace == r.schema
	if rel.tracked {
		rel.schema = t.schema
		rel.cols = make([]int, len(t.schema.Fields))
		rel.parsers = make([]func(arrowbatch.RowWriter, []byte) error, len(t.schema.Fields))
		for i, f := range t.schema.Fields {
			rel.cols[i] = slices.IndexFunc(msg.Columns, func(c *pglogrepl.RelationMessageColumn) bool { return c.Name == f.Name })
			if rel.cols[i] >= 0 {
				pt, _ := typeFor(msg.Columns[rel.cols[i]].DataType, f)
				rel.parsers[i] = pt.fromText
			}
		}
		for _, pk := range t.pks {
			for i, col := range msg.Columns {
				if col.Name == pk {
					rel.keys = append(rel.keys, i)
					break
				}
			}
		}
		if len(rel.keys) != len(t.pks) {
			rel.tracked = false
		}
	}
	r.relations[msg.RelationID] = rel
}

func (r *pgCDCRun) relation(id uint32) (*cdcRelation, bool) {
	rel, ok := r.relations[id]
	return rel, ok && rel.tracked
}

// push appends one change row: the tuple's columns in schema order (a delete
// carries only its key columns, the rest null), stamped with its operation and
// stream position.
func (r *pgCDCRun) push(rel *cdcRelation, tuple, fallback *pglogrepl.TupleData, op rowmodel.Operation, lsn pglogrepl.LSN) error {
	if !r.pastFloor(rel.message.RelationName, lsn) {
		return nil
	}
	w, err := r.writer(rel.message.RelationName)
	if err != nil {
		return err
	}
	if err := rel.appendTuple(w, tuple, fallback, op); err != nil {
		return err
	}
	r.seq++
	if err := w.EndRow(rowmodel.Meta{Op: op, LSN: lsn.String(), Seq: r.seq}); err != nil {
		return err
	}
	r.emitted++
	if r.limit > 0 && r.emitted >= r.limit {
		r.limited = true
	}
	return nil
}

func (r *pgCDCRun) pastFloor(resource string, lsn pglogrepl.LSN) bool {
	floor, ok := r.floors[resource]
	return !ok || lsn >= floor
}

// pushStreamMarks drains every resource's writer at the cycle's final position,
// so the next cycle resumes from here even for resources that saw no change.
func (r *pgCDCRun) pushStreamMarks(lsn pglogrepl.LSN) error {
	// The final position must outrank every row even when checkpoint folds arrive out of order.
	r.seq++
	for _, resource := range r.resources {
		w, err := r.writer(resource)
		if err != nil {
			return err
		}
		if err := w.Drain(rowmodel.Meta{LSN: lsn.String(), Seq: r.seq}); err != nil {
			return err
		}
	}
	return nil
}

// complete rejects a tuple that does not carry one value per relation column.
func (rel *cdcRelation) complete(tuple *pglogrepl.TupleData) error {
	if tuple == nil || len(tuple.Columns) != len(rel.message.Columns) {
		return fmt.Errorf("postgres cdc: %s.%s tuple has %d columns, want %d", rel.message.Namespace, rel.message.RelationName, tupleColumnCount(tuple), len(rel.message.Columns))
	}
	return nil
}

// keyOf returns the tuple's primary-key values as text.
func (rel *cdcRelation) keyOf(tuple *pglogrepl.TupleData) ([]string, error) {
	if err := rel.complete(tuple); err != nil {
		return nil, err
	}
	key := make([]string, len(rel.keys))
	for n, i := range rel.keys {
		col := tuple.Columns[i]
		if col.DataType != pglogrepl.TupleDataTypeText {
			return nil, fmt.Errorf("postgres cdc: primary-key column %q is not present in tuple", rel.message.Columns[i].Name)
		}
		key[n] = string(col.Data)
	}
	return key, nil
}

// appendTuple appends the tuple's values in schema order. An unchanged TOAST
// column takes its value from fallback (the old tuple); a delete appends only the
// key columns and nulls the rest.
func (rel *cdcRelation) appendTuple(w arrowbatch.RowWriter, tuple, fallback *pglogrepl.TupleData, op rowmodel.Operation) error {
	if err := rel.complete(tuple); err != nil {
		return err
	}
	for f, i := range rel.cols {
		if i < 0 || (op == rowmodel.OpDelete && !slices.Contains(rel.keys, i)) {
			w.Null()
			continue
		}
		col := tuple.Columns[i]
		if col.DataType == pglogrepl.TupleDataTypeToast {
			if fallback == nil || len(fallback.Columns) != len(tuple.Columns) || fallback.Columns[i].DataType == pglogrepl.TupleDataTypeToast {
				return fmt.Errorf("postgres cdc: unchanged TOAST column %q on %s.%s has no old value; set REPLICA IDENTITY FULL", rel.message.Columns[i].Name, rel.message.Namespace, rel.message.RelationName)
			}
			col = fallback.Columns[i]
		}
		switch col.DataType {
		case pglogrepl.TupleDataTypeNull:
			w.Null()
		case pglogrepl.TupleDataTypeText:
			if err := rel.parsers[f](w, col.Data); err != nil {
				return fmt.Errorf("postgres cdc: decode %s.%s column %q: %w", rel.message.Namespace, rel.message.RelationName, rel.message.Columns[i].Name, err)
			}
		case pglogrepl.TupleDataTypeBinary:
			return fmt.Errorf("postgres cdc: column %q: binary tuple data is not supported", rel.message.Columns[i].Name)
		default:
			return fmt.Errorf("postgres cdc: column %q: tuple data type %q is not materialized", rel.message.Columns[i].Name, col.DataType)
		}
	}
	return nil
}

func tupleColumnCount(tuple *pglogrepl.TupleData) int {
	if tuple == nil {
		return 0
	}
	return len(tuple.Columns)
}

func maxLSN(a, b pglogrepl.LSN) pglogrepl.LSN {
	if a > b {
		return a
	}
	return b
}
