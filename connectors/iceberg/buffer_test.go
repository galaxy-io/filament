package iceberg

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestRecordBufOwnsInMemoryBatchUntilClose(t *testing.T) {
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	b := testBatch(t, alloc)
	rb := newRecordBuf(1 << 20)

	if err := rb.append(b, arrowbatch.Bytes(b.Rows())); err != nil {
		t.Fatal(err)
	}
	b.Release() // release the pipeline's reference; recordBuf still owns one

	if err := rb.stream(func(rows arrow.RecordBatch, ops []rowmodel.Operation) error {
		if rows.NumRows() != 1 || len(ops) != 1 || ops[0] != rowmodel.OpUpdate {
			t.Fatalf("streamed rows=%d ops=%v", rows.NumRows(), ops)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rb.close()
	if b.NumRows() != 0 {
		t.Fatal("buffer close did not release its batch reference")
	}
	alloc.AssertSize(t, 0)
}

func TestRecordBufSpillDoesNotDependOnBatchLifetime(t *testing.T) {
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	b := testBatch(t, alloc)
	rb := newRecordBuf(0) // any non-empty batch spills immediately

	if err := rb.append(b, arrowbatch.Bytes(b.Rows())); err != nil {
		t.Fatal(err)
	}
	b.Release()
	if b.NumRows() != 0 {
		t.Fatal("spilled buffer unexpectedly retained the source batch")
	}

	if err := rb.stream(func(rows arrow.RecordBatch, ops []rowmodel.Operation) error {
		if rows.NumRows() != 1 || len(ops) != 1 || ops[0] != rowmodel.OpUpdate {
			t.Fatalf("streamed rows=%d ops=%v", rows.NumRows(), ops)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	rb.close()
	alloc.AssertSize(t, 0)
}

func testBatch(t *testing.T, alloc memory.Allocator) *arrowbatch.Batch {
	t.Helper()
	builder := array.NewInt64Builder(alloc)
	builder.Append(42)
	values := builder.NewArray()
	builder.Release()

	schema := arrow.NewSchema([]arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int64}}, nil)
	record := array.NewRecordBatch(schema, []arrow.Array{values}, 1)
	values.Release()
	return arrowbatch.NewBatch(record, arrowbatch.NewOperations([]rowmodel.Operation{rowmodel.OpUpdate}))
}
