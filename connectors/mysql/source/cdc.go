package mysql

// Change-data-capture via the binlog replication protocol. ExtractChanges connects
// as a replica (go-mysql BinlogSyncer), decodes ROW-format events for the requested
// tables, and appends insert/update/delete rows through the same per-type parsers
// as the snapshot reader, so sinks cannot tell the two apart.
//
// A resource without a stream checkpoint is first read in full through one
// consistent snapshot (see snapshotBootstrap). The binlog position captured just
// before that snapshot opens becomes the resource's floor: the change stream
// replays everything after it, so the baseline and the stream share one point.
//
// A run has catch-up semantics: it captures the server's current binlog position as
// a watermark, streams from the oldest resource floor up to that watermark, and
// returns. Re-requesting the run continues from the persisted cursor, so continuous
// CDC is a re-request loop (the same mechanism as snapshot resume). The cursor is a
// stream checkpoint (checkpoint.ModeStream): a GTID set on a gtid_mode=ON server
// (the default — see cdc_gtid.go), else "file:pos". Every row carries it in
// its stream cursor and draining each writer persists the final watermark even for a run that
// saw no changes — without it, an idle run would leave no cursor and the next
// run would bootstrap the resource again.
//
// Requirements on the server: ROW binlog format (the 8.0+ default), binlog retention
// covering the resume window, and REPLICATION SLAVE/CLIENT privileges. Column names
// are not in the binlog (binlog_row_metadata=MINIMAL default), so each table's
// column list is read from information_schema and invalidated on any DDL event; a
// column-count mismatch mid-stream fails the run rather than mis-mapping values.

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	gomysql "github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/replication"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/rowmodel"
)

// defaultServerID identifies this client in the replica topology; it must differ
// from the server's and any real replica's server_id. Override with "server_id".
const defaultServerID = 62347

var _ filament.ChangeSource = (*Source)(nil)

// ExtractChanges streams binlog row events from the checkpointed position to the
// position the server reports at call time, pushing one record per changed row.
// The cursor kind is chosen automatically: GTID when the server runs with
// gtid_mode=ON (survives replica promotion — the executed-transaction set is
// global to the topology, binlog file offsets are not), file:pos otherwise.
// Existing file:pos checkpoints keep the file:pos path even on a GTID server,
// so an in-flight stream never jumps cursors mid-run; a new run id migrates.
//
// Resources without a checkpoint are bootstrapped first: read in full through a
// consistent snapshot in this same cycle, then streamed from the position captured
// before that snapshot opened. Every resource therefore has a floor, and a row
// event below its resource's floor is skipped rather than re-delivered.
func (s *Source) ExtractChanges(ctx context.Context, sink arrowbatch.Inlet, opts filament.ChangeExtractOpts) error {
	if s.db == nil {
		return fmt.Errorf("mysql source: extract changes before configure")
	}
	if len(opts.Resources) == 0 {
		return nil
	}
	run := newCDCRun(sink, opts.Resources, opts.Limit)
	bootstrap := resourcesWithoutCheckpoints(opts.Resources, opts.Checkpoints)
	if err := run.prepare(ctx, s, bootstrap); err != nil {
		return err
	}

	if useGTID, err := s.chooseGTID(ctx, opts.Checkpoints); err != nil {
		return err
	} else if useGTID {
		return s.extractChangesGTID(ctx, run, opts, bootstrap)
	}

	floors, err := positionFloors(opts.Checkpoints)
	if err != nil {
		return err
	}
	if len(bootstrap) > 0 {
		floor, err := s.masterPosition(ctx)
		if err != nil {
			return err
		}
		if err := s.snapshotBootstrap(ctx, run, bootstrap); err != nil {
			return err
		}
		for _, resource := range bootstrap {
			floors[resource] = floor
		}
	}
	watermark, err := s.masterPosition(ctx)
	if err != nil {
		return err
	}
	start := oldestPosition(floors)
	// A resource's mark never falls below its floor: a truncated cycle can stop
	// the stream short of a newly bootstrapped resource's snapshot point, and
	// marking it there would replay changes the snapshot already included.
	mark := func(at gomysql.Position) func(string) (string, error) {
		return func(resource string) (string, error) { return posString(laterPosition(at, floors[resource])), nil }
	}

	if posCmp(start, watermark) >= 0 {
		return run.pushStreamMarks(ctx, s, mark(watermark))
	}

	syncer := replication.NewBinlogSyncer(s.binlogConfig())
	defer syncer.Close()
	streamer, err := syncer.StartSync(start)
	if err != nil {
		return fmt.Errorf("mysql cdc: start sync at %s: %w", posString(start), err)
	}

	pos := start
	// An event ending at or before a resource's floor was delivered by the
	// bootstrap snapshot or a previous cycle.
	run.skip = func(resource string) bool { return posCmp(pos, floors[resource]) <= 0 }

	for {
		ev, err := streamer.GetEvent(ctx)
		if err != nil {
			return fmt.Errorf("mysql cdc: read event at %s: %w", posString(pos), err)
		}
		if rot, ok := ev.Event.(*replication.RotateEvent); ok {
			pos = gomysql.Position{Name: string(rot.NextLogName), Pos: uint32(rot.Position)} //nolint:gosec // binlog positions fit uint32 by protocol
			continue
		}
		if ev.Header.LogPos > 0 {
			pos.Pos = ev.Header.LogPos
		}

		switch e := ev.Event.(type) {
		case *replication.QueryEvent:
			// DDL (or any statement event): drop the schema cache so the next row
			// event re-reads column lists that may have changed.
			run.clearSchema()
		case *replication.RowsEvent:
			limited, err := run.pushRowsEvent(ctx, s, ev.Header.EventType, e, posString(pos))
			if err != nil {
				return err
			}
			if limited {
				// Truncated: mark every resource at the current position so a
				// resource that saw no rows this cycle (a just-bootstrapped one in
				// particular) still resumes from here rather than starting over.
				return run.pushStreamMarks(ctx, s, mark(pos))
			}
		}

		if posCmp(pos, watermark) >= 0 {
			return run.pushStreamMarks(ctx, s, mark(pos))
		}
	}
}

