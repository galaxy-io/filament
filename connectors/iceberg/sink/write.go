package iceberg

import (
	"context"
	"fmt"
	"iter"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Apply validates the batch against the run's write policy and buffers it
// for the resource's table, retaining its rows until commit (or spilling them).
func (s *Sink) Apply(_ context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	it := s.tables[b.Resource]
	if it == nil {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: no schema ensured for resource %q", b.Resource)
	}
	policy := opts.Policy
	if len(policy.Keys) == 0 {
		policy.Keys = append([]string(nil), it.primaryKey...)
	}
	if policy.Capability.RequiresPK && len(policy.Keys) == 0 {
		s.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: write policy %q requires primary key for resource %q", policy.Capability.Mode, b.Resource)
	}
	st := s.activeStageLocked()
	s.mu.Unlock()

	if err := policy.ValidateBatch(b.Resource, b); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: %w", err)
	}
	nbytes := arrowbatch.Bytes(b.Rows())

	st.mu.Lock()
	rb := st.buf[b.Resource]
	if rb == nil {
		rb = newRecordBuf(s.stageBufLimitBytes)
		st.buf[b.Resource] = rb
	}
	if err := rb.setPolicy(policy); err != nil {
		st.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
	}
	if err := rb.append(b, nbytes); err != nil {
		st.mu.Unlock()
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
	}
	st.mu.Unlock()

	return filament.WriteReceipt{
		URI:      it.tbl.Location(),
		Bytes:    nbytes,
		Rows:     b.NumRows(),
		WriteCRC: b.IntegrityCRC(),
	}, nil
}

// writeBuffer streams a resource buffer into the Iceberg table in one
// transaction, one batch at a time. It reloads the table from the catalog
// first: a credential-vending catalog issues storage tokens with a TTL at load
// time, and a buffered run can outlive them by commit.
func (s *Sink) writeBuffer(ctx context.Context, it *iceTable, rb *recordBuf, mode writeMode) error {
	s.mu.Lock()
	cat := s.cat
	s.mu.Unlock()
	tbl, err := cat.LoadTable(ctx, it.tbl.Identifier())
	if err != nil {
		return fmt.Errorf("reload table: %w", err)
	}
	it.tbl = tbl
	it.schema = tbl.Schema()

	arrowSchema, err := table.SchemaToArrowSchema(it.schema, nil, false, false)
	if err != nil {
		return fmt.Errorf("build arrow schema: %w", err)
	}

	switch mode {
	case writeModeUpsert, writeModeDelete, writeModeMerge:
		return s.writeMutationBuffer(ctx, it, rb, mode, arrowSchema)
	}

	batches, errPtr := conformedBatches(rb, arrowSchema)
	rdr := array.ReaderFromIter(arrowSchema, batches)
	defer rdr.Release()

	txn := it.tbl.NewTransaction()
	switch mode {
	case writeModeAppend:
		if err := txn.Append(ctx, rdr, nil); err != nil {
			return fmt.Errorf("append: %w", err)
		}
	case writeModeReplace:
		if err := txn.Overwrite(ctx, rdr, nil); err != nil {
			return fmt.Errorf("replace: %w", err)
		}
	default:
		return fmt.Errorf("unsupported write mode %q", mode)
	}
	if *errPtr != nil {
		return fmt.Errorf("read buffer: %w", *errPtr)
	}
	updated, err := txn.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	it.tbl = updated
	it.schema = updated.Schema()
	return nil
}

func (s *Sink) writeMutationBuffer(ctx context.Context, it *iceTable, rb *recordBuf, mode writeMode, arrowSchema *arrow.Schema) error {
	m, err := foldMutations(ctx, it, rb, rb.policy.Keys, arrowSchema)
	if err != nil {
		return err
	}
	defer m.release()
	if m.filter == nil {
		return nil
	}

	txn := it.tbl.NewTransaction()
	if len(m.live) == 0 {
		if err := txn.Delete(ctx, m.filter, nil); err != nil {
			return fmt.Errorf("delete: %w", err)
		}
	} else {
		rdr, err := array.NewRecordReader(arrowSchema, m.live)
		if err != nil {
			return fmt.Errorf("mutation reader: %w", err)
		}
		defer rdr.Release()
		if err := txn.Overwrite(ctx, rdr, nil, table.WithOverwriteFilter(m.filter)); err != nil {
			return fmt.Errorf("%s: %w", mode, err)
		}
	}
	updated, err := txn.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	it.tbl = updated
	it.schema = updated.Schema()
	return nil
}

// conformedBatches returns an iterator yielding each buffered batch in the
// table's Arrow schema. Any streaming error is surfaced through the returned
// pointer (the iter.Seq2 stops on the first error). Caller checks *err after
// the reader drains.
//
//nolint:gocritic // *error is the iterator out-param, read after the seq drains
func conformedBatches(rb *recordBuf, schema *arrow.Schema) (iter.Seq2[arrow.RecordBatch, error], *error) {
	var streamErr error
	seq := func(yield func(arrow.RecordBatch, error) bool) {
		streamErr = rb.stream(func(rows arrow.RecordBatch, _ []rowmodel.Operation) error {
			rec, err := conform(rows, schema)
			if err != nil {
				return err
			}
			if !yield(rec, nil) {
				rec.Release()
				return errStopIteration
			}
			return nil
		})
		if streamErr == errStopIteration {
			streamErr = nil
		}
	}
	return seq, &streamErr
}

// errStopIteration unwinds rb.stream when the consumer stops early; it is never
// surfaced as a real error.
var errStopIteration = fmt.Errorf("stop iteration")
