package arrowbatch

import (
	"errors"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/rowmodel"
)

type collect struct {
	chunks  []*Batch
	drained []int
	fail    error
}

func (c *collect) Chunk(ch *Batch) error {
	if c.fail != nil {
		return c.fail
	}
	c.chunks = append(c.chunks, ch)
	return nil
}

func (c *collect) Drained(_ rowmodel.Meta, rows int) error {
	c.drained = append(c.drained, rows)
	return nil
}

func testSchema() rowmodel.Schema {
	return rowmodel.Schema{Resource: "t", Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
		{Name: "amount", Logical: rowmodel.LogicalDecimal, Precision: 12, Scale: 2, Nullable: true},
		{Name: "at", Logical: rowmodel.LogicalTimestampTZ, Nullable: true},
	}}
}

func TestBuilderFlushesOnMaxRows(t *testing.T) {
	c := &collect{}
	b := NewBuilder(Schema(testSchema()), memory.DefaultAllocator, Options{MaxRows: 2}, c)
	for i := range 5 {
		b.Int64(int64(i))
		if i%2 == 0 {
			b.String("n")
		} else {
			b.Null()
		}
		b.Decimal(decimal128.FromI64(int64(i * 100)))
		b.Timestamp(int64(i))
		if err := b.EndRow(rowmodel.Meta{Op: rowmodel.OpInsert}); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(c.chunks))
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(c.chunks) != 3 || c.chunks[2].Rows().NumRows() != 1 {
		t.Fatalf("after flush: %d chunks", len(c.chunks))
	}
	ids := c.chunks[1].Rows().Column(0).(*array.Int64)
	if ids.Value(0) != 2 || ids.Value(1) != 3 {
		t.Fatalf("chunk 1 ids = %v", ids)
	}
	names := c.chunks[1].Rows().Column(1).(*array.String)
	if !names.IsNull(1) || names.Value(0) != "n" {
		t.Fatalf("names = %v", names)
	}
	for _, ch := range c.chunks {
		if ch.Operations().Len() != 0 {
			t.Fatalf("insert-only chunk has ops %v", ch.Operations())
		}
	}
	// Flush with nothing buffered emits nothing.
	if err := b.Flush(); err != nil || len(c.chunks) != 3 {
		t.Fatalf("empty flush: err=%v chunks=%d", err, len(c.chunks))
	}
}

func TestBuilderOpsAndLastMeta(t *testing.T) {
	c := &collect{}
	b := NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 10}, c)
	row := func(op rowmodel.Operation, key string) {
		b.Int64(1)
		b.Null()
		b.Null()
		b.Null()
		if err := b.EndRow(rowmodel.Meta{Op: op, Key: []string{key}}); err != nil {
			t.Fatal(err)
		}
	}
	row(rowmodel.OpInsert, "a")
	row(rowmodel.OpUpdate, "b")
	row(rowmodel.OpInsert, "c")
	if err := b.Drain(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	if len(c.chunks) != 1 || len(c.drained) != 1 || c.drained[0] != 3 {
		t.Fatalf("chunks=%d drained=%v", len(c.chunks), c.drained)
	}
	ch := c.chunks[0]
	want := []rowmodel.Operation{rowmodel.OpInsert, rowmodel.OpUpdate, rowmodel.OpInsert}
	ops := ch.Operations()
	if ops.Len() != 3 || ops.At(0) != want[0] || ops.At(1) != want[1] || ops.At(2) != want[2] {
		t.Fatalf("ops = %v", ops)
	}
	if len(ch.Last.Key) != 1 || ch.Last.Key[0] != "c" {
		t.Fatalf("last = %+v", ch.Last)
	}
}

func TestBuilderMaxBytes(t *testing.T) {
	c := &collect{}
	b := NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 100, MaxBytes: 40}, c)
	for i := range 4 {
		b.Int64(int64(i))
		b.String("0123456789") // 14 bytes counted + 8 for id + 0 nulls = 22/row
		b.Null()
		b.Null()
		if err := b.EndRow(rowmodel.Meta{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.chunks) != 2 || c.chunks[0].Rows().NumRows() != 2 {
		t.Fatalf("chunks = %d", len(c.chunks))
	}
}

func TestBuilderRequestFlush(t *testing.T) {
	c := &collect{}
	b := NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 100}, c)
	b.Int64(1)
	b.Null()
	b.Null()
	b.RequestFlush() // lands after this row closes, never inside it
	b.Null()
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	if len(c.chunks) != 1 || c.chunks[0].Rows().NumRows() != 1 {
		t.Fatalf("chunks = %d", len(c.chunks))
	}
}

func TestBuilderErrors(t *testing.T) {
	c := &collect{}
	b := NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 100}, c)
	b.Int64(1)
	b.Int64(2) // wrong type for "name"
	if err := b.EndRow(rowmodel.Meta{}); err == nil {
		t.Fatal("type mismatch not reported")
	}
	if err := b.Flush(); err == nil {
		t.Fatal("error not sticky")
	}

	b = NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 100}, c)
	b.Int64(1)
	if err := b.EndRow(rowmodel.Meta{}); err == nil {
		t.Fatal("short row not reported")
	}

	b = NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 100}, c)
	b.Int64(1)
	if err := b.Flush(); !errors.Is(err, errMidRow) {
		t.Fatalf("mid-row flush: %v", err)
	}

	c = &collect{fail: errors.New("closed")}
	b = NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 1}, c)
	b.Int64(1)
	b.Null()
	b.Null()
	b.Null()
	if err := b.EndRow(rowmodel.Meta{}); err == nil || err.Error() != "closed" {
		t.Fatalf("receiver error not surfaced: %v", err)
	}
}