// chooseGTID decides the cursor kind for this run: GTID when the server supports
// it AND no resource carries a legacy file:pos cursor (continuity beats upgrade —
// jumping cursor kinds mid-stream would need a file:pos → GTID translation the
// binlog does not offer).
func (s *Source) chooseGTID(ctx context.Context, cps map[string]filament.Checkpoint) (bool, error) {
	for _, cp := range cps {
		if cursor, _, ok := checkpoint.ParseStream(cp); ok && !strings.HasPrefix(cursor, gtidCursorPrefix) {
			return false, nil
		}
	}
	return s.gtidMode(ctx)
}

type cdcRun struct {
	sink      arrowbatch.Inlet
	resources []string
	tracked   map[string]bool
	tables    map[string]*cdcTable // decoder cache, invalidated on DDL; writers persist
	// skip reports whether the row event being decoded lies at or below the
	// resource's floor and was therefore already delivered; set by each stream
	// loop over its own cursor kind.
	skip    func(resource string) bool
	seq     uint64
	emitted int
	limit   int
}

// cdcTable is one tracked table's decoder (from information_schema) and the row
// writer its changes append into.
type cdcTable struct {
	dec    *rowDecoder
	writer arrowbatch.RowWriter
}

func newCDCRun(sink arrowbatch.Inlet, resources []string, limit int) *cdcRun {
	tracked := make(map[string]bool, len(resources))
	for _, r := range resources {
		tracked[r] = true
	}
	return &cdcRun{
		sink:      sink,
		resources: resources,
		tracked:   tracked,
		tables:    map[string]*cdcTable{},
		limit:     limit,
	}
}

// clearSchema drops the cached decoders after a DDL event; the writers stay,
// so a table whose columns actually changed fails on the next event rather
// than mis-mapping values.
func (r *cdcRun) clearSchema() {
	for _, t := range r.tables {
		t.dec = nil
	}
}

func (r *cdcRun) pushRowsEvent(ctx context.Context, s *Source, typ replication.EventType, e *replication.RowsEvent, cursor string) (bool, error) {
	db, table := string(e.Table.Schema), string(e.Table.Table)
	if db != s.database || !r.tracked[table] || r.skip(table) {
		return false, nil
	}
	t, err := r.table(ctx, s, table, len(e.Table.ColumnType))
	if err != nil {
		return false, err
	}
	n, err := r.pushRows(t, typ, table, e.Rows, cursor)
	if err != nil {
		return false, err
	}
	r.emitted += n
	return r.limit > 0 && r.emitted >= r.limit, nil
}

