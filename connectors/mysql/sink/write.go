package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"hash/crc32"
	"strings"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Apply validates the batch against the run's write policy, then routes it:
// replace and append load into the table, upsert loads with REPLACE, merge
// applies the change stream in order.
func (t *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if t.db == nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write before open")
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: no schema ensured for resource %q", b.Resource)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.write(ctx, tbl, b, false)
	case filament.WriteAppend:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.write(ctx, tbl, b, false)
	case filament.WriteUpsert:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.write(ctx, tbl, b, true)
	case filament.WriteMerge:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("mysql sink: %w", err)
		}
		return t.writeMerge(ctx, tbl, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// execer runs statements on one session: a connection or a transaction, so
// SHOW WARNINGS reads the statement just run.
type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// load streams rows [lo, hi) into the table (or, keysOnly, its keys temp table).
// LOCAL INFILE implies IGNORE: the server turns duplicate keys and bad values
// into skipped rows and coerced values with a warning, so the load checks the
// count of rows landed and the session's warnings and fails on either. Returns
// the rows loaded and payload bytes.
func (tbl *table) load(ctx context.Context, x execer, b *arrowbatch.Batch, lo, hi int, replace, keysOnly bool, integrity *writeIntegrity) (int, int64, error) {
	rows := b.Rows()
	if int(rows.NumCols()) != len(tbl.idents) {
		return 0, 0, fmt.Errorf("mysql sink: %s seq %d has %d columns, table has %d", b.Resource, b.Seq, rows.NumCols(), len(tbl.idents))
	}
	l, target, idents, json := tbl.rows, tbl.qualified, tbl.idents, tbl.json
	if keysOnly {
		l, target, idents, json = tbl.keys, tbl.keysTemp, tbl.keyIdents, nil
	}
	payload, expectedCRC := l.encode(rows, lo, hi)
	integrity.arrow = b.IntegrityCRC()
	integrity.encoded = crc32.Update(integrity.encoded, loadCRCTable, payload)
	name, done := register(payload)
	defer done()
	if err := verifyLoadChecksum(payload, expectedCRC); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: verify load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	res, err := x.ExecContext(ctx, loadSQL(name, target, idents, json, replace))
	if err != nil {
		return 0, 0, fmt.Errorf("mysql sink: load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	if !replace { // with REPLACE a duplicate is the point and counts twice
		if n, err := res.RowsAffected(); err == nil && n != int64(hi-lo) {
			return 0, 0, fmt.Errorf("mysql sink: load %s seq %d: %d of %d rows landed (duplicate keys?)", b.Resource, b.Seq, n, hi-lo)
		}
	}
	if err := firstWarning(ctx, x); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: load %s seq %d: %w", b.Resource, b.Seq, err)
	}
	return hi - lo, int64(len(payload)), nil
}

// firstWarning returns the session's first warning from the statement just run
// as an error, or nil when there is none.
func firstWarning(ctx context.Context, x execer) error {
	rows, err := x.QueryContext(ctx, "SHOW WARNINGS LIMIT 1")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return rows.Err()
	}
	var level, msg string
	var code int
	if err := rows.Scan(&level, &code, &msg); err != nil {
		return err
	}
	return fmt.Errorf("%s %d: %s", strings.ToLower(level), code, msg)
}

// write loads the whole batch; replace makes duplicate keys overwrite, the
// idempotent upsert an at-least-once resume relies on.
func (t *Sink) write(ctx context.Context, tbl *table, b *arrowbatch.Batch, replace bool) (filament.WriteReceipt, error) {
	conn, err := t.db.Conn(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: write %s: %w", b.Resource, err)
	}
	defer func() { _ = conn.Close() }()
	var integrity writeIntegrity
	rows, nbytes, err := tbl.load(ctx, conn, b, 0, b.NumRows(), replace, false, &integrity)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes, integrity), nil
}

// writeMerge applies a CDC batch: rows split into maximal same-kind runs in
// arrival order — a delete of a key must not jump over its re-insert — and each
// run lands as one statement: inserts/updates through LOAD DATA REPLACE, deletes
// through the keys temp table and a join delete. One transaction on one
// connection (the temp table is per session) makes the batch atomic.
func (t *Sink) writeMerge(ctx context.Context, tbl *table, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if tbl.deleteSQL == "" {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: merge on keyless resource %q", b.Resource)
	}
	conn, err := t.db.Conn(ctx)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: merge %s: %w", b.Resource, err)
	}
	defer func() { _ = conn.Close() }()
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: begin merge %s: %w", b.Resource, err)
	}
	defer func() { _ = tx.Rollback() }()

	var nbytes int64
	rows := 0
	var integrity writeIntegrity
	n := b.NumRows()
	for lo := 0; lo < n; {
		deleting := b.Op(lo) == rowmodel.OpDelete
		hi := lo + 1
		for hi < n && (b.Op(hi) == rowmodel.OpDelete) == deleting {
			hi++
		}
		var (
			k   int
			nb  int64
			err error
		)
		if deleting {
			k, nb, err = t.deleteRange(ctx, tx, tbl, b, lo, hi, &integrity)
		} else {
			k, nb, err = tbl.load(ctx, tx, b, lo, hi, true, false, &integrity)
		}
		if err != nil {
			return filament.WriteReceipt{}, err
		}
		rows += k
		nbytes += nb
		lo = hi
	}
	if err := tx.Commit(); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("mysql sink: commit merge %s seq %d: %w", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes, integrity), nil
}

// deleteRange loads the keys of rows [lo, hi) into the session's keys temp table
// and deletes the matching destination rows.
func (t *Sink) deleteRange(ctx context.Context, tx *sql.Tx, tbl *table, b *arrowbatch.Batch, lo, hi int, integrity *writeIntegrity) (int, int64, error) {
	if _, err := tx.ExecContext(ctx, tbl.keysTempSQL); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: keys temp table %s: %w", b.Resource, err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+tbl.keysTemp); err != nil { //nolint:gosec // identifier backtick-quoted via quoteIdent
		return 0, 0, fmt.Errorf("mysql sink: clear keys %s: %w", b.Resource, err)
	}
	k, nb, err := tbl.load(ctx, tx, b, lo, hi, false, true, integrity)
	if err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, tbl.deleteSQL); err != nil {
		return 0, 0, fmt.Errorf("mysql sink: delete %s seq %d: %w", b.Resource, b.Seq, err)
	}
	return k, nb, nil
}

type writeIntegrity struct {
	arrow   uint32
	encoded uint32
}

func (t *Sink) receipt(b *arrowbatch.Batch, rows int, nbytes int64, integrity writeIntegrity) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("mysql://%s.%s", t.database, b.Resource),
		Bytes:      nbytes,
		Rows:       rows,
		WriteCRC:   integrity.arrow,
		EncodedCRC: &integrity.encoded,
	}
}
