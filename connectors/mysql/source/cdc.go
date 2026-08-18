package mysql

// Change-data-capture via the binlog replication protocol. ExtractChanges connects
// as a replica (go-mysql BinlogSyncer), decodes ROW-format events for the requested
// tables, and appends insert/update/delete rows through the same per-type parsers
// as the snapshot reader, so sinks cannot tell the two apart.
//
// A run has catch-up semantics: it captures the server's current binlog position as
// a watermark, streams from the last checkpointed position up to that watermark, and
// returns. Re-requesting the run continues from the persisted cursor, so continuous
// CDC is a re-request loop (the same mechanism as snapshot resume). The cursor is a
// stream checkpoint (checkpoint.ModeStream): a GTID set on a gtid_mode=ON server
// (the default — see cdc_gtid.go), else "file:pos". Every row carries it in
// RowMeta.LSN and draining each writer persists the final watermark even for a run that
// saw no changes — without it, an idle first run would leave no cursor and the next
// run would re-capture a later position, silently skipping the gap between them.
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
	"github.com/galaxy-io/filament/checkpoint"
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
func (s *Source) ExtractChanges(ctx context.Context, sink filament.RecordSink, opts filament.ChangeExtractOpts) error {
	if s.db == nil {
		return fmt.Errorf("mysql source: extract changes before configure")
	}
	if useGTID, err := s.chooseGTID(ctx, opts.Checkpoints); err != nil {
		return err
	} else if useGTID {
		watermark, err := s.gtidExecuted(ctx)
		if err != nil {
			return err
		}
		return s.extractChangesGTID(ctx, sink, opts, watermark)
	}

	watermark, err := s.masterPosition(ctx)
	if err != nil {
		return err
	}
	start, ok := startPosition(opts.Checkpoints)
	if !ok {
		start = watermark // first run: begin at the current tail
	}

	run := newCDCRun(sink, opts.Resources, opts.Limit)

	if posCmp(start, watermark) >= 0 {
		return run.pushStreamMarksLSN(ctx, s, posString(watermark))
	}

	syncer := replication.NewBinlogSyncer(s.binlogConfig())
	defer syncer.Close()
	streamer, err := syncer.StartSync(start)
	if err != nil {
		return fmt.Errorf("mysql cdc: start sync at %s: %w", posString(start), err)
	}

	pos := start

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
				return nil // truncated: no watermark sentinel, resume re-reads from the cursor
			}
		}

		if posCmp(pos, watermark) >= 0 {
			return run.pushStreamMarksLSN(ctx, s, posString(pos))
		}
	}
}

// chooseGTID decides the cursor kind for this run: GTID when the server supports
// it AND no resource carries a legacy file:pos cursor (continuity beats upgrade —
// jumping cursor kinds mid-stream would need a file:pos → GTID translation the
// binlog does not offer).
func (s *Source) chooseGTID(ctx context.Context, cps map[string]filament.Checkpoint) (bool, error) {
	for _, cp := range cps {
		if lsn, _, ok := checkpoint.ParseStream(cp); ok && !strings.HasPrefix(lsn, gtidCursorPrefix) {
			return false, nil
		}
	}
	return s.gtidMode(ctx)
}

type cdcRun struct {
	sink      filament.RecordSink
	resources []string
	tracked   map[string]bool
	tables    map[string]*cdcTable // decoder cache, invalidated on DDL; writers persist
	seq       uint64
	emitted   int
	limit     int
}

// cdcTable is one tracked table's decoder (from information_schema) and the row
// writer its changes append into.
type cdcTable struct {
	dec    *rowDecoder
	writer filament.RowWriter
}

func newCDCRun(sink filament.RecordSink, resources []string, limit int) *cdcRun {
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

func (r *cdcRun) pushRowsEvent(ctx context.Context, s *Source, typ replication.EventType, e *replication.RowsEvent, lsn string) (bool, error) {
	db, table := string(e.Table.Schema), string(e.Table.Table)
	if db != s.database || !r.tracked[table] {
		return false, nil
	}
	t, err := r.table(ctx, s, table, len(e.Table.ColumnType))
	if err != nil {
		return false, err
	}
	n, err := r.pushRows(t, typ, table, e.Rows, lsn)
	if err != nil {
		return false, err
	}
	r.emitted += n
	return r.limit > 0 && r.emitted >= r.limit, nil
}

// pushStreamMarksLSN drains every resource's writer at the final stream
// position, so the cursor persists even when a resource saw no changes this run.
func (r *cdcRun) pushStreamMarksLSN(ctx context.Context, s *Source, lsn string) error {
	for _, resource := range r.resources {
		t, err := r.table(ctx, s, resource, -1)
		if err != nil {
			return err
		}
		if err := t.writer.Drain(filament.RowMeta{LSN: lsn, Seq: r.seq}); err != nil {
			return err
		}
	}
	return nil
}

// pushRows appends one ROW event's rows, stamped with the stream cursor. Updates
// arrive as (before, after) pairs: an unchanged-key update appends OpUpdate with
// the after image; a key-changing update appends OpDelete(before) +
// OpInsert(after) so the sink's merge keeps exactly one row.
func (r *cdcRun) pushRows(t *cdcTable, typ replication.EventType, table string, rows [][]any, lsn string) (int, error) {
	push := func(op filament.Operation, row []any) error {
		if err := t.dec.appendBinlogRow(t.writer, row); err != nil {
			return fmt.Errorf("mysql cdc: %s row: %w", table, err)
		}
		r.seq++
		return t.writer.EndRow(filament.RowMeta{Op: op, LSN: lsn, Seq: r.seq})
	}

	n := 0
	switch {
	case isWriteRows(typ):
		for _, row := range rows {
			if err := push(filament.OpInsert, row); err != nil {
				return n, err
			}
			n++
		}
	case isDeleteRows(typ):
		for _, row := range rows {
			if err := push(filament.OpDelete, row); err != nil {
				return n, err
			}
			n++
		}
	case isUpdateRows(typ):
		for i := 0; i+1 < len(rows); i += 2 {
			before, after := rows[i], rows[i+1]
			if t.dec.keyOf(before) == t.dec.keyOf(after) {
				if err := push(filament.OpUpdate, after); err != nil {
					return n, err
				}
				n++
				continue
			}
			if err := push(filament.OpDelete, before); err != nil {
				return n, err
			}
			if err := push(filament.OpInsert, after); err != nil {
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
func (d *rowDecoder) appendBinlogRow(w filament.RowWriter, row []any) error {
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

// startPosition picks the oldest checkpointed position across the run's resources —
// the safe restart point; re-delivered events are absorbed by the idempotent merge.
func startPosition(cps map[string]filament.Checkpoint) (gomysql.Position, bool) {
	var out gomysql.Position
	found := false
	for _, cp := range cps {
		lsn, _, ok := checkpoint.ParseStream(cp)
		if !ok {
			continue
		}
		pos, err := parsePos(lsn)
		if err != nil {
			continue
		}
		if !found || posCmp(pos, out) < 0 {
			out = pos
			found = true
		}
	}
	return out, found
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
		UseDecimal: true, // decimals as exact decimal values, not lossy float64
		// Timestamps render in UTC, matching the query path's pinned session zone.
		TimestampStringLocation: time.UTC,
	}
}
