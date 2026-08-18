package postgres

import (
	"math"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

func TestTextForms(t *testing.T) {
	cases := []struct {
		fn   func(dst, src []byte) ([]byte, error)
		src  []byte
		want string
	}{
		{textBool, []byte{1}, "t"},
		{textInt4, be32(-7), "-7"},
		{textFloat8, f64(0.1), "0.1"},
		{textFloat8, f64(math.Inf(-1)), "-Infinity"},
		{textNumeric, numericWire(0, -1, 4, 500), "0.0500"},
		{textNumeric, numericWire(numericNeg, 0, 0, 5), "-5"},
		{textNumeric, numericWire(numericNaN, 0, 0), "NaN"},
		{textDate, be32(0), "2000-01-01"},
		{textDate, be32(-730119), "0001-01-01"},
		{textDate, be32(-730120), "0001-12-31 BC"},
		{textDate, be32(math.MaxInt32), "infinity"},
		{textTimestamp, be64(batch.MicrosPerDay + 3661_500000), "2000-01-02 01:01:01.5"},
		{textTimestamptz, be64(0), "2000-01-01 00:00:00+00"},
		{textTime, be64(batch.MicrosPerHour*13 + 5), "13:00:00.000005"},
		{textBytea, []byte{0, 255}, `\x00ff`},
	}
	for _, c := range cases {
		got, err := c.fn(nil, c.src)
		if err != nil {
			t.Errorf("%s: %v", c.want, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("got %q want %q", got, c.want)
		}
	}
}

func TestTextDecode(t *testing.T) {
	rs := filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "amt", Logical: filament.LogicalDecimal, Precision: 10, Scale: 2},
		{Name: "d", Logical: filament.LogicalDate},
		{Name: "at", Logical: filament.LogicalTimestampTZ},
		{Name: "ts", Logical: filament.LogicalTimestamp},
		{Name: "tm", Logical: filament.LogicalTime},
		{Name: "raw", Logical: filament.LogicalBytes},
		{Name: "ok", Logical: filament.LogicalBool},
	}}
	oids := []uint32{pgtype.Int8OID, pgtype.NumericOID, pgtype.DateOID, pgtype.TimestamptzOID, pgtype.TimestampOID, pgtype.TimeOID, pgtype.ByteaOID, pgtype.BoolOID}
	texts := []string{"42", "1.5", "2024-03-01", "2024-03-01 12:00:00.25+02", "0001-01-01 00:00:00 BC", "23:59:59.999999", `\xdead`, "f"}
	c := &collect{}
	b := batch.New(batch.Schema(rs), batch.Options{MaxRows: 10}, c)
	for i, f := range rs.Fields {
		pt, _ := typeFor(oids[i], f)
		if err := pt.fromText(b, []byte(texts[i])); err != nil {
			t.Fatalf("%s: %v", f.Name, err)
		}
	}
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	rows := c.chunks[0].Rows
	if got := rows.Column(1).(*array.Decimal128).Value(0).ToString(2); got != "1.50" {
		t.Errorf("amt = %s", got)
	}
	if got := int32(rows.Column(2).(*array.Date32).Value(0)); got != 19783 {
		t.Errorf("d = %d", got)
	}
	// 12:00:00.25 at +02 is 10:00:00.25 UTC.
	wantAt := (19783*24+10)*batch.MicrosPerHour + 250000
	if got := int64(rows.Column(3).(*array.Timestamp).Value(0)); got != wantAt {
		t.Errorf("at = %d want %d", got, wantAt)
	}
	// 1 BC is proleptic year 0.
	if got := int64(rows.Column(4).(*array.Timestamp).Value(0)); got != batch.DateToDays(0, 1, 1)*batch.MicrosPerDay {
		t.Errorf("ts = %d", got)
	}
	if got := int64(rows.Column(5).(*array.Time64).Value(0)); got != batch.MicrosPerDay-1 {
		t.Errorf("tm = %d", got)
	}
	if got := rows.Column(6).(*array.Binary).Value(0); string(got) != "\xde\xad" {
		t.Errorf("raw = %x", got)
	}
	if rows.Column(7).(*array.Boolean).Value(0) {
		t.Errorf("ok should be false")
	}
}

// TestDecDecimal covers the wire forms Postgres actually sends: trailing zero
// groups stripped (50000.00 is one group at weight 1), fraction groups past the
// scale, and the extremes of a 38-digit column.
func TestDecDecimal(t *testing.T) {
	cases := []struct {
		name  string
		scale int32
		src   []byte
		want  string // decimal128 rendered at scale; "" for null
		bad   bool
	}{
		{"1.50", 2, numericWire(0, 0, 2, 1, 5000), "1.50", false},
		{"50000.00 one group", 2, numericWire(0, 1, 2, 5), "50000.00", false},
		{"12340000.00 two groups", 2, numericWire(0, 1, 2, 1234), "12340000.00", false},
		{"0.0001 scale 4", 4, numericWire(0, -1, 4, 1), "0.0001", false},
		{"0.05 scale 2", 2, numericWire(0, -1, 2, 500), "0.05", false},
		{"-123456.000", 3, numericWire(numericNeg, 1, 3, 12, 3456), "-123456.000", false},
		{"zero", 2, numericWire(0, 0, 2), "0.00", false},
		{"1.5 in scale 4", 4, numericWire(0, 0, 1, 1, 5000), "1.5000", false},
		{"38 digits", 2, numericWire(0, 8, 2, 9999, 9999, 9999, 9999, 9999, 9999, 9999, 9999, 9999, 9900), "999999999999999999999999999999999999.99", false},
		{"NaN is null", 2, numericWire(numericNaN, 0, 0), "", false},
		{"too many fraction digits", 2, numericWire(0, 0, 3, 1, 5050), "", true},
		{"infinity", 2, numericWire(numericPInf, 0, 0), "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rs := filament.RecordSchema{Fields: []filament.SchemaField{{Name: "v", Logical: filament.LogicalDecimal, Precision: 38, Scale: int(c.scale), Nullable: true}}}
			col := &collect{}
			b := batch.New(batch.Schema(rs), batch.Options{MaxRows: 1}, col)
			err := decDecimal(c.scale)(b, c.src)
			if c.bad {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := b.EndRow(filament.RowMeta{}); err != nil {
				t.Fatal(err)
			}
			arr := col.chunks[0].Rows.Column(0).(*array.Decimal128)
			if c.want == "" {
				if !arr.IsNull(0) {
					t.Fatalf("want null, got %s", arr.Value(0).ToString(c.scale))
				}
				return
			}
			if got := arr.Value(0).ToString(c.scale); got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}
