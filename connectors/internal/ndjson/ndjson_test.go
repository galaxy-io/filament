package ndjson

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

type collect struct{ chunks []batch.Chunk }

func (c *collect) Chunk(ch batch.Chunk) error          { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(filament.RowMeta, int) error { return nil }

func TestAppendRow(t *testing.T) {
	rs := filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "na\"me", Logical: filament.LogicalString, Nullable: true},
		{Name: "doc", Logical: filament.LogicalJSON, Nullable: true},
		{Name: "amt", Logical: filament.LogicalDecimal, Precision: 10, Scale: 2},
		{Name: "f", Logical: filament.LogicalFloat64},
		{Name: "raw", Logical: filament.LogicalBytes},
		{Name: "d", Logical: filament.LogicalDate},
		{Name: "at", Logical: filament.LogicalTimestampTZ},
		{Name: "t", Logical: filament.LogicalTime},
		{Name: "ok", Logical: filament.LogicalBool},
	}}
	schema := batch.Schema(rs)
	c := &collect{}
	b := batch.New(schema, batch.Options{MaxRows: 10}, c)
	b.Int64(7)
	b.String("a\tbé\x01")
	b.String(`{"k":[1,2]}`)
	b.Decimal(decimal128.FromI64(12345))
	b.Float64(math.NaN())
	b.Bytes([]byte{0, 255})
	b.Date(19723) // 2024-01-01
	b.Timestamp(1704067200_000000 + 123456)
	b.Time(3600_000000 + 5)
	b.Bool(true)
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	b.Int64(8)
	b.Null()
	b.Null()
	b.Decimal(decimal128.FromI64(-5))
	b.Float64(1.5)
	b.Bytes(nil)
	b.Date(0)
	b.Timestamp(0)
	b.Time(0)
	b.Bool(false)
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	rows := c.chunks[0].Rows
	enc := NewEncoder(schema)
	got0 := string(enc.AppendRow(nil, rows, 0))
	got1 := string(enc.AppendRow(nil, rows, 1))
	want0 := `{"id":7,"na\"me":"a\tbé\u0001","doc":{"k":[1,2]},"amt":123.45,"f":"NaN","raw":"AP8=","d":"2024-01-01","at":"2024-01-01T00:00:00.123456Z","t":"01:00:00.000005","ok":true}`
	want1 := `{"id":8,"na\"me":null,"doc":null,"amt":-0.05,"f":1.5,"raw":"","d":"1970-01-01","at":"1970-01-01T00:00:00Z","t":"00:00:00","ok":false}`
	if got0 != want0 {
		t.Errorf("row 0\n got %s\nwant %s", got0, want0)
	}
	if got1 != want1 {
		t.Errorf("row 1\n got %s\nwant %s", got1, want1)
	}
	for _, line := range []string{got0, got1} {
		if !json.Valid([]byte(line)) {
			t.Errorf("invalid JSON: %s", line)
		}
	}
}
