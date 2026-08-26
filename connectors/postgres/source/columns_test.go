package postgres

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

type collect struct{ chunks []*arrowbatch.Batch }

func (c *collect) Chunk(ch *arrowbatch.Batch) error { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(rowmodel.Meta, int) error { return nil }

func be32(v int32) []byte  { return binary.BigEndian.AppendUint32(nil, uint32(v)) }
func be64(v int64) []byte  { return binary.BigEndian.AppendUint64(nil, uint64(v)) }
func f64(v float64) []byte { return binary.BigEndian.AppendUint64(nil, math.Float64bits(v)) }

// numericWire builds the binary numeric wire form.
func numericWire(sign uint16, weight int16, dscale uint16, digits ...uint16) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(len(digits)))
	b = binary.BigEndian.AppendUint16(b, uint16(weight))
	b = binary.BigEndian.AppendUint16(b, sign)
	b = binary.BigEndian.AppendUint16(b, dscale)
	for _, d := range digits {
		b = binary.BigEndian.AppendUint16(b, d)
	}
	return b
}

// numericTypmod packs precision and scale as pg_attribute.atttypmod does.
func numericTypmod(prec, scale int32) int32 { return (prec<<16 | scale) + 4 }

func testColumns() []column {
	return []column{
		{name: "id", native: "bigint", oid: pgtype.Int8OID, typmod: -1},
		{name: "n", native: "smallint", oid: pgtype.Int2OID, typmod: -1, nullable: true},
		{name: "ok", native: "boolean", oid: pgtype.BoolOID, typmod: -1},
		{name: "f", native: "double precision", oid: pgtype.Float8OID, typmod: -1},
		{name: "amt", native: "numeric(12,2)", oid: pgtype.NumericOID, typmod: numericTypmod(12, 2)},
		{name: "big", native: "numeric", oid: pgtype.NumericOID, typmod: -1},
		{name: "s", native: "text", oid: pgtype.TextOID, typmod: -1},
		{name: "u", native: "uuid", oid: pgtype.UUIDOID, typmod: -1},
		{name: "d", native: "date", oid: pgtype.DateOID, typmod: -1},
		{name: "at", native: "timestamp with time zone", oid: pgtype.TimestamptzOID, typmod: -1},
		{name: "doc", native: "jsonb", oid: pgtype.JSONBOID, typmod: -1},
		{name: "tags", native: "text[]", oid: 1009, typmod: -1},
		{name: "raw", native: "bytea", oid: pgtype.ByteaOID, typmod: -1},
	}
}

func TestSchemaOf(t *testing.T) {
	rs := schemaOf("t", testColumns(), []string{"id"})
	if rs.Engine != "postgres" || len(rs.PrimaryKey) != 1 || rs.PrimaryKey[0] != "id" {
		t.Fatalf("schema = %+v", rs)
	}
	want := map[string]rowmodel.LogicalType{
		"id": rowmodel.LogicalInt64, "n": rowmodel.LogicalInt16, "ok": rowmodel.LogicalBool, "f": rowmodel.LogicalFloat64,
		"amt": rowmodel.LogicalDecimal, "big": rowmodel.LogicalDecimal, "s": rowmodel.LogicalString, "u": rowmodel.LogicalUUID,
		"d": rowmodel.LogicalDate, "at": rowmodel.LogicalTimestampTZ, "doc": rowmodel.LogicalJSON, "tags": rowmodel.LogicalArray,
		"raw": rowmodel.LogicalBytes,
	}
	for _, f := range rs.Fields {
		if f.Logical != want[f.Name] {
			t.Errorf("%s logical = %s, want %s", f.Name, f.Logical, want[f.Name])
		}
	}
	amt, big := rs.Fields[4], rs.Fields[5]
	if amt.Precision != 12 || amt.Scale != 2 || big.Precision != 0 {
		t.Fatalf("numeric bounds: amt=%+v big=%+v", amt, big)
	}
	if !rs.Fields[1].Nullable || rs.Fields[0].Nullable {
		t.Fatalf("nullability lost")
	}
}

func TestRowDecoder(t *testing.T) {
	dec, err := newRowDecoder("t", testColumns(), []string{"id"})
	if err != nil {
		t.Fatal(err)
	}
	wantSelect := `t."id", t."n", t."ok", t."f", t."amt", t."big", t."s", t."u", t."d", t."at", t."doc", (t."tags")::text, t."raw"`
	if dec.selectList != wantSelect {
		t.Fatalf("select list\n got %s\nwant %s", dec.selectList, wantSelect)
	}

	c := &collect{}
	b := arrowbatch.NewBuilder(arrowbatch.Schema(dec.schema), nil, arrowbatch.Options{MaxRows: 10}, c)
	uuid := []byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0}
	raw := [][]byte{
		be64(42),
		nil,
		{1},
		f64(1.5),
		numericWire(0, 0, 2, 1, 5000),           // 1.50
		numericWire(numericNeg, 1, 3, 12, 3456), // -123456.000: groups 12|3456, weight 1, dscale 3
		[]byte("héllo"),
		uuid,
		be32(0), // 2000-01-01
		be64(0), // 2000-01-01T00:00:00Z
		append([]byte{1}, `{"a":1}`...),
		[]byte("{a,b}"),
		{0xde, 0xad},
	}
	if err := dec.appendRow(b, raw); err != nil {
		t.Fatal(err)
	}
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	keys, err := dec.texts(raw, dec.pkIdx)
	if err != nil || len(keys) != 1 || keys[0] != "42" {
		t.Fatalf("pk texts = %v, %v", keys, err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	defer c.chunks[0].Release()
	rows := c.chunks[0].Rows()
	if got := rows.Column(0).(*array.Int64).Value(0); got != 42 {
		t.Errorf("id = %d", got)
	}
	if !rows.Column(1).IsNull(0) {
		t.Errorf("n should be null")
	}
	if !rows.Column(2).(*array.Boolean).Value(0) {
		t.Errorf("ok should be true")
	}
	if got := rows.Column(4).(*array.Decimal128).Value(0).ToString(2); got != "1.50" {
		t.Errorf("amt = %s", got)
	}
	if got := rows.Column(5).(*array.String).Value(0); got != "-123456.000" {
		t.Errorf("big = %s", got)
	}
	if got := rows.Column(6).(*array.String).Value(0); got != "héllo" {
		t.Errorf("s = %s", got)
	}
	if got := rows.Column(7).(*array.String).Value(0); got != "12345678-9abc-def0-1234-56789abcdef0" {
		t.Errorf("u = %s", got)
	}
	if got := rows.Column(8).(*array.Date32).Value(0); int32(got) != 10957 {
		t.Errorf("d = %d", got)
	}
	if got := rows.Column(9).(*array.Timestamp).Value(0); int64(got) != 946684800000000 {
		t.Errorf("at = %d", got)
	}
	if got := rows.Column(10).(*array.String).Value(0); got != `{"a":1}` {
		t.Errorf("doc = %s", got)
	}
	if got := rows.Column(11).(*array.String).Value(0); got != "{a,b}" {
		t.Errorf("tags = %s", got)
	}
	if got := rows.Column(12).(*array.Binary).Value(0); string(got) != "\xde\xad" {
		t.Errorf("raw = %x", got)
	}
}
