package arrowbatch

import (
	"errors"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/rowmodel"
)

func TestBatchReleasesArrowMemoryOnLastReference(t *testing.T) {
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	recv := &collect{}
	builder := NewBuilder(Schema(rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}}), alloc, Options{MaxRows: 1}, recv)
	builder.Int64(1)
	if err := builder.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	batch := recv.chunks[0]
	batch.Retain()
	batch.Release()
	if batch.Rows() == nil {
		t.Fatal("first release freed a retained batch")
	}
	batch.Release()
	if batch.Rows() != nil {
		t.Fatal("last release did not clear borrowed rows")
	}
	alloc.AssertSize(t, 0)
}

func TestBuilderReleasesRejectedBatch(t *testing.T) {
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	recv := &collect{fail: errors.New("reject")}
	builder := NewBuilder(Schema(rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}}), alloc, Options{MaxRows: 1}, recv)
	builder.Int64(1)
	if err := builder.EndRow(rowmodel.Meta{}); err == nil {
		t.Fatal("rejected handoff returned nil")
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	alloc.AssertSize(t, 0)
}

func TestBuilderCloseIsIdempotent(t *testing.T) {
	builder := NewBuilder(Schema(rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}}), nil, Options{MaxRows: 1}, &collect{})
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	builder.Int64(1)
	if err := builder.EndRow(rowmodel.Meta{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("EndRow after Close = %v, want ErrClosed", err)
	}
}
