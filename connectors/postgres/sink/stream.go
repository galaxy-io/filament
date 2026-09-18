package postgres

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/stream"
)

// streamSession combines shared lifecycle rules with Postgres transaction state.
type streamSession struct {
	lifecycle stream.Lifecycle
	active    *sinkEpoch
	// cleanupErr conservatively retains uncertainty from failed rollback attempts.
	cleanupErr error
}

type sinkEpoch struct {
	ref      filament.EpochRef
	tx       pgx.Tx
	receipts map[string]filament.EpochReceipt
}

var _ filament.StreamingSink = (*Sink)(nil)

// BeginEpoch starts one append transaction. Lifecycle calls follow completed Apply calls.
func (t *Sink) BeginEpoch(ctx context.Context, ref filament.EpochRef) error {
	if !t.continuous || t.pool == nil {
		return filament.ErrEpochMismatch
	}
	if err := t.stream.lifecycle.CheckBegin(ref); err != nil {
		return err
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	if err := t.stream.lifecycle.MarkBegun(ref); err != nil {
		return errors.Join(err, tx.Rollback(ctx))
	}
	t.stream.active = &sinkEpoch{ref: ref, tx: tx, receipts: make(map[string]filament.EpochReceipt)}
	return nil
}

func (t *Sink) applyEpoch(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	e := t.stream.active
	if e == nil || opts.Policy.Capability.Mode != filament.WriteAppend {
		return filament.WriteReceipt{}, filament.ErrEpochMismatch
	}
	if err := t.stream.lifecycle.CheckApply(opts.Epoch); err != nil {
		return filament.WriteReceipt{}, err
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		t.stream.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		err := errors.New("postgres stream: schema not ensured")
		t.stream.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	c := tbl.copierFor(b.Rows(), false)
	payload, crc := c.encode(b.Rows(), 0, b.NumRows(), false)
	if err := verifyCopyChecksum(payload, crc); err != nil {
		t.stream.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	tag, err := e.tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.qualified, tbl.idents))
	if err != nil {
		t.stream.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	if tag.RowsAffected() != int64(b.NumRows()) {
		err := errors.New("postgres stream: copy row count mismatch")
		t.stream.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	r := t.receipt(b, b.NumRows(), int64(len(payload)), writeIntegrity{arrow: b.IntegrityCRC(), encoded: crc})
	total := e.receipts[b.Resource]
	total.Resource = b.Resource
	total.Rows += int64(r.Rows)
	total.Bytes += r.Bytes
	e.receipts[b.Resource] = total
	return r, nil
}

func epochReceipts(e *sinkEpoch) []filament.EpochReceipt {
	result := make([]filament.EpochReceipt, 0, len(e.receipts))
	for _, r := range e.receipts {
		result = append(result, r)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Resource < result[j].Resource })
	return result
}

// CommitEpoch returns receipts only after PostgreSQL confirms transaction commit.
// A failed commit is ambiguous; the session cannot begin another epoch.
func (t *Sink) CommitEpoch(ctx context.Context, ref filament.EpochRef) ([]filament.EpochReceipt, error) {
	if receipts, replay, err := t.stream.lifecycle.CheckCommit(ref); err != nil || replay {
		return receipts, err
	}
	e := t.stream.active
	if err := e.tx.Commit(ctx); err != nil {
		t.stream.lifecycle.Fail(err)
		return nil, err
	}
	receipts := epochReceipts(e)
	if err := t.stream.lifecycle.MarkCommitted(ref, receipts); err != nil {
		return nil, err
	}
	t.stream.active = nil
	return receipts, nil
}

// AbortEpoch discards only the current transaction, preserving committed epochs.
func (t *Sink) AbortEpoch(ctx context.Context, ref filament.EpochRef) error {
	if err := t.stream.lifecycle.CheckAbort(ref); err != nil {
		return err
	}
	if t.stream.active == nil {
		return nil
	}
	err := t.stream.active.tx.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		err = nil
	}
	if err != nil {
		t.stream.lifecycle.Fail(err)
		t.stream.cleanupErr = errors.Join(t.stream.cleanupErr, err)
		return err
	}
	t.stream.active = nil
	return t.stream.lifecycle.MarkAborted(ref)
}

// CloseSession is called after the pipeline has joined all Apply calls.
// PostgreSQL connections are closed on cancellation; no background tasks are started.
func (t *Sink) CloseSession(ctx context.Context) error {
	if closed, err := t.stream.lifecycle.Closed(); closed {
		return err
	}
	var err error
	if t.stream.active != nil {
		err = t.AbortEpoch(ctx, t.stream.active.ref)
	}
	t.release()
	// Retain Postgres's conservative policy for failed sessions; the shared
	// lifecycle does not infer quiescence from a failure.
	if failure := t.stream.lifecycle.Failure(); failure != nil {
		err = errors.Join(err, errors.New("postgres stream: failed epoch; destination quiescence is unproven"), failure)
	}
	err = errors.Join(err, t.stream.cleanupErr, ctx.Err())
	t.stream.lifecycle.MarkClosed(err)
	return err
}
