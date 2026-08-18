package batch

import (
	"errors"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament"
)

type collect struct {
	chunks  []Chunk
	drained []int
	fail    error
}

func (c *collect) Chunk(ch Chunk) error {
	if c.fail != nil {
		return c.fail
	}
	c.chunks = append(c.chunks, ch)
	return nil
}

func (c *collect) Drained(_ filament.RowMeta, rows int) error {
	c.drained = append(c.drained, rows)
	return nil
}

func testSchema() filament.RecordSchema {
	return filament.RecordSchema{Resource: "t", Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "name", Logical: filament.LogicalString, Nullable: true},
		{Name: "amount", Logical: filament.LogicalDecimal, Precision: 12, Scale: 2, Nullable: true},
		{Name: "at", Logical: filament.LogicalTimestampTZ, Nullable: true},
	}}
}

func TestBuilderFlushesOnMaxRows(t *testing.T) {
	c := &collect{}
	b := New(Schema(testSchema()), Options{MaxRows: 2}, c)
	for i := range 5 {
		b.Int64(int64(i))
		if i%2 == 0 {
			b.String("n")
		} else {
			b.Null()
		}
		b.Decimal(decimal128.FromI64(int64(i * 100)))
		b.Timestamp(int64(i))
		if err := b.EndRow(filament.RowMeta{Op: filament.OpInsert}); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.chunks) != 2 {
		t.Fatalf("chunks = %d, want 2", len(c.chunks))
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	if len(c.chunks) != 3 || c.chunks[2].Rows.NumRows() != 1 {
		t.Fatalf("after flush: %d chunks", len(c.chunks))
	}
	ids := c.chunks[1].Rows.Column(0).(*array.Int64)
	if ids.Value(0) != 2 || ids.Value(1) != 3 {
		t.Fatalf("chunk 1 ids = %v", ids)
	}
	names := c.chunks[1].Rows.Column(1).(*array.String)
	if !names.IsNull(1) || names.Value(0) != "n" {
		t.Fatalf("names = %v", names)
	}
	for _, ch := range c.chunks {
		if ch.Ops != nil {
			t.Fatalf("insert-only chunk has ops %v", ch.Ops)
		}
	}
	// Flush with nothing buffered emits nothing.
	if err := b.Flush(); err != nil || len(c.chunks) != 3 {
		t.Fatalf("empty flush: err=%v chunks=%d", err, len(c.chunks))
	}
}

func TestBuilderOpsAndLastMeta(t *testing.T) {
	c := &collect{}
	b := New(Schema(testSchema()), Options{MaxRows: 10}, c)
	row := func(op filament.Operation, key string) {
		b.Int64(1)
		b.Null()
		b.Null()
		b.Null()
		if err := b.EndRow(filament.RowMeta{Op: op, Key: []string{key}}); err != nil {
			t.Fatal(err)
		}
	}
	row(filament.OpInsert, "a")
	row(filament.OpUpdate, "b")
	row(filament.OpInsert, "c")
	if err := b.Drain(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	if len(c.chunks) != 1 || len(c.drained) != 1 || c.drained[0] != 3 {
		t.Fatalf("chunks=%d drained=%v", len(c.chunks), c.drained)
	}
	ch := c.chunks[0]
	want := []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpInsert}
	if len(ch.Ops) != 3 || ch.Ops[0] != want[0] || ch.Ops[1] != want[1] || ch.Ops[2] != want[2] {
		t.Fatalf("ops = %v", ch.Ops)
	}
	if len(ch.Last.Key) != 1 || ch.Last.Key[0] != "c" {
		t.Fatalf("last = %+v", ch.Last)
	}
}

func TestBuilderMaxBytes(t *testing.T) {
	c := &collect{}
	b := New(Schema(testSchema()), Options{MaxRows: 100, MaxBytes: 40}, c)
	for i := range 4 {
		b.Int64(int64(i))
		b.String("0123456789") // 14 bytes counted + 8 for id + 0 nulls = 22/row
		b.Null()
		b.Null()
		if err := b.EndRow(filament.RowMeta{}); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.chunks) != 2 || c.chunks[0].Rows.NumRows() != 2 {
		t.Fatalf("chunks = %d", len(c.chunks))
	}
}

func TestBuilderRequestFlush(t *testing.T) {
	c := &collect{}
	b := New(Schema(testSchema()), Options{MaxRows: 100}, c)
	b.Int64(1)
	b.Null()
	b.Null()
	b.RequestFlush() // lands after this row closes, never inside it
	b.Null()
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	if len(c.chunks) != 1 || c.chunks[0].Rows.NumRows() != 1 {
		t.Fatalf("chunks = %d", len(c.chunks))
	}
}

func TestBuilderErrors(t *testing.T) {
	c := &collect{}
	b := New(Schema(testSchema()), Options{MaxRows: 100}, c)
	b.Int64(1)
	b.Int64(2) // wrong type for "name"
	if err := b.EndRow(filament.RowMeta{}); err == nil {
		t.Fatal("type mismatch not reported")
	}
	if err := b.Flush(); err == nil {
		t.Fatal("error not sticky")
	}

	b = New(Schema(testSchema()), Options{MaxRows: 100}, c)
	b.Int64(1)
	if err := b.EndRow(filament.RowMeta{}); err == nil {
		t.Fatal("short row not reported")
	}

	b = New(Schema(testSchema()), Options{MaxRows: 100}, c)
	b.Int64(1)
	if err := b.Flush(); !errors.Is(err, errMidRow) {
		t.Fatalf("mid-row flush: %v", err)
	}

	c = &collect{fail: errors.New("closed")}
	b = New(Schema(testSchema()), Options{MaxRows: 1}, c)
	b.Int64(1)
	b.Null()
	b.Null()
	b.Null()
	if err := b.EndRow(filament.RowMeta{}); err == nil || err.Error() != "closed" {
		t.Fatalf("receiver error not surfaced: %v", err)
	}
}

func TestAppendDecimal(t *testing.T) {
	cases := map[string]struct {
		v     decimal128.Num
		scale int
	}{
		"1.50":                 {decimal128.FromI64(150), 2},
		"-0.05":                {decimal128.FromI64(-5), 2},
		"0.00":                 {decimal128.FromI64(0), 2},
		"7":                    {decimal128.FromI64(7), 0},
		"9223372036854775808":  {decimal128.New(0, 1<<63), 0},
		"18446744073709551616": {decimal128.New(1, 0), 0},
	}
	for want, c := range cases {
		if got := string(AppendDecimal(nil, c.v, c.scale)); got != want {
			t.Errorf("got %q want %q", got, want)
		}
	}
}
