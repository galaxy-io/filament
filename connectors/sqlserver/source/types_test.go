package sqlserver

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/decimal128"
	mssql "github.com/microsoft/go-mssqldb"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// recordingWriter renders each appended value as text.
type recordingWriter struct{ vals []string }

func (w *recordingWriter) add(v string)      { w.vals = append(w.vals, v) }
func (w *recordingWriter) Null()             { w.add("null") }
func (w *recordingWriter) Bool(v bool)       { w.add(fmt.Sprint("bool:", v)) }
func (w *recordingWriter) Int16(v int16)     { w.add(fmt.Sprint("i16:", v)) }
func (w *recordingWriter) Int32(v int32)     { w.add(fmt.Sprint("i32:", v)) }
func (w *recordingWriter) Int64(v int64)     { w.add(fmt.Sprint("i64:", v)) }
func (w *recordingWriter) Float32(v float32) { w.add(fmt.Sprint("f32:", v)) }
func (w *recordingWriter) Float64(v float64) { w.add(fmt.Sprint("f64:", v)) }
func (w *recordingWriter) Decimal(v decimal128.Num) {
	w.add("dec:" + v.BigInt().String())
}
func (w *recordingWriter) String(v string)            { w.add("str:" + v) }
func (w *recordingWriter) StringBytes(v []byte)       { w.add("str:" + string(v)) }
func (w *recordingWriter) Bytes(v []byte)             { w.add(fmt.Sprintf("bytes:%x", v)) }
func (w *recordingWriter) Date(v int32)               { w.add(fmt.Sprint("date:", v)) }
func (w *recordingWriter) Time(v int64)               { w.add(fmt.Sprint("time:", v)) }
func (w *recordingWriter) Timestamp(v int64)          { w.add(fmt.Sprint("ts:", v)) }
func (w *recordingWriter) EndRow(rowmodel.Meta) error { return nil }
func (w *recordingWriter) Flush() error               { return nil }
func (w *recordingWriter) Drain(rowmodel.Meta) error  { return nil }
func (w *recordingWriter) Close() error               { return nil }

var _ arrowbatch.RowWriter = (*recordingWriter)(nil)

