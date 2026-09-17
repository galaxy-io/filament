package postgres

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

type sinkEpoch struct {
	ref      filament.EpochRef
	tx       pgx.Tx
	receipts map[string]filament.EpochReceipt
}

var _ filament.StreamingSink = (*Sink)(nil)

// BeginEpoch starts one append transaction. Lifecycle calls follow completed Apply calls.
func (t *Sink) BeginEpoch(ctx context.Context, ref filament.EpochRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	if !t.continuous || t.pool == nil || t.epoch != nil || t.epochFailed || ref.Attempt != t.attempt {
		return filament.ErrEpochMismatch
	}
	if t.lastEpoch != nil && ref.Epoch != t.lastEpoch.ref.Epoch+1 {
		return filament.ErrEpochMismatch
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	t.epoch = &sinkEpoch{ref: ref, tx: tx, receipts: make(map[string]filament.EpochReceipt)}
	return nil
}

func (t *Sink) applyEpoch(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	e := t.epoch
	if e == nil || t.epochFailed || opts.Epoch == nil || *opts.Epoch != e.ref || opts.Policy.Capability.Mode != filament.WriteAppend {
		return filament.WriteReceipt{}, filament.ErrEpochMismatch
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		t.epochFailed = true
		return filament.WriteReceipt{}, err
	}
	tbl := t.tables[b.Resource]
	if tbl == nil {
		t.epochFailed = true
		return filament.WriteReceipt{}, errors.New("postgres stream: schema not ensured")
	}
	c := tbl.copierFor(b.Rows(), false)
	payload, crc := c.encode(b.Rows(), 0, b.NumRows(), false)
	if err := verifyCopyChecksum(payload, crc); err != nil {
		t.epochFailed = true
		return filament.WriteReceipt{}, err
	}
	tag, err := e.tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.qualified, tbl.idents))
	if err != nil {
		t.epochFailed = true
		return filament.WriteReceipt{}, err
	}
	if tag.RowsAffected() != int64(b.NumRows()) {
		t.epochFailed = true
		return filament.WriteReceipt{}, errors.New("postgres stream: copy row count mismatch")
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
	if t.epoch == nil && t.lastEpoch != nil && t.lastEpoch.ref == ref {
		return epochReceipts(t.lastEpoch), nil
	}
	e := t.epoch
	if e == nil || e.ref != ref || t.epochFailed {
		return nil, filament.ErrEpochMismatch
	}
	if err := e.tx.Commit(ctx); err != nil {
		t.epochFailed = true
		return nil, err
	}
	e.tx = nil
	t.lastEpoch = e
	t.epoch = nil
	return epochReceipts(e), nil
}

// AbortEpoch discards only the current transaction, preserving committed epochs.
func (t *Sink) AbortEpoch(ctx context.Context, ref filament.EpochRef) error {
	if t.epoch == nil {
		return nil
	}
	if t.epoch.ref != ref {
		return filament.ErrEpochMismatch
	}
	err := t.epoch.tx.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		err = nil
	}
	t.epoch = nil
	// Failed Apply/Commit keeps the session poisoned even after abort.
	return err
}

// CloseSession is called after the pipeline has joined all Apply calls.
// PostgreSQL connections are closed on cancellation; no background tasks are started.
func (t *Sink) CloseSession(ctx context.Context) error {
	var err error
	if t.epoch != nil {
		err = t.AbortEpoch(ctx, t.epoch.ref)
	}
	t.release()
	if t.epochFailed {
		err = errors.Join(err, errors.New("postgres stream: failed epoch; destination quiescence is unproven"))
	}
	return errors.Join(err, ctx.Err())
}
