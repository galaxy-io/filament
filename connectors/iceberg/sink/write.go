package iceberg

import (
	"context"
	"fmt"
	"iter"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	iceberg "github.com/apache/iceberg-go"
	"github.com/apache/iceberg-go/table"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// Apply validates the batch against the run's write policy and buffers it
// for the resource's table, retaining its rows until commit (or spilling them).
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	return s.applyWrite(ctx, b, opts)
}

func (s *Sink) applyBatch(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
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
	st := s.active
	cat := s.cat
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
	st.mu.Unlock()

	rb.mu.Lock()
	defer rb.mu.Unlock()
	if err := rb.setPolicy(policy); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
	}
	var err error
	if policy.Capability.Mode == filament.WriteAppend {
		// Append writes data files now; Commit only attaches them.
		ident := it.tbl.Identifier()
		err = rb.appendFile(ctx, b, nbytes, func() *appendFiles { return newAppendFiles(cat, ident) })
	} else {
		err = rb.append(b, nbytes)
	}
	if err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("iceberg sink: buffer %s: %w", b.Resource, err)
	}

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
	if rb.files != nil {
		if mode != writeModeAppend {
			return fmt.Errorf("data files were written for %s mode", mode)
		}
		updated, err := rb.files.commit(ctx, it.tbl)
		if err != nil {
			return fmt.Errorf("append: %w", err)
		}
		it.tbl = updated
		it.schema = updated.Schema()
		return nil
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
	var (
		deleteIDs    []int
		deleteSchema *arrow.Schema
	)
	if mergeOnRead(it.tbl, mode, rb.policy.Keys) {
		var err error
		if deleteIDs, deleteSchema, err = equalityDeleteSchema(it, rb.policy.Keys); err != nil {
			return err
		}
	} else if has, err := hasDeleteFiles(ctx, it.tbl); err != nil {
		return fmt.Errorf("%s: read manifests: %w", mode, err)
	} else if has {
		return fmt.Errorf("%s: table has delete files that a copy-on-write rewrite would drop; compact them or set the table back to merge-on-read", mode)
	}
	m, err := foldMutations(ctx, it, rb, rb.policy.Keys, arrowSchema, deleteSchema)
	if err != nil {
		return err
	}
	defer m.release()
	if m.touched == 0 {
		return nil
	}
	if deleteSchema != nil {
		return commitRowDelta(ctx, it, m, deleteIDs, arrowSchema)
	}

	// Overwrite always rewrites files, where Delete follows write.delete.mode and
	// could leave position deletes a later rewrite would drop.
	txn := it.tbl.NewTransaction()
	rdr, err := array.NewRecordReader(arrowSchema, m.live)
	if err != nil {
		return fmt.Errorf("mutation reader: %w", err)
	}
	defer rdr.Release()
	if err := txn.Overwrite(ctx, rdr, nil, table.WithOverwriteFilter(m.filter)); err != nil {
		return fmt.Errorf("%s: %w", mode, err)
	}
	updated, err := txn.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	it.tbl = updated
	it.schema = updated.Schema()
	return nil
}

// mergeOnRead reports whether the table asks for merge-on-read: write.merge.mode
// governs upsert and merge, write.delete.mode pure deletes. Iceberg defaults
// both to copy-on-write. Format v1 tables cannot hold delete files, and a
// partitioned delete must land in the partition of the row it shadows, which an
// update may have moved, so both fall back to copy-on-write. Floating-point
// columns cannot be equality-delete keys, so a float key falls back too.
func mergeOnRead(tbl *table.Table, mode writeMode, keys []string) bool {
	key := writeMergeModeKey
	if mode == writeModeDelete {
		key = table.WriteDeleteModeKey
	}
	if tbl.Properties().Get(key, table.WriteModeCopyOnWrite) != table.WriteModeMergeOnRead {
		return false
	}
	if tbl.Metadata().Version() < 2 || !tbl.Spec().IsUnpartitioned() {
		return false
	}
	for _, k := range keys {
		if field, ok := tbl.Schema().FindFieldByName(k); ok {
			switch field.Type.(type) {
			case iceberg.Float32Type, iceberg.Float64Type:
				return false
			}
		}
	}
	return true
}