func TestDecoders(t *testing.T) {
	guid := []byte{0xff, 0x19, 0x96, 0x6f, 0x86, 0x8b, 0x11, 0xd0, 0xb4, 0x2d, 0x00, 0xc0, 0x4f, 0xc9, 0x64, 0xff}
	cases := []struct {
		name    string
		col     mssql.RawColumn
		logical rowmodel.LogicalType
		scale   int
		v       []byte
		want    string
	}{
		{"bit", mssql.RawColumn{Type: mssql.TDSBitN}, rowmodel.LogicalBool, 0, []byte{1}, "bool:true"},
		{"tinyint is unsigned", mssql.RawColumn{Type: mssql.TDSIntN}, rowmodel.LogicalInt16, 0, []byte{0xff}, "i16:255"},
		{"smallint", mssql.RawColumn{Type: mssql.TDSIntN}, rowmodel.LogicalInt16, 0, []byte{0xfe, 0xff}, "i16:-2"},
		{"int", mssql.RawColumn{Type: mssql.TDSInt4}, rowmodel.LogicalInt32, 0, []byte{0, 0, 0, 0x80}, "i32:-2147483648"},
		{"bigint", mssql.RawColumn{Type: mssql.TDSIntN}, rowmodel.LogicalInt64, 0, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}, "i64:9223372036854775807"},
		{"real", mssql.RawColumn{Type: mssql.TDSFltN}, rowmodel.LogicalFloat32, 0, []byte{0, 0, 0xc0, 0x3f}, "f32:1.5"},
		{"float", mssql.RawColumn{Type: mssql.TDSFltN}, rowmodel.LogicalFloat64, 0, []byte{0, 0, 0, 0, 0, 0, 0xf8, 0xbf}, "f64:-1.5"},
		{"decimal", mssql.RawColumn{Type: mssql.TDSDecimalN, Scale: 2}, rowmodel.LogicalDecimal, 2, []byte{1, 0x39, 0x30, 0, 0}, "dec:12345"},
		{"negative decimal", mssql.RawColumn{Type: mssql.TDSNumericN, Scale: 2}, rowmodel.LogicalDecimal, 2, []byte{0, 0x39, 0x30, 0, 0}, "dec:-12345"},
		{"money", mssql.RawColumn{Type: mssql.TDSMoneyN}, rowmodel.LogicalDecimal, 4, []byte{0, 0, 0, 0, 0x39, 0x30, 0, 0}, "dec:12345"},
		{"negative money", mssql.RawColumn{Type: mssql.TDSMoneyN}, rowmodel.LogicalDecimal, 4, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, "dec:-1"},
		{"smallmoney", mssql.RawColumn{Type: mssql.TDSMoneyN}, rowmodel.LogicalDecimal, 4, []byte{0xfe, 0xff, 0xff, 0xff}, "dec:-2"},
		{"unix epoch date", mssql.RawColumn{Type: mssql.TDSDateN}, rowmodel.LogicalDate, 0, []byte{0x3a, 0xf9, 0x0a}, "date:0"},
		{"first date", mssql.RawColumn{Type: mssql.TDSDateN}, rowmodel.LogicalDate, 0, []byte{0, 0, 0}, "date:-719162"},
		{"time(7) truncates to micros", mssql.RawColumn{Type: mssql.TDSTimeN, Scale: 7}, rowmodel.LogicalTime, 0, le(10_000_001, 5), "time:1000000"},
		{"time(0)", mssql.RawColumn{Type: mssql.TDSTimeN, Scale: 0}, rowmodel.LogicalTime, 0, le(3661, 3), "time:3661000000"},
		{"datetime2(3)", mssql.RawColumn{Type: mssql.TDSDateTime2N, Scale: 3}, rowmodel.LogicalTimestamp, 0, append(le(1500, 4), 0x3b, 0xf9, 0x0a), "ts:86401500000"},
		{"datetimeoffset is UTC", mssql.RawColumn{Type: mssql.TDSDateTimeOffsetN, Scale: 0}, rowmodel.LogicalTimestampTZ, 0, append(append(le(60, 3), 0x3a, 0xf9, 0x0a), 0x3c, 0x00), "ts:60000000"},
		{"datetime", mssql.RawColumn{Type: mssql.TDSDateTimeN}, rowmodel.LogicalTimestamp, 0, append(le(25567, 4), le(1, 4)...), "ts:3333"},
		{"smalldatetime", mssql.RawColumn{Type: mssql.TDSDateTimeN}, rowmodel.LogicalTimestamp, 0, append(le(25568, 2), le(1, 2)...), "ts:86460000000"},
		{"uniqueidentifier", mssql.RawColumn{Type: mssql.TDSGuid}, rowmodel.LogicalUUID, 0, guid, "str:6F9619FF-8B86-D011-B42D-00C04FC964FF"},
		{"nvarchar", mssql.RawColumn{Type: mssql.TDSNVarChar}, rowmodel.LogicalString, 0, utf16le("héllo 😀"), "str:héllo 😀"},
		{"varchar ascii", mssql.RawColumn{Type: mssql.TDSBigVarChar}, rowmodel.LogicalString, 0, []byte("plain"), "str:plain"},
		{"xml", mssql.RawColumn{Type: mssql.TDSXml}, rowmodel.LogicalString, 0, utf16le("<a/>"), "str:<a/>"},
		{"varbinary", mssql.RawColumn{Type: mssql.TDSBigVarBin}, rowmodel.LogicalBytes, 0, []byte{0xde, 0xad}, "bytes:dead"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := decoderFor(&tc.col, rowmodel.Field{Name: tc.name, Logical: tc.logical, Precision: 38, Scale: tc.scale})
			if err != nil {
				t.Fatal(err)
			}
			w := &recordingWriter{}
			if err := dec(&rowReader{}, w, tc.v); err != nil {
				t.Fatal(err)
			}
			if len(w.vals) != 1 || w.vals[0] != tc.want {
				t.Fatalf("got %v, want %s", w.vals, tc.want)
			}
		})
	}
}