// pushStreamMarks drains every resource's writer at the cursor the callback
// reports for it, so the cursor persists even when a resource saw no changes
// this run. The callback lets each mode keep a resource's mark at or above its
// own floor.
func (r *cdcRun) pushStreamMarks(ctx context.Context, s *Source, cursor func(resource string) (string, error)) error {
	for _, resource := range r.resources {
		t, err := r.table(ctx, s, resource, -1)
		if err != nil {
			return err
		}
		mark, err := cursor(resource)
		if err != nil {
			return err
		}
		if err := t.writer.Drain(rowmodel.Meta{LSN: mark, Seq: r.seq}); err != nil {
			return err
		}
	}
	return nil
}

// pushRows appends one ROW event's rows, stamped with the stream cursor. Updates
// arrive as (before, after) pairs: an unchanged-key update appends OpUpdate with
// the after image; a key-changing update appends OpDelete(before) +
// OpInsert(after) so the sink's merge keeps exactly one row.
func (r *cdcRun) pushRows(t *cdcTable, typ replication.EventType, table string, rows [][]any, cursor string) (int, error) {
	push := func(op rowmodel.Operation, row []any) error {
		if err := t.dec.appendBinlogRow(t.writer, row); err != nil {
			return fmt.Errorf("mysql cdc: %s row: %w", table, err)
		}
		r.seq++
		return t.writer.EndRow(rowmodel.Meta{Op: op, LSN: cursor, Seq: r.seq})
	}

	n := 0
	switch {
	case isWriteRows(typ):
		for _, row := range rows {
			if err := push(rowmodel.OpInsert, row); err != nil {
				return n, err
			}
			n++
		}
	case isDeleteRows(typ):
		for _, row := range rows {
			if err := push(rowmodel.OpDelete, row); err != nil {
				return n, err
			}
			n++
		}
	case isUpdateRows(typ):
		for i := 0; i+1 < len(rows); i += 2 {
			before, after := rows[i], rows[i+1]
			if t.dec.keyOf(before) == t.dec.keyOf(after) {
				if err := push(rowmodel.OpUpdate, after); err != nil {
					return n, err
				}
				n++
				continue
			}
			if err := push(rowmodel.OpDelete, before); err != nil {
				return n, err
			}
			if err := push(rowmodel.OpInsert, after); err != nil {
				return n + 1, err
			}
			n += 2
		}
	}
	return n, nil
}

func isWriteRows(t replication.EventType) bool {
	return t == replication.WRITE_ROWS_EVENTv0 || t == replication.WRITE_ROWS_EVENTv1 || t == replication.WRITE_ROWS_EVENTv2
}

func isDeleteRows(t replication.EventType) bool {
	return t == replication.DELETE_ROWS_EVENTv0 || t == replication.DELETE_ROWS_EVENTv1 || t == replication.DELETE_ROWS_EVENTv2
}

func isUpdateRows(t replication.EventType) bool {
	return t == replication.UPDATE_ROWS_EVENTv0 || t == replication.UPDATE_ROWS_EVENTv1 || t == replication.UPDATE_ROWS_EVENTv2
}

// table returns the tracked table's decoder and writer, reading the decoder
// through to information_schema on a cache miss and opening the writer on first
// use. want is the binlog event's column count (-1 for a stream mark): the binlog
// carries column count but not names (binlog_row_metadata defaults to MINIMAL), so
// a mismatch means the catalog and the event disagree — DDL landed between them —
// and mapping by position would silently put values in wrong columns; fail instead.
func (r *cdcRun) table(ctx context.Context, s *Source, table string, want int) (*cdcTable, error) {
	t := r.tables[table]
	if t == nil {
		t = &cdcTable{}
		r.tables[table] = t
	}
	if t.dec == nil {
		dec, err := s.decoderFor(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("mysql cdc: columns %q: %w", table, err)
		}
		t.dec = dec
	}
	if want >= 0 && len(t.dec.types) != want {
		return nil, fmt.Errorf("mysql cdc: %q has %d columns in the catalog but %d in the binlog event (concurrent DDL?); re-snapshot the table", table, len(t.dec.types), want)
	}
	if t.writer == nil {
		w, err := r.sink.Builder(table, 0, t.dec.schema)
		if err != nil {
			return nil, err
		}
		t.writer = w
	}
	return t, nil
}