// hasDeleteFiles reports whether the current snapshot tracks any live delete
// files. iceberg-go's copy-on-write rewrite reads data files without their
// delete files, so rewriting such a table would resurrect deleted rows. The
// manifest list is read because the snapshot summary totals are optional.
func hasDeleteFiles(ctx context.Context, tbl *table.Table) (bool, error) {
	snap := tbl.CurrentSnapshot()
	if snap == nil {
		return false, nil
	}
	fs, err := tbl.FS(ctx)
	if err != nil {
		return false, err
	}
	manifests, err := snap.Manifests(fs)
	if err != nil {
		return false, err
	}
	for _, m := range manifests {
		if m.ManifestContent() == iceberg.ManifestContentDeletes && m.AddedDataFiles()+m.ExistingDataFiles() > 0 {
			return true, nil
		}
	}
	return false, nil
}

// equalityDeleteSchema resolves the key columns to field IDs and the Arrow
// schema the equality-delete writer expects for them.
func equalityDeleteSchema(it *iceTable, keys []string) ([]int, *arrow.Schema, error) {
	ids := make([]int, len(keys))
	for i, key := range keys {
		field, ok := it.schema.FindFieldByName(key)
		if !ok {
			return nil, nil, fmt.Errorf("primary key %q is not in iceberg schema", key)
		}
		ids[i] = field.ID
	}
	keySchema, err := it.schema.Select(true, keys...)
	if err != nil {
		return nil, nil, err
	}
	sc, err := table.SchemaToArrowSchemaWithOptions(keySchema, table.ArrowSchemaOptions{IncludeFieldIDs: true, TableProperties: it.tbl.Properties()})
	if err != nil {
		return nil, nil, err
	}
	return ids, sc, nil
}

// commitRowDelta writes the surviving rows as data files and the remaining
// touched keys as equality deletes, committed as one snapshot. The deletes only
// shadow rows from earlier snapshots, so the rows added beside them survive.
func commitRowDelta(ctx context.Context, it *iceTable, m *mutation, deleteIDs []int, arrowSchema *arrow.Schema) error {
	if len(m.live) == 0 && len(m.deletes) == 0 {
		return nil
	}
	txn := it.tbl.NewTransaction()
	rd := txn.NewRowDelta(nil)
	if len(m.deletes) > 0 {
		deletes, err := txn.WriteEqualityDeletes(ctx, deleteIDs, borrowed(m.deletes))
		if err != nil {
			return fmt.Errorf("write equality deletes: %w", err)
		}
		rd.AddDeletes(deletes...)
	}
	if len(m.live) > 0 {
		for df, err := range table.WriteRecords(ctx, it.tbl, arrowSchema, retained(m.live)) {
			if err != nil {
				return fmt.Errorf("write data files: %w", err)
			}
			rd.AddRows(df)
		}
	}
	if err := rd.Commit(ctx); err != nil {
		return fmt.Errorf("row delta: %w", err)
	}
	updated, err := txn.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	it.tbl = updated
	it.schema = updated.Schema()
	return nil
}

// retained yields each batch with an extra reference for WriteRecords, which
// releases every batch it is handed, including the one it stops on. The
// mutation still releases its own.
func retained(batches []arrow.RecordBatch) iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		for _, b := range batches {
			b.Retain()
			if !yield(b, nil) {
				return
			}
		}
	}
}

// borrowed yields batches without a reference for WriteEqualityDeletes, whose
// writers retain what they keep and release nothing they are handed.
func borrowed(batches []arrow.RecordBatch) iter.Seq2[arrow.RecordBatch, error] {
	return func(yield func(arrow.RecordBatch, error) bool) {
		for _, b := range batches {
			if !yield(b, nil) {
				return
			}
		}
	}
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