func TestDecoderRejectsMismatch(t *testing.T) {
	for _, tc := range []struct {
		col mssql.RawColumn
		f   rowmodel.Field
	}{
		{mssql.RawColumn{Type: mssql.TDSNVarChar}, rowmodel.Field{Logical: rowmodel.LogicalInt32}},
		{mssql.RawColumn{Type: mssql.TDSDecimalN, Scale: 3}, rowmodel.Field{Logical: rowmodel.LogicalDecimal, Scale: 2}},
		{mssql.RawColumn{Type: mssql.TDSIntN}, rowmodel.Field{Logical: rowmodel.LogicalBytes}},
	} {
		if _, err := decoderFor(&tc.col, tc.f); err == nil {
			t.Errorf("TDS %#x into %s accepted", uint8(tc.col.Type), tc.f.Logical)
		}
	}
}

func TestKeyFormatters(t *testing.T) {
	cases := []struct {
		col  mssql.RawColumn
		v    []byte
		want string
	}{
		{mssql.RawColumn{Type: mssql.TDSIntN}, []byte{0xfe, 0xff, 0xff, 0xff}, "-2"},
		{mssql.RawColumn{Type: mssql.TDSDecimalN, Scale: 2}, []byte{0, 0x39, 0x30, 0, 0}, "-123.45"},
		{mssql.RawColumn{Type: mssql.TDSMoneyN}, []byte{0, 0, 0, 0, 0x39, 0x30, 0, 0}, "1.2345"},
		{mssql.RawColumn{Type: mssql.TDSDateN}, []byte{0, 0, 0}, "0001-01-01"},
		{mssql.RawColumn{Type: mssql.TDSDateTime2N, Scale: 7}, append(le(10_000_001, 5), 0x3a, 0xf9, 0x0a), "1970-01-01T00:00:01.0000001"},
		{mssql.RawColumn{Type: mssql.TDSDateTime2N, Scale: 0}, append(le(86399, 3), 0x3a, 0xf9, 0x0a), "1970-01-01T23:59:59"},
		{mssql.RawColumn{Type: mssql.TDSDateTimeOffsetN, Scale: 3}, append(append(le(1, 4), 0x3a, 0xf9, 0x0a), 0, 0), "1970-01-01T00:00:00.001Z"},
		{mssql.RawColumn{Type: mssql.TDSDateTimeN}, append(le(0, 4), le(1, 4)...), "1900-01-01T00:00:00.003"},
		{mssql.RawColumn{Type: mssql.TDSDateTimeN}, append(le(0, 4), le(2, 4)...), "1900-01-01T00:00:00.007"},
		{mssql.RawColumn{Type: mssql.TDSDateTimeN}, append(le(0, 4), le(25_919_999, 4)...), "1900-01-01T23:59:59.997"},
		{mssql.RawColumn{Type: mssql.TDSTimeN, Scale: 2}, le(4_500, 3), "00:00:45.00"},
		{mssql.RawColumn{Type: mssql.TDSNVarChar}, utf16le("ключ"), "ключ"},
		{mssql.RawColumn{Type: mssql.TDSBigVarBin}, []byte{0, 1, 2}, "b64:AAEC"},
	}
	for _, tc := range cases {
		f, err := keyFormatterFor(&tc.col)
		if err != nil {
			t.Fatal(err)
		}
		got, err := f(tc.v)
		if err != nil || got != tc.want {
			t.Errorf("TDS %#x %x: got %q, %v; want %q", uint8(tc.col.Type), tc.v, got, err, tc.want)
		}
	}
	if _, err := keyFormatterFor(&mssql.RawColumn{Type: mssql.TDSImage}); err == nil {
		t.Error("image accepted as a checkpoint key")
	}
}

func TestColumnDecl(t *testing.T) {
	for _, tc := range []struct {
		c    column
		want string
	}{
		{column{typeName: "decimal", precision: 15, scale: 2}, "decimal(15,2)"},
		{column{typeName: "nvarchar", maxLength: 80}, "nvarchar(40)"},
		{column{typeName: "varbinary", maxLength: -1}, "varbinary(max)"},
		{column{typeName: "datetime2", scale: 7}, "datetime2(7)"},
		{column{typeName: "real"}, "real"},
		{column{typeName: "float", precision: 53}, "float"},
		{column{typeName: "int"}, "int"},
	} {
		if got := tc.c.decl(); got != tc.want {
			t.Errorf("decl(%+v) = %s, want %s", tc.c, got, tc.want)
		}
	}
}

