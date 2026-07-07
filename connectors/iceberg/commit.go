package iceberg

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/iceberg-go/table"
)

// commitRowsPerChunk bounds how many records are parsed into one Arrow record
// batch at a time, so a large buffer streams to the writer instead of
// materializing as one giant JSON string + table in memory.
const commitRowsPerChunk = 8192

// writeBuffer streams a resource buffer into the Iceberg table in one
// transaction, parsing the buffer in bounded chunks rather than all at once.
func (s *Sink) writeBuffer(ctx context.Context, it *iceTable, rb *recordBuf, mode writeMode) error {
	switch mode {
	case writeModeUpsert, writeModeDelete, writeModeMerge:
		return s.writeMutationBuffer(ctx, it, rb, mode)
	}

	arrowSchema, err := table.SchemaToArrowSchema(it.schema, nil, false, false)
	if err != nil {
		return fmt.Errorf("build arrow schema: %w", err)
	}
	mem := memory.NewGoAllocator()

	// Collect chunk payloads lazily via the buffer's streamer, converting each to
	// an Arrow record batch only as the reader pulls it.
	batches, errPtr := chunkBatches(arrowSchema, mem, rb, jsonColumns(it.record))
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

func (s *Sink) writeMutationBuffer(ctx context.Context, it *iceTable, rb *recordBuf, mode writeMode) error {
	keys := rb.policy.Keys
	changes, err := collectMutationState(it, rb, keys)
	if err != nil {
		return err
	}
	if len(changes.filterKeys) == 0 {
		return nil
	}
	filter, err := keyFilter(it, keys, changes.filterKeys)
	if err != nil {
		return err
	}

	txn := it.tbl.NewTransaction()
	if len(changes.live) == 0 {
		if err := txn.Delete(ctx, filter, nil); err != nil {
			return fmt.Errorf("delete: %w", err)
		}
	} else {
		arrowSchema, err := table.SchemaToArrowSchema(it.schema, nil, false, false)
		if err != nil {
			return fmt.Errorf("build arrow schema: %w", err)
		}
		mem := memory.NewGoAllocator()
		rb := recordsFromRaw(changes.live)
		batches, errPtr := chunkBatches(arrowSchema, mem, rb, jsonColumns(it.record))
		rdr := array.ReaderFromIter(arrowSchema, batches)
		defer rdr.Release()
		if err := txn.Overwrite(ctx, rdr, nil, table.WithOverwriteFilter(filter)); err != nil {
			return fmt.Errorf("%s: %w", mode, err)
		}
		if *errPtr != nil {
			return fmt.Errorf("read mutation buffer: %w", *errPtr)
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

// chunkBatches returns an iterator yielding one Arrow record batch per buffer
// chunk. Any streaming/parse error is surfaced through the returned pointer
// (the iter.Seq2 stops on the first error). Caller checks *err after the reader
// drains.
func chunkBatches(schema *arrow.Schema, mem memory.Allocator, rb *recordBuf, jsonCols map[string]bool) (iter.Seq2[arrow.RecordBatch, error], *error) {
	var streamErr error
	seq := func(yield func(arrow.RecordBatch, error) bool) {
		streamErr = rb.stream(commitRowsPerChunk, func(recs []json.RawMessage) error {
			recs, err := encodeJSONColumns(recs, jsonCols)
			if err != nil {
				return fmt.Errorf("parse chunk: %w", err)
			}
			rec, _, err := array.RecordFromJSON(mem, schema, strings.NewReader(jsonArray(recs)))
			if err != nil {
				return fmt.Errorf("parse chunk: %w", err)
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

// jsonArray frames a chunk of pre-validated JSON objects as one array literal,
// the form RecordFromJSON expects.
func jsonArray(recs []json.RawMessage) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, r := range recs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(r)
	}
	b.WriteByte(']')
	return b.String()
}
