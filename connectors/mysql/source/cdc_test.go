package mysql

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type collect struct{ chunks []*arrowbatch.Batch }

func (c *collect) Chunk(ch *arrowbatch.Batch) error { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(rowmodel.Meta, int) error { return nil }

// TestAppendBinlogRowEnumSet feeds enum and set values in the form go-mysql
// decodes them from a row event (the 1-based member index and the member
// bitmap, as int64) and expects the member text the query path reads.
func TestAppendBinlogRowEnumSet(t *testing.T) {
	dec := newRowDecoder("t", []column{
		{name: "size", dataType: "enum", fullType: "enum('small','Medium','it''s')"},
		{name: "tags", dataType: "set", fullType: "set('a','b','c')"},
		{name: "opts", dataType: "set", fullType: "set('','x')"},
	}, nil)
	rows := []struct {
		row              []any
		size, tags, opts string
	}{
		{[]any{int64(2), int64(5), int64(3)}, "Medium", "a,c", "x"}, // no comma after the empty member
		{[]any{int64(3), int64(0), int64(1)}, "it's", "", ""},
		{[]any{int64(0), int64(2), int64(2)}, "", "b", "x"}, // 0 is enum's '' error value
	}
	c := &collect{}
	b := arrowbatch.NewBuilder(arrowbatch.Schema(dec.schema), nil, arrowbatch.Options{MaxRows: len(rows)}, c)
	for _, r := range rows {
		if err := dec.appendBinlogRow(b, r.row); err != nil {
			t.Fatal(err)
		}
		if err := b.EndRow(rowmodel.Meta{}); err != nil {
			t.Fatal(err)
		}
	}
	defer c.chunks[0].Release()
	size := c.chunks[0].Rows().Column(0).(*array.String)
	tags := c.chunks[0].Rows().Column(1).(*array.String)
	opts := c.chunks[0].Rows().Column(2).(*array.String)
	for i, r := range rows {
		if got := size.Value(i); got != r.size {
			t.Errorf("row %d size = %q, want %q", i, got, r.size)
		}
		if got := tags.Value(i); got != r.tags {
			t.Errorf("row %d tags = %q, want %q", i, got, r.tags)
		}
		if got := opts.Value(i); got != r.opts {
			t.Errorf("row %d opts = %q, want %q", i, got, r.opts)
		}
	}
}

// TestAppendBinlogRowBit feeds bit values as go-mysql decodes them (int64) and
// expects the big-endian bytes the query path reads for bit(M).
func TestAppendBinlogRowBit(t *testing.T) {
	dec := newRowDecoder("t", []column{
		{name: "flag", dataType: "bit", fullType: "bit(1)"},
		{name: "mask", dataType: "bit", fullType: "bit(12)"},
	}, nil)
	c := &collect{}
	b := arrowbatch.NewBuilder(arrowbatch.Schema(dec.schema), nil, arrowbatch.Options{MaxRows: 1}, c)
	if err := dec.appendBinlogRow(b, []any{int64(1), int64(0x0a05)}); err != nil {
		t.Fatal(err)
	}
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	defer c.chunks[0].Release()
	rows := c.chunks[0].Rows()
	if got := rows.Column(0).(*array.Binary).Value(0); string(got) != "\x01" {
		t.Errorf("flag = %x, want 01", got)
	}
	if got := rows.Column(1).(*array.Binary).Value(0); string(got) != "\x0a\x05" {
		t.Errorf("mask = %x, want 0a05", got)
	}
}