// appendBinlogRow appends one decoded binlog row in column order. Every value
// is taken in its text form ([]byte, string, or a Stringer such as a decimal;
// numbers rendered), the same form the query path parses; binary columns are
// their raw bytes.
func (d *rowDecoder) appendBinlogRow(w arrowbatch.RowWriter, row []any) error {
	if len(row) < len(d.types) {
		return fmt.Errorf("row has %d values for %d columns", len(row), len(d.types))
	}
	for i, t := range d.types {
		if row[i] == nil {
			w.Null()
			continue
		}
		if err := t.parse(w, valueBytes(row[i])); err != nil {
			return fmt.Errorf("column %q: %w", d.schema.Fields[i].Name, err)
		}
	}
	return nil
}

// keyOf renders the primary-key tuple, columns joined with 0x1F, to compare an
// update's before and after images.
func (d *rowDecoder) keyOf(row []any) string {
	var b strings.Builder
	for i, pk := range d.pks {
		if i > 0 {
			b.WriteByte(0x1f)
		}
		for c, f := range d.schema.Fields {
			if f.Name == pk.name && c < len(row) {
				b.Write(valueBytes(row[c]))
			}
		}
	}
	return b.String()
}

// valueBytes normalizes a binlog value's textual forms ([]byte, string, or a
// Stringer such as a decimal) to bytes; numbers are rendered.
func valueBytes(v any) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case string:
		return []byte(t)
	case fmt.Stringer:
		return []byte(t.String())
	default:
		return []byte(fmt.Sprint(t))
	}
}

// masterPosition reads the server's current binlog write position. MySQL 8.2
// renamed the statement; try the new spelling first.
func (s *Source) masterPosition(ctx context.Context) (gomysql.Position, error) {
	for _, stmt := range []string{"SHOW BINARY LOG STATUS", "SHOW MASTER STATUS"} {
		rows, err := s.db.QueryContext(ctx, stmt)
		if err != nil {
			continue
		}
		pos, err := scanPosition(rows)
		_ = rows.Close()
		if err != nil {
			return gomysql.Position{}, err
		}
		return pos, nil
	}
	return gomysql.Position{}, fmt.Errorf("mysql cdc: cannot read binlog position (is log_bin enabled and REPLICATION CLIENT granted?)")
}

// scanPosition pulls (file, position) out of a SHOW … STATUS result, tolerating the
// varying trailing columns across server versions.
func scanPosition(rows interface {
	Columns() ([]string, error)
	Next() bool
	Scan(...any) error
	Err() error
},
) (gomysql.Position, error) {
	colNames, err := rows.Columns()
	if err != nil {
		return gomysql.Position{}, err
	}
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return gomysql.Position{}, err
		}
		return gomysql.Position{}, fmt.Errorf("mysql cdc: binlog status returned no rows (log_bin disabled?)")
	}
	var file string
	var pos uint32
	dest := make([]any, len(colNames))
	var discard any
	for i := range dest {
		dest[i] = &discard
	}
	dest[0], dest[1] = &file, &pos
	if err := rows.Scan(dest...); err != nil {
		return gomysql.Position{}, err
	}
	return gomysql.Position{Name: file, Pos: pos}, nil
}

// resourcesWithoutCheckpoints lists the resources that have never completed a
// bootstrap: no stream cursor was persisted for them.
func resourcesWithoutCheckpoints(resources []string, cps map[string]filament.Checkpoint) []string {
	missing := make([]string, 0, len(resources))
	for _, resource := range resources {
		if cps == nil || cps[resource] == nil {
			missing = append(missing, resource)
		}
	}
	return missing
}

// prepare resolves the decoder and writer of every resource the bootstrap will
// read. It runs before the snapshot connection is taken, so the pool stays
// usable with max_conns=1 while the snapshot transaction is held.
func (r *cdcRun) prepare(ctx context.Context, s *Source, resources []string) error {
	for _, resource := range resources {
		if _, err := r.table(ctx, s, resource, -1); err != nil {
			return err
		}
	}
	return nil
}

