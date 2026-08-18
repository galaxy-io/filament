package iceberg

import (
	"context"
	"fmt"
	"iter"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
)

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
		streamErr = rb.stream(func(rows arrow.RecordBatch, _ []filament.Operation) error {
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
