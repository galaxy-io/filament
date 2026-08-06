package postgres

// PostgreSQL change-data-capture uses a persistent logical replication slot and
// the built-in pgoutput plugin. Each extraction is a bounded catch-up cycle: it
// captures pg_current_wal_lsn(), replays committed changes from the oldest
// resource checkpoint through that watermark, emits a stream marker for every
// resource, and returns. The next engine cycle resumes from those markers.

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

const standbyStatusInterval = 10 * time.Second

var replicationNameRE = regexp.MustCompile(`^[a-z_][a-z0-9_]{0,62}$`)

func validReplicationName(name string) bool { return replicationNameRE.MatchString(name) }

type cdcRelation struct {
	message *pglogrepl.RelationMessage
	tracked bool
	keys    []int
}

type pgCDCRun struct {
	sink      filament.RecordSink
	schema    string
	resources []string
	tracked   map[string][]string
	relations map[uint32]*cdcRelation
	seq       uint64
	emitted   int
	limit     int
	limited   bool
	inTxn     bool
	lastLSN   pglogrepl.LSN
}

// ExtractChanges replays pgoutput row changes for the selected tables. The
// publication is created/expanded by default; set manage_publication=false when
// database administration owns it instead. A slot is never dropped automatically:
// its stable identity is what makes a later catch-up lossless.
func (s *Source) ExtractChanges(ctx context.Context, sink filament.RecordSink, opts filament.ChangeExtractOpts) error {
	if s.pool == nil || s.dsn == "" {
		return fmt.Errorf("postgres source: extract changes before configure")
	}
	if len(opts.Resources) == 0 {
		return nil
	}
	tracked := make(map[string][]string, len(opts.Resources))
	for _, resource := range opts.Resources {
		pks, err := s.lookupPrimaryKey(ctx, s.schema, resource)
		if err != nil {
			return fmt.Errorf("postgres cdc: primary key %q: %w", resource, err)
		}
		if len(pks) == 0 {
			return fmt.Errorf("postgres cdc: resource %q has no primary key", resource)
		}
		tracked[resource] = pkNames(pks)
	}
	if err := s.ensurePublication(ctx, opts.Resources); err != nil {
		return err
	}

	repl, err := s.replicationConn(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = repl.Close(context.WithoutCancel(ctx)) }()

	slotStart, err := s.ensureReplicationSlot(ctx, repl)
	if err != nil {
		return err
	}
	watermark, err := s.currentLSN(ctx)
	if err != nil {
		return err
	}
	start, seq, haveCheckpoint, err := startPostgresLSN(opts.Checkpoints)
	if err != nil {
		return err
	}
	if !haveCheckpoint {
		start = slotStart
	} else if start < slotStart {
		return fmt.Errorf("postgres cdc: checkpoint %s is older than slot %q confirmed position %s; WAL was consumed by another client", start, s.slotName, slotStart)
	}
	if start > watermark {
		return fmt.Errorf("postgres cdc: checkpoint %s is ahead of server watermark %s (timeline changed?)", start, watermark)
	}

	if err := pglogrepl.StartReplication(ctx, repl, s.slotName, start, pglogrepl.StartReplicationOptions{
		PluginArgs: []string{"proto_version '1'", "publication_names '" + s.publication + "'"},
	}); err != nil {
		return fmt.Errorf("postgres cdc: start slot %q at %s: %w", s.slotName, start, err)
	}
	// Only acknowledge the cursor which was durable before this extraction began.
	// The final watermark is acknowledged by the next cycle, after the sink and
	// tracker have committed it.
	if err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{WALWritePosition: start}); err != nil {
		return fmt.Errorf("postgres cdc: acknowledge start %s: %w", start, err)
	}

	run := &pgCDCRun{
		sink: sink, schema: s.schema, resources: opts.Resources, tracked: tracked,
		relations: map[uint32]*cdcRelation{}, seq: seq, limit: opts.Limit,
	}
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
				if err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{WALWritePosition: start}); err != nil {
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
					if err := pglogrepl.SendStandbyStatusUpdate(ctx, repl, pglogrepl.StandbyStatusUpdate{WALWritePosition: start}); err != nil {
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
	qualified := make([]string, len(resources))
	for i, resource := range resources {
		qualified[i] = pgx.Identifier{s.schema, resource}.Sanitize()
	}
	if !exists {
		if !s.managePublication {
			return fmt.Errorf("postgres cdc: publication %q does not exist", s.publication)
		}
		stmt := "CREATE PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " FOR TABLE " + strings.Join(qualified, ", ") + " WITH (publish = 'insert, update, delete')"
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

	rows, err := s.pool.Query(ctx, `SELECT tablename FROM pg_publication_tables WHERE pubname=$1 AND schemaname=$2`, s.publication, s.schema)
	if err != nil {
		return fmt.Errorf("postgres cdc: publication tables: %w", err)
	}
	published := map[string]bool{}
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return err
		}
		published[table] = true
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return fmt.Errorf("postgres cdc: publication tables: %w", err)
	}
	var missing []string
	for _, resource := range resources {
		if !published[resource] {
			missing = append(missing, pgx.Identifier{s.schema, resource}.Sanitize())
		}
	}
	if len(missing) == 0 {
		return nil
	}
	if !s.managePublication {
		return fmt.Errorf("postgres cdc: publication %q is missing selected tables: %s", s.publication, strings.Join(missing, ", "))
	}
	stmt := "ALTER PUBLICATION " + pgx.Identifier{s.publication}.Sanitize() + " ADD TABLE " + strings.Join(missing, ", ")
	if _, err := s.pool.Exec(ctx, stmt); err != nil {
		return fmt.Errorf("postgres cdc: add publication tables: %w", err)
	}
	return nil
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
	conn, err := pgconn.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres cdc: connect replication protocol: %w", err)
	}
	return conn, nil
}

