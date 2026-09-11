package postgres

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Apply validates the batch against the run's write policy, then routes it: replace
// and append COPY into the table, upsert folds through a temp table, merge applies
// the change stream in order.
func (t *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if t.pool == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write before open")
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: no schema ensured for resource %q", b.Resource)
	}
	switch opts.Policy.Capability.Mode {
	case filament.WriteReplace:
		policy := opts.Policy
		policy.Capability.AcceptsOps = []filament.Operation{filament.OpInsert}
		if err := policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.write(ctx, tbl, b)
	case filament.WriteAppend:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.write(ctx, tbl, b)
	case filament.WriteUpsert:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.writeUpsert(ctx, tbl, b)
	case filament.WriteMerge:
		if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("postgres sink: %w", err)
		}
		return t.writeMerge(ctx, tbl, b)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

// copierFor returns the batch's COPY renderer over all columns, built once per
// Arrow schema (one per source builder). Column order is the schema's; the
// destination column list is the same order.
func (tbl *table) copierFor(rows arrow.RecordBatch, keysOnly bool) *copier {
	schema := rows.Schema()
	tbl.mu.Lock()
	defer tbl.mu.Unlock()
	cache := tbl.copier
	if keysOnly {
		cache = tbl.keys
	}
	if c := cache[schema]; c != nil {
		return c
	}
	idx := make([]int, schema.NumFields())
	for i := range idx {
		idx[i] = i
	}
	types := tbl.types
	if keysOnly {
		idx, types = tbl.keyIdx, pick(tbl.types, tbl.keyIdx)
	}
	c := newCopier(schema, idx, types)
	cache[schema] = c
	return c
}

// write COPYs the whole batch into the table.
func (t *Sink) write(ctx context.Context, tbl *table, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	rows := b.Rows()
	c := tbl.copierFor(rows, false)
	payload, expectedCRC := c.encode(rows, 0, b.NumRows(), false)
	integrity := writeIntegrity{arrow: b.IntegrityCRC()}
	conn, err := t.pool.Acquire(ctx)
	if err != nil {
		return filament.WriteReceipt{}, copyError("acquire", b.Resource, b.Seq, err)
	}
	defer conn.Release()
	integrity.encoded = crc32.Update(integrity.encoded, copyCRCTable, payload)
	if err := verifyCopyChecksum(payload, expectedCRC); err != nil {
		return filament.WriteReceipt{}, copyError("verify copy", b.Resource, b.Seq, err)
	}
	tag, err := conn.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.qualified, tbl.idents))
	if err != nil {
		return filament.WriteReceipt{}, copyError("copy", b.Resource, b.Seq, err)
	}
	t.written.Add(tag.RowsAffected())
	return t.receipt(b, int(tag.RowsAffected()), int64(len(payload)), integrity), nil
}

// writeUpsert loads the batch into the connection's temp table and folds it into
// the destination by key, in one transaction.
func (t *Sink) writeUpsert(ctx context.Context, tbl *table, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if tbl.upsertSQL == "" {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: upsert on keyless resource %q", b.Resource)
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return filament.WriteReceipt{}, copyError("begin", b.Resource, b.Seq, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()
	var integrity writeIntegrity
	rows, nbytes, err := t.upsertRange(ctx, tx, tbl, b, 0, b.NumRows(), &integrity)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.WriteReceipt{}, copyError("commit", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes, integrity), nil
}

// upsertRange COPYs rows [lo, hi) into the temp table and folds them into the
// destination. Returns rows folded and payload bytes.
func (t *Sink) upsertRange(ctx context.Context, tx pgx.Tx, tbl *table, b *arrowbatch.Batch, lo, hi int, integrity *writeIntegrity) (int, int64, error) {
	if _, err := tx.Exec(ctx, tbl.tempSQL); err != nil {
		return 0, 0, copyError("temp table", b.Resource, b.Seq, err)
	}
	rows := b.Rows()
	c := tbl.copierFor(rows, false)
	payload, expectedCRC := c.encode(rows, lo, hi, true)
	integrity.arrow = b.IntegrityCRC()
	idents := append(append(make([]string, 0, len(tbl.idents)+1), tbl.idents...), "_ord")
	integrity.encoded = crc32.Update(integrity.encoded, copyCRCTable, payload)
	if err := verifyCopyChecksum(payload, expectedCRC); err != nil {
		return 0, 0, copyError("verify copy", b.Resource, b.Seq, err)
	}
	if _, err := tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.temp, idents)); err != nil {
		return 0, 0, copyError("copy", b.Resource, b.Seq, err)
	}
	if _, err := tx.Exec(ctx, tbl.upsertSQL); err != nil {
		return 0, 0, copyError("upsert", b.Resource, b.Seq, err)
	}
	return hi - lo, int64(len(payload)), nil
}

// deleteRange COPYs the keys of rows [lo, hi) into the keys temp table and deletes
// the matching destination rows.
func (t *Sink) deleteRange(ctx context.Context, tx pgx.Tx, tbl *table, b *arrowbatch.Batch, lo, hi int, integrity *writeIntegrity) (int, int64, error) {
	if _, err := tx.Exec(ctx, tbl.keysTempSQL); err != nil {
		return 0, 0, copyError("keys temp table", b.Resource, b.Seq, err)
	}
	rows := b.Rows()
	c := tbl.copierFor(rows, true)
	payload, expectedCRC := c.encode(rows, lo, hi, false)
	integrity.arrow = b.IntegrityCRC()
	keyIdents := pick(tbl.idents, tbl.keyIdx)
	integrity.encoded = crc32.Update(integrity.encoded, copyCRCTable, payload)
	if err := verifyCopyChecksum(payload, expectedCRC); err != nil {
		return 0, 0, copyError("verify copy keys", b.Resource, b.Seq, err)
	}
	if _, err := tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.keysTemp, keyIdents)); err != nil {
		return 0, 0, copyError("copy keys", b.Resource, b.Seq, err)
	}
	if _, err := tx.Exec(ctx, tbl.deleteSQL); err != nil {
		return 0, 0, copyError("delete", b.Resource, b.Seq, err)
	}
	return hi - lo, int64(len(payload)), nil
}

// writeMerge applies CDC rows in arrival order. Maximal delete/non-delete runs
// become one fold each, and one database transaction makes the whole batch atomic
// before its WAL checkpoint can be committed.
func (t *Sink) writeMerge(ctx context.Context, tbl *table, b *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if tbl.upsertSQL == "" {
		return filament.WriteReceipt{}, fmt.Errorf("postgres sink: merge on keyless resource %q", b.Resource)
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return filament.WriteReceipt{}, copyError("begin merge", b.Resource, b.Seq, err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

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
			k, nb, err = t.upsertRange(ctx, tx, tbl, b, lo, hi, &integrity)
		}
		if err != nil {
			return filament.WriteReceipt{}, err
		}
		rows += k
		nbytes += nb
		lo = hi
	}
	if err := tx.Commit(ctx); err != nil {
		return filament.WriteReceipt{}, copyError("commit merge", b.Resource, b.Seq, err)
	}
	t.written.Add(int64(rows))
	return t.receipt(b, rows, nbytes, integrity), nil
}

type writeIntegrity struct {
	arrow   uint32
	encoded uint32
}

func (t *Sink) receipt(b *arrowbatch.Batch, rows int, nbytes int64, integrity writeIntegrity) filament.WriteReceipt {
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("postgres://%s.%s", t.schema, b.Resource),
		Bytes:      nbytes,
		Rows:       rows,
		WriteCRC:   integrity.arrow,
		EncodedCRC: &integrity.encoded,
	}
}