func le(v uint64, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(v >> (8 * i))
	}
	return b
}

func utf16le(s string) []byte {
	var b []byte
	for _, r := range s {
		if r > 0xffff {
			r -= 0x10000
			hi, lo := 0xd800+(r>>10), 0xdc00+(r&0x3ff)
			b = append(b, byte(hi), byte(hi>>8), byte(lo), byte(lo>>8))
			continue
		}
		b = append(b, byte(r), byte(r>>8))
	}
	return b
}

func TestIntDecoderRejectsAWiderValue(t *testing.T) {
	for _, tc := range []struct {
		logical rowmodel.LogicalType
		v       []byte
	}{
		{rowmodel.LogicalInt16, []byte{0x00, 0x80, 0, 0}},                               // int 32768
		{rowmodel.LogicalInt32, []byte{0, 0, 0, 0x80, 0, 0, 0, 0}},                      // bigint 2^31
		{rowmodel.LogicalInt32, []byte{0xff, 0xff, 0xff, 0x7f, 0xff, 0xff, 0xff, 0xff}}, // bigint -2^31-1
	} {
		w := &recordingWriter{}
		if err := intDecoder(tc.logical)(nil, w, tc.v); err == nil {
			t.Errorf("%s accepted %x as %v", tc.logical, tc.v, w.vals)
		}
	}
}

func TestFloat32DecoderRejectsAWiderValue(t *testing.T) {
	dec := floatDecoder(rowmodel.LogicalFloat32)
	for _, v := range []float64{1.5, 3.4e38, math.Inf(1)} {
		w := &recordingWriter{}
		if err := dec(nil, w, binary.LittleEndian.AppendUint64(nil, math.Float64bits(v))); err != nil {
			t.Errorf("%g rejected: %v", v, err)
		}
	}
	if err := dec(nil, &recordingWriter{}, binary.LittleEndian.AppendUint64(nil, math.Float64bits(1e100))); err == nil {
		t.Error("1e100 accepted as real")
	}
}

func TestKeyFormatterRejectsLoneSurrogates(t *testing.T) {
	f, err := keyFormatterFor(&mssql.RawColumn{Type: mssql.TDSNVarChar})
	if err != nil {
		t.Fatal(err)
	}
	utf16le := func(units ...uint16) []byte {
		var b []byte
		for _, u := range units {
			b = binary.LittleEndian.AppendUint16(b, u)
		}
		return b
	}
	if got, err := f(utf16le('a', 0xd83d, 0xde00)); err != nil || got != "a😀" {
		t.Fatalf("pair = %q, %v", got, err)
	}
	for _, v := range [][]byte{utf16le('a', 0xd83d), utf16le(0xde00, 'a'), utf16le(0xd83d, 'a')} {
		if _, err := f(v); err == nil {
			t.Errorf("%x accepted", v)
		}
	}
}

func TestDecimalDecodersRejectAWiderColumn(t *testing.T) {
	field := func(precision, scale int) rowmodel.Field {
		return rowmodel.Field{Name: "d", Logical: rowmodel.LogicalDecimal, Precision: precision, Scale: scale}
	}
	for _, tc := range []struct {
		name  string
		col   mssql.RawColumn
		field rowmodel.Field
		ok    bool
	}{
		{"decimal as declared", mssql.RawColumn{Type: mssql.TDSDecimalN, Precision: 5, Scale: 2}, field(5, 2), true},
		{"decimal widened", mssql.RawColumn{Type: mssql.TDSDecimalN, Precision: 10, Scale: 2}, field(5, 2), false},
		{"money", mssql.RawColumn{Type: mssql.TDSMoneyN, Size: 8}, field(19, 4), true},
		{"smallmoney", mssql.RawColumn{Type: mssql.TDSMoneyN, Size: 4}, field(10, 4), true},
		{"smallmoney widened to money", mssql.RawColumn{Type: mssql.TDSMoneyN, Size: 8}, field(10, 4), false},
		{"decimal changed to money", mssql.RawColumn{Type: mssql.TDSMoneyN, Size: 8}, field(10, 2), false},
	} {
		if _, err := decoderFor(&tc.col, tc.field); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want accepted %v", tc.name, err, tc.ok)
		}
	}
}