func (s *Source) ensureReplicationSlot(ctx context.Context, conn *pgconn.PgConn) (pglogrepl.LSN, error) {
	var plugin, database string
	var confirmed, restart *string
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT plugin, database, active, confirmed_flush_lsn::text, restart_lsn::text
		FROM pg_replication_slots WHERE slot_name=$1`, s.slotName).Scan(&plugin, &database, &active, &confirmed, &restart)
	if err != nil && err != pgx.ErrNoRows {
		return 0, fmt.Errorf("postgres cdc: inspect slot %q: %w", s.slotName, err)
	}
	if err == pgx.ErrNoRows {
		created, err := pglogrepl.CreateReplicationSlot(ctx, conn, s.slotName, "pgoutput", pglogrepl.CreateReplicationSlotOptions{})
		if err != nil {
			return 0, fmt.Errorf("postgres cdc: create slot %q: %w", s.slotName, err)
		}
		lsn, err := pglogrepl.ParseLSN(created.ConsistentPoint)
		if err != nil {
			return 0, fmt.Errorf("postgres cdc: slot consistent point %q: %w", created.ConsistentPoint, err)
		}
		return lsn, nil
	}
	if plugin != "pgoutput" {
		return 0, fmt.Errorf("postgres cdc: slot %q uses plugin %q, want pgoutput", s.slotName, plugin)
	}
	var currentDB string
	if err := s.pool.QueryRow(ctx, "SELECT current_database()").Scan(&currentDB); err != nil {
		return 0, err
	}
	if database != currentDB {
		return 0, fmt.Errorf("postgres cdc: slot %q belongs to database %q, want %q", s.slotName, database, currentDB)
	}
	if active {
		return 0, fmt.Errorf("postgres cdc: slot %q is already active; use a unique slot per pipeline", s.slotName)
	}
	position := confirmed
	if position == nil || *position == "" {
		position = restart
	}
	if position == nil || *position == "" {
		return 0, fmt.Errorf("postgres cdc: slot %q has no restart position", s.slotName)
	}
	lsn, err := pglogrepl.ParseLSN(*position)
	if err != nil {
		return 0, fmt.Errorf("postgres cdc: slot position %q: %w", *position, err)
	}
	return lsn, nil
}

func (s *Source) currentLSN(ctx context.Context) (pglogrepl.LSN, error) {
	var raw string
	if err := s.pool.QueryRow(ctx, "SELECT pg_current_wal_lsn()::text").Scan(&raw); err != nil {
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
		rec, err := rel.record(msg.Tuple, nil, filament.OpInsert, walStart)
		if err != nil {
			return 0, err
		}
		if err := r.push(rec); err != nil {
			return 0, err
		}
	case *pglogrepl.UpdateMessage:
		rel, ok := r.relation(msg.RelationID)
		if !ok {
			return 0, nil
		}
		after, err := rel.record(msg.NewTuple, msg.OldTuple, filament.OpUpdate, walStart)
		if err != nil {
			return 0, err
		}
		if msg.OldTuple != nil {
			before, err := rel.record(msg.OldTuple, nil, filament.OpDelete, walStart)
			if err != nil {
				return 0, err
			}
			if before.ID != after.ID {
				if err := r.push(before); err != nil {
					return 0, err
				}
				after.Op = filament.OpInsert
			}
		}
		if err := r.push(after); err != nil {
			return 0, err
		}
	case *pglogrepl.DeleteMessage:
		rel, ok := r.relation(msg.RelationID)
		if !ok {
			return 0, nil
		}
		rec, err := rel.record(msg.OldTuple, nil, filament.OpDelete, walStart)
		if err != nil {
			return 0, err
		}
		if err := r.push(rec); err != nil {
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
	pks, tracked := r.tracked[msg.RelationName]
	rel.tracked = tracked && msg.Namespace == r.schema
	if tracked {
		for _, pk := range pks {
			for i, col := range msg.Columns {
				if col.Name == pk {
					rel.keys = append(rel.keys, i)
					break
				}
			}
		}
		if len(rel.keys) != len(pks) {
			rel.tracked = false
		}
	}
	r.relations[msg.RelationID] = rel
}

func (r *pgCDCRun) relation(id uint32) (*cdcRelation, bool) {
	rel, ok := r.relations[id]
	return rel, ok && rel.tracked
}

func (r *pgCDCRun) push(rec filament.Record) error {
	r.seq++
	rec.Meta.Seq = r.seq
	if err := r.sink.Push(rec); err != nil {
		return err
	}
	r.emitted++
	if r.limit > 0 && r.emitted >= r.limit {
		r.limited = true
	}
	return nil
}

func (r *pgCDCRun) pushStreamMarks(lsn pglogrepl.LSN) error {
	for _, resource := range r.resources {
		rec := filament.Record{Resource: resource, Drained: true}
		rec.Meta.LSN = lsn.String()
		rec.Meta.Seq = r.seq
		if err := r.sink.Push(rec); err != nil {
			return err
		}
	}
	return nil
}

func (rel *cdcRelation) record(tuple, fallback *pglogrepl.TupleData, op filament.Operation, lsn pglogrepl.LSN) (filament.Record, error) {
	if tuple == nil || len(tuple.Columns) != len(rel.message.Columns) {
		return filament.Record{}, fmt.Errorf("postgres cdc: %s.%s tuple has %d columns, want %d", rel.message.Namespace, rel.message.RelationName, tupleColumnCount(tuple), len(rel.message.Columns))
	}
	values := make([]*pglogrepl.TupleDataColumn, len(tuple.Columns))
	copy(values, tuple.Columns)
	for i, col := range values {
		if op == filament.OpDelete && !containsIndex(rel.keys, i) {
			continue
		}
		if col.DataType != pglogrepl.TupleDataTypeToast {
			continue
		}
		if fallback == nil || len(fallback.Columns) != len(values) || fallback.Columns[i].DataType == pglogrepl.TupleDataTypeToast {
			return filament.Record{}, fmt.Errorf("postgres cdc: unchanged TOAST column %q on %s.%s has no old value; set REPLICA IDENTITY FULL", rel.message.Columns[i].Name, rel.message.Namespace, rel.message.RelationName)
		}
		values[i] = fallback.Columns[i]
	}
	idParts := make([]string, len(rel.keys))
	for n, i := range rel.keys {
		col := values[i]
		if col.DataType != pglogrepl.TupleDataTypeText {
			return filament.Record{}, fmt.Errorf("postgres cdc: primary-key column %q is not present in %s tuple", rel.message.Columns[i].Name, filament.OperationName(op))
		}
		idParts[n] = string(col.Data)
	}

	buf := []byte{'{'}
	written := 0
	for i, col := range values {
		if op == filament.OpDelete && !containsIndex(rel.keys, i) {
			continue
		}
		if written > 0 {
			buf = append(buf, ',')
		}
		key, _ := json.Marshal(rel.message.Columns[i].Name)
		buf = append(buf, key...)
		buf = append(buf, ':')
		var err error
		buf, err = appendPGOutputJSON(buf, col, rel.message.Columns[i].DataType)
		if err != nil {
			return filament.Record{}, fmt.Errorf("postgres cdc: encode %s.%s column %q: %w", rel.message.Namespace, rel.message.RelationName, rel.message.Columns[i].Name, err)
		}
		written++
	}
	buf = append(buf, '}')
	return filament.Record{Resource: rel.message.RelationName, ID: joinKey(idParts), Op: op, Data: buf, Meta: filament.RecordMeta{LSN: lsn.String()}}, nil
}

func appendPGOutputJSON(dst []byte, col *pglogrepl.TupleDataColumn, oid uint32) ([]byte, error) {
	switch col.DataType {
	case pglogrepl.TupleDataTypeNull:
		return append(dst, "null"...), nil
	case pglogrepl.TupleDataTypeText:
	case pglogrepl.TupleDataTypeBinary:
		return nil, fmt.Errorf("binary tuple data is not supported")
	default:
		return nil, fmt.Errorf("tuple data type %q is not materialized", col.DataType)
	}
	raw := col.Data
	switch oid {
	case pgtype.BoolOID:
		if string(raw) == "t" {
			return append(dst, "true"...), nil
		}
		if string(raw) == "f" {
			return append(dst, "false"...), nil
		}
		return nil, fmt.Errorf("invalid boolean %q", raw)
	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID, pgtype.OIDOID,
		pgtype.Float4OID, pgtype.Float8OID, pgtype.NumericOID:
		lower := strings.ToLower(string(raw))
		if lower != "nan" && lower != "infinity" && lower != "-infinity" {
			return append(dst, raw...), nil
		}
	case pgtype.JSONOID, pgtype.JSONBOID:
		if !json.Valid(raw) {
			return nil, fmt.Errorf("invalid JSON value")
		}
		return append(dst, raw...), nil
	}
	quoted, err := json.Marshal(string(raw))
	if err != nil {
		return nil, err
	}
	return append(dst, quoted...), nil
}

func containsIndex(indices []int, want int) bool {
	for _, index := range indices {
		if index == want {
			return true
		}
	}
	return false
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