// snapshotBootstrap reads each resource in full, sequentially, through one
// consistent-snapshot transaction, appending into the run's CDC writers. The
// caller captures the stream floor before this opens the snapshot and the
// watermark after it returns, so nothing committed during the read is lost:
// it is either in the snapshot or replayed by the stream. Row limits do not
// apply — a partial baseline is worse than a long first cycle.
func (s *Source) snapshotBootstrap(ctx context.Context, run *cdcRun, resources []string) error {
	return s.withSnapshotTx(ctx, func(ctx context.Context, q querier) error {
		for _, resource := range resources {
			t := run.tables[resource]
			w := snapshotWriter{t.writer}
			qualified := quoteIdent(s.database) + "." + quoteIdent(resource)
			var err error
			if len(t.dec.pks) == 0 {
				err = s.extractKeyless(ctx, w, q, resource, qualified, t.dec, 0)
			} else {
				var shards []keyShard
				shards, err = keyShardsFrom(resource, qualified, t.dec, keysetPlan{Shards: []checkpoint.KeysetShard{{}}})
				if err == nil {
					err = s.extractKeysetShard(ctx, w, q, shards[0], 0)
				}
			}
			if err != nil {
				return fmt.Errorf("mysql cdc: initial snapshot %q: %w", resource, err)
			}
		}
		return nil
	})
}

// snapshotWriter feeds bootstrap rows into a CDC writer without the keyset
// cursor the full reader stamps on each row: a CDC resource resumes from its
// stream mark, never from a key.
type snapshotWriter struct{ arrowbatch.RowWriter }

func (w snapshotWriter) EndRow(rowmodel.Meta) error { return w.RowWriter.EndRow(rowmodel.Meta{}) }

// positionFloors decodes each resource's checkpointed file:pos cursor. A
// resource's floor is where its stream resumes; events at or below it were
// delivered by an earlier cycle.
func positionFloors(cps map[string]filament.Checkpoint) (map[string]gomysql.Position, error) {
	floors := make(map[string]gomysql.Position, len(cps))
	for resource, cp := range cps {
		if cp == nil {
			continue
		}
		cursor, _, ok := checkpoint.ParseStream(cp)
		if !ok {
			return nil, fmt.Errorf("mysql cdc: checkpoint for %q is not a stream cursor", resource)
		}
		pos, err := parsePos(cursor)
		if err != nil {
			return nil, fmt.Errorf("mysql cdc: checkpoint for %q: %w", resource, err)
		}
		floors[resource] = pos
	}
	return floors, nil
}

// oldestPosition picks the stream start: the lowest floor across the run's
// resources. Events between it and a newer floor are skipped per resource.
func oldestPosition(floors map[string]gomysql.Position) gomysql.Position {
	var out gomysql.Position
	found := false
	for _, pos := range floors {
		if !found || posCmp(pos, out) < 0 {
			out = pos
			found = true
		}
	}
	return out
}

// laterPosition returns the later of two positions.
func laterPosition(a, b gomysql.Position) gomysql.Position {
	if posCmp(b, a) > 0 {
		return b
	}
	return a
}

func posString(p gomysql.Position) string {
	return p.Name + ":" + strconv.FormatUint(uint64(p.Pos), 10)
}

func parsePos(s string) (gomysql.Position, error) {
	name, posStr, ok := strings.Cut(s, ":")
	if !ok {
		return gomysql.Position{}, fmt.Errorf("bad binlog position %q", s)
	}
	n, err := strconv.ParseUint(posStr, 10, 32)
	if err != nil {
		return gomysql.Position{}, fmt.Errorf("bad binlog position %q: %w", s, err)
	}
	return gomysql.Position{Name: name, Pos: uint32(n)}, nil
}

// posCmp orders binlog positions: by file (the numeric suffix increases across
// rotations, and the names are zero-padded so lexical order matches), then offset.
func posCmp(a, b gomysql.Position) int {
	if a.Name != b.Name {
		return strings.Compare(a.Name, b.Name)
	}
	switch {
	case a.Pos < b.Pos:
		return -1
	case a.Pos > b.Pos:
		return 1
	default:
		return 0
	}
}

// binlogConfig builds the replication client config from the connection settings
// captured at Configure.
func (s *Source) binlogConfig() replication.BinlogSyncerConfig {
	return replication.BinlogSyncerConfig{
		ServerID:   s.serverID,
		Flavor:     "mysql",
		Host:       s.binlogHost,
		Port:       s.binlogPort,
		User:       s.binlogUser,
		Password:   s.binlogPass,
		TLSConfig:  s.binlogTLS,
		UseDecimal: true, // decimals as exact decimal values, not lossy float64
		// Timestamps render in UTC, matching the query path's pinned session zone.
		TimestampStringLocation: time.UTC,
	}
}
