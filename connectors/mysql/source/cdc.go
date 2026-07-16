package mysql

// Change-data-capture via the binlog replication protocol. ExtractChanges connects
// as a replica (go-mysql BinlogSyncer), decodes ROW-format events for the requested
// tables, and emits insert/update/delete records encoded in the same JSON shape as
// the snapshot reader, so sinks cannot tell the two apart.
//
// A run has catch-up semantics: it captures the server's current binlog position as
// a watermark, streams from the last checkpointed position up to that watermark, and
// returns. Re-requesting the run continues from the persisted cursor, so continuous
// CDC is a re-request loop (the same mechanism as snapshot resume). The cursor is a
// stream checkpoint (checkpoint.ModeStream): a GTID set on a gtid_mode=ON server
// (the default — see cdc_gtid.go), else "file:pos". Every record carries it in
// Meta.LSN and a Drained sentinel persists the final watermark even for a run that
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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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
		return run.pushStreamMarksLSN(posString(watermark))
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
			return run.pushStreamMarksLSN(posString(pos))
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
	tables    map[string][]column // schema cache, invalidated on DDL
	seq       uint64
	emitted   int
	limit     int
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
		tables:    map[string][]column{},
		limit:     limit,
	}
}

func (r *cdcRun) clearSchema() {
	clear(r.tables)
}

func (r *cdcRun) pushRowsEvent(ctx context.Context, s *Source, typ replication.EventType, e *replication.RowsEvent, lsn string) (bool, error) {
	db, table := string(e.Table.Schema), string(e.Table.Table)
	if db != s.database || !r.tracked[table] {
		return false, nil
	}
	cols, err := s.cachedColumns(ctx, r.tables, table, len(e.Table.ColumnType))
	if err != nil {
		return false, err
	}
	n, err := s.pushRowsEventLSN(r.sink, typ, table, cols, e.Rows, lsn, &r.seq)
	if err != nil {
		return false, err
	}
	r.emitted += n
	return r.limit > 0 && r.emitted >= r.limit, nil
}

// pushStreamMarksLSN emits one Drained sentinel per resource carrying the final
// stream position, so the cursor persists even when a resource saw no changes
// this run.
func (r *cdcRun) pushStreamMarksLSN(lsn string) error {
	for _, resource := range r.resources {
		rec := filament.Record{Resource: resource, Drained: true}
		rec.Meta.LSN = lsn
		rec.Meta.Seq = r.seq
		if err := r.sink.Push(rec); err != nil {
			return err
		}
	}
	return nil
}

// pushRowsEventLSN decodes one ROW event into records stamped with the given
// stream cursor. Updates arrive as (before, after) pairs: an unchanged-key update
// emits OpUpdate with the after image; a key-changing update emits
// OpDelete(before) + OpInsert(after) so the sink's merge keeps exactly one row.
func (s *Source) pushRowsEventLSN(sink filament.RecordSink, typ replication.EventType, table string, cols []column, rows [][]any, lsn string, seq *uint64) (int, error) {
	push := func(op filament.Operation, row []any) error {
		data, err := encodeRowJSON(cols, row)
		if err != nil {
			return fmt.Errorf("mysql cdc: encode %s row: %w", table, err)
		}
		*seq++
		rec := filament.Record{
			Resource: table,
			ID:       rowID(cols, row),
			Op:       op,
			Data:     data,
		}
		rec.Meta.LSN = lsn
		rec.Meta.Seq = *seq
		return sink.Push(rec)
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
			if rowID(cols, before) == rowID(cols, after) {
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

// cachedColumns returns table's ordinal column list, reading through to
// information_schema on a cache miss. The binlog carries column count but not names
// (binlog_row_metadata defaults to MINIMAL), so a count mismatch means the catalog
// and the event disagree — DDL landed between them — and mapping by position would
// silently put values in wrong columns; fail instead.
func (s *Source) cachedColumns(ctx context.Context, cache map[string][]column, table string, want int) ([]column, error) {
	cols, ok := cache[table]
	if !ok {
		var err error
		cols, _, err = s.tableMeta(ctx, table)
		if err != nil {
			return nil, fmt.Errorf("mysql cdc: columns %q: %w", table, err)
		}
		cache[table] = cols
	}
	if len(cols) != want {
		return nil, fmt.Errorf("mysql cdc: %q has %d columns in the catalog but %d in the binlog event (concurrent DDL?); re-snapshot the table", table, len(cols), want)
	}
	return cols, nil
}

// rowID renders the primary-key tuple as the record id, columns joined with unit
// separator 0x1F — the same convention as the snapshot reader's CONCAT_WS(CHAR(31)).
// A keyless table falls back to the whole row (such tables cannot be merged anyway;
// policy validation rejects them for CDC ingestion).
func rowID(cols []column, row []any) string {
	var parts []string
	for i, c := range cols {
		if c.pkOrder > 0 && i < len(row) {
			parts = append(parts, valueText(row[i]))
		}
	}
	if len(parts) == 0 {
		for i := range row {
			parts = append(parts, valueText(row[i]))
		}
	}
	return strings.Join(parts, "\x1f")
}

// valueText renders a binlog value as plain text (for record ids).
func valueText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(t)
	}
}

// encodeRowJSON renders one decoded binlog row as a JSON object in column order,
// matching the snapshot reader's JSON_OBJECT output: binary columns as base64
// (TO_BASE64 convention, reversed by the typed sink's FROM_BASE64), JSON columns
// embedded raw, everything else as JSON scalars.
func encodeRowJSON(cols []column, row []any) ([]byte, error) {
	if len(row) < len(cols) {
		return nil, fmt.Errorf("row has %d values for %d columns", len(row), len(cols))
	}
	buf := make([]byte, 0, 64*len(cols))
	buf = append(buf, '{')
	for i, c := range cols {
		if i > 0 {
			buf = append(buf, ',')
		}
		key, err := json.Marshal(c.name)
		if err != nil {
			return nil, err
		}
		buf = append(buf, key...)
		buf = append(buf, ':')
		buf, err = appendJSONValue(buf, row[i], c)
		if err != nil {
			return nil, fmt.Errorf("column %q: %w", c.name, err)
		}
	}
	return append(buf, '}'), nil
}

func appendJSONValue(buf []byte, v any, c column) ([]byte, error) {
	if v == nil {
		return append(buf, "null"...), nil
	}
	switch t := v.(type) {
	case int8, int16, int32, int64, int, uint8, uint16, uint32, uint64, uint:
		return append(buf, fmt.Sprint(t)...), nil
	case float32:
		return strconv.AppendFloat(buf, float64(t), 'g', -1, 32), nil
	case float64:
		return strconv.AppendFloat(buf, t, 'g', -1, 64), nil
	case bool:
		return strconv.AppendBool(buf, t), nil
	}

	raw := valueBytes(v)
	switch {
	case strings.EqualFold(c.dataType, "json"):
		if json.Valid(raw) {
			return append(buf, raw...), nil
		}
		return nil, fmt.Errorf("invalid JSON payload")
	case isBinaryType(c.dataType):
		buf = append(buf, '"')
		buf = base64.StdEncoding.AppendEncode(buf, raw)
		return append(buf, '"'), nil
	default:
		quoted, err := json.Marshal(string(raw))
		if err != nil {
			return nil, err
		}
		return append(buf, quoted...), nil
	}
}

// valueBytes normalizes a binlog value's textual forms ([]byte, string, or a
// Stringer such as a decimal) to bytes.
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

// ── binlog position plumbing ─────────────────────────────────────────────────

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
	}
}
