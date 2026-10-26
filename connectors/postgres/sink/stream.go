package postgres

import (
	"bytes"
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/sinkstate"
)

// writeSession combines shared lifecycle rules with Postgres transaction state.
type writeSession struct {
	lifecycle sinkstate.Lifecycle
	active    *transactionWrite
	// cleanupErr conservatively retains uncertainty from failed rollback attempts.
	cleanupErr error
	commitErr  error
}

type transactionWrite struct {
	tx             pgx.Tx
	previousTables map[string]*table
}

// Begin starts one write transaction. Lifecycle calls follow completed Apply calls.
func (t *Sink) Begin(ctx context.Context) error {
	if t.pool == nil {
		return sinkstate.ErrState
	}
	if err := t.session.lifecycle.CheckBegin(); err != nil {
		return err
	}
	if !t.transactionalWrites {
		return t.session.lifecycle.MarkBegun()
	}
	tx, err := t.pool.Begin(ctx)
	if err != nil {
		return err
	}
	// Acquire locks in a stable order, retaining them through epoch completion.
	names := make([]string, 0, len(t.tables))
	for name := range t.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := t.validateTable(ctx, tx, t.tables[name]); err != nil {
			return errors.Join(err, tx.Rollback(ctx))
		}
	}
	if err := t.session.lifecycle.MarkBegun(); err != nil {
		return errors.Join(err, tx.Rollback(ctx))
	}
	t.session.active = &transactionWrite{tx: tx}
	return nil
}

func (t *Sink) applyWrite(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	e := t.session.active
	if e == nil {
		return filament.WriteReceipt{}, sinkstate.ErrState
	}
	if err := t.session.lifecycle.CheckApply(); err != nil {
		return filament.WriteReceipt{}, err
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	tbl := t.tableFor(b.Resource)
	if tbl == nil {
		err := errors.New("postgres stream: schema not ensured")
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}

	if opts.Policy.Capability.Mode == filament.WriteMerge || opts.Policy.Capability.Mode == filament.WriteUpsert {
		r, err := t.mergeInTransaction(ctx, e.tx, tbl, b)
		if err != nil {
			t.session.lifecycle.Fail(err)
			return filament.WriteReceipt{}, err
		}
		return r, nil
	}
	if opts.Policy.Capability.Mode != filament.WriteAppend {
		err := sinkstate.ErrState
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	c, err := tbl.copierFor(b.Rows(), false)
	if err != nil {
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	payload, crc := c.encode(b.Rows(), 0, b.NumRows(), false)
	if err := verifyCopyChecksum(payload, crc); err != nil {
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	tag, err := e.tx.Conn().PgConn().CopyFrom(ctx, bytes.NewReader(payload), c.sql(tbl.qualified, tbl.idents))
	if err != nil {
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	if tag.RowsAffected() != int64(b.NumRows()) {
		err := errors.New("postgres stream: copy row count mismatch")
		t.session.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	r := t.receipt(b, b.NumRows(), int64(len(payload)), writeIntegrity{arrow: b.IntegrityCRC(), encoded: crc})
	return r, nil
}

// Commit succeeds only after PostgreSQL confirms transaction commit.
// A failed commit is ambiguous; the session cannot begin another epoch.
func (t *Sink) Commit(ctx context.Context) error {
	if err := t.session.lifecycle.CheckApply(); err != nil {
		return err
	}
	if !t.transactionalWrites {
		return t.session.lifecycle.MarkCommitted()
	}
	e := t.session.active
	if err := e.tx.Commit(ctx); err != nil {
		t.session.commitErr = err
		t.session.lifecycle.Fail(err)
		return err
	}
	if err := t.session.lifecycle.MarkCommitted(); err != nil {
		return err
	}
	t.session.active = nil
	return nil
}

// Abort discards only the current transaction, preserving committed epochs.
func (t *Sink) Abort(ctx context.Context) error {
	if err := t.session.lifecycle.CheckAbort(); err != nil {
		return err
	}
	if !t.transactionalWrites {
		if err := t.abortSnapshot(ctx); err != nil {
			return err
		}
		return t.session.lifecycle.MarkAborted()
	}
	if t.session.active == nil {
		return nil
	}
	err := t.session.active.tx.Rollback(ctx)
	if errors.Is(err, pgx.ErrTxClosed) {
		err = nil
	}
	if err != nil {
		t.session.lifecycle.Fail(err)
		t.session.cleanupErr = errors.Join(t.session.cleanupErr, err)
		return err
	}
	for resource, previous := range t.session.active.previousTables {
		if previous == nil {
			delete(t.tables, resource)
		} else {
			t.tables[resource] = previous
		}
	}
	t.session.active = nil
	return t.session.lifecycle.MarkAborted()
}

// Close is called after the pipeline has joined all Apply calls.
// PostgreSQL connections are closed on cancellation; no background tasks are started.
func (t *Sink) Close(ctx context.Context) error {
	if closed, err := t.session.lifecycle.Closed(); closed {
		return err
	}
	var err error
	if t.session.active != nil {
		err = t.Abort(ctx)
	}
	t.release()
	// A confirmed rollback proves the failed transaction has ended. A lost
	// commit response or rollback failure still leaves its outcome uncertain.
	if failure := t.session.lifecycle.Failure(); failure != nil && (!t.transactionalWrites || t.session.commitErr != nil || t.session.cleanupErr != nil) {
		err = errors.Join(err, errors.New("postgres stream: failed epoch; destination quiescence is unproven"), failure)
	}
	err = errors.Join(err, t.session.cleanupErr, ctx.Err())
	t.session.lifecycle.MarkClosed(err)
	return err
}
