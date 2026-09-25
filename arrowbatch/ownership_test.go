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

func TestBuilderOwnsRetainedStreamMeta(t *testing.T) {
	recv := &collect{}
	builder := NewBuilder(Schema(rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}}), nil, Options{MaxRows: 10}, recv)
	defer builder.Close()
	meta := rowmodel.Meta{LSN: "bounded", Seq: 7, Stream: &rowmodel.StreamMeta{Identity: rowmodel.EventIdentity{Domain: rowmodel.DomainKey{Incarnation: "i", Domain: "d"}, Position: rowmodel.Position{Codec: "opaque", Value: []byte("p")}, Ordinal: 3}}}
	builder.Int64(1)
	if err := builder.EndRow(meta); err != nil {
		t.Fatal(err)
	}
	// The source may reuse its metadata immediately after EndRow.
	meta.Stream.Identity.Position.Value[0] = 'x'
	meta.Stream.Identity.Ordinal = 9
	if err := builder.Flush(); err != nil {
		t.Fatal(err)
	}
	defer recv.chunks[0].Release()
	got := recv.chunks[0].Last
	if got.Stream == meta.Stream || string(got.Stream.Identity.Position.Value) != "p" || got.Stream.Identity.Ordinal != 3 || got.LSN != "bounded" || got.Seq != 7 {
		t.Fatalf("retained metadata aliases source: %+v", got)
	}
	builder.Int64(2)
	if err := builder.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := builder.Flush(); err != nil {
		t.Fatal(err)
	}
	defer recv.chunks[1].Release()
	if recv.chunks[1].Last.Stream != nil {
		t.Fatal("stream metadata leaked into bounded row")
	}
}
