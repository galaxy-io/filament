// Package ndjson renders Arrow rows as JSON objects, one per line, for the sinks
// that write text (stdout, object stores).
package ndjson

import (
	"encoding/base64"
	"math"
	"strconv"
	"unicode/utf8"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

// Encoder renders rows of one Arrow schema. Column keys are pre-escaped once.
type Encoder struct {
	keys []string // `"name":` per column
	vals []valueFn
}

type valueFn func(dst []byte, col arrow.Array, i int) []byte

// NewEncoder prepares an encoder for schema.
func NewEncoder(schema *arrow.Schema) *Encoder {
	e := &Encoder{keys: make([]string, schema.NumFields()), vals: make([]valueFn, schema.NumFields())}
	for i, f := range schema.Fields() {
		e.keys[i] = string(appendString(nil, f.Name)) + ":"
		e.vals[i] = valueFor(f)
	}
	return e
}

// AppendRow appends row i of rows as one JSON object, without a trailing newline.
func (e *Encoder) AppendRow(dst []byte, rows arrow.RecordBatch, i int) []byte {
	dst = append(dst, '{')
	for c, col := range rows.Columns() {
		if c > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, e.keys[c]...)
		if col.IsNull(i) {
			dst = append(dst, "null"...)
			continue
		}
		dst = e.vals[c](dst, col, i)
	}
	return append(dst, '}')
}

// valueFor picks the renderer for a field from its Arrow storage type and the
// logical type it carries. json columns are JSON text and embed raw; bytes are
// base64; dates and times are ISO 8601, with the int32/int64 extremes spelled
// infinity as Postgres does.
func valueFor(f arrow.Field) valueFn {
	switch f.Type.ID() {
	case arrow.BOOL:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return strconv.AppendBool(dst, col.(*array.Boolean).Value(i))
		}
	case arrow.INT16:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return strconv.AppendInt(dst, int64(col.(*array.Int16).Value(i)), 10)
		}
	case arrow.INT32:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return strconv.AppendInt(dst, int64(col.(*array.Int32).Value(i)), 10)
		}
	case arrow.INT64:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return strconv.AppendInt(dst, col.(*array.Int64).Value(i), 10)
		}
	case arrow.FLOAT32:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendFloat(dst, float64(col.(*array.Float32).Value(i)), 32)
		}
	case arrow.FLOAT64:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendFloat(dst, col.(*array.Float64).Value(i), 64)
		}
	case arrow.DECIMAL128:
		scale := f.Type.(*arrow.Decimal128Type).Scale
		return func(dst []byte, col arrow.Array, i int) []byte {
			return append(dst, col.(*array.Decimal128).Value(i).ToString(scale)...)
		}
	case arrow.STRING:
		switch batch.LogicalOf(f) {
		case filament.LogicalJSON:
			return func(dst []byte, col arrow.Array, i int) []byte {
				return append(dst, col.(*array.String).Value(i)...)
			}
		case filament.LogicalDecimal: // unbounded numeric text: a JSON number unless special
			return func(dst []byte, col arrow.Array, i int) []byte {
				v := col.(*array.String).Value(i)
				if v != "" && (v[0] == 'N' || v[0] == 'I' || v[0] == '-' && len(v) > 1 && v[1] == 'I') {
					return appendString(dst, v)
				}
				return append(dst, v...)
			}
		}
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendString(dst, col.(*array.String).Value(i))
		}
	case arrow.BINARY:
		return func(dst []byte, col arrow.Array, i int) []byte {
			dst = append(dst, '"')
			dst = base64.StdEncoding.AppendEncode(dst, col.(*array.Binary).Value(i))
			return append(dst, '"')
		}
	case arrow.DATE32:
		return func(dst []byte, col arrow.Array, i int) []byte {
			dst = append(dst, '"')
			switch v := col.(*array.Date32).Value(i); v {
			case math.MaxInt32:
				dst = append(dst, "infinity"...)
			case math.MinInt32:
				dst = append(dst, "-infinity"...)
			default:
				dst = batch.AppendDate(dst, int64(v))
			}
			return append(dst, '"')
		}
	case arrow.TIME64:
		return func(dst []byte, col arrow.Array, i int) []byte {
			dst = append(dst, '"')
			dst = batch.AppendTimeOfDay(dst, int64(col.(*array.Time64).Value(i)))
			return append(dst, '"')
		}
	case arrow.TIMESTAMP:
		utc := f.Type.(*arrow.TimestampType).TimeZone != ""
		return func(dst []byte, col arrow.Array, i int) []byte {
			dst = append(dst, '"')
			switch v := int64(col.(*array.Timestamp).Value(i)); v {
			case math.MaxInt64:
				dst = append(dst, "infinity"...)
			case math.MinInt64:
				dst = append(dst, "-infinity"...)
			default:
				dst = batch.AppendTimestamp(dst, v, 'T')
				if utc {
					dst = append(dst, 'Z')
				}
			}
			return append(dst, '"')
		}
	default:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendString(dst, col.ValueStr(i))
		}
	}
}

// appendFloat renders a float as a JSON number, or as the strings "NaN",
// "Infinity", "-Infinity", which JSON cannot spell as numbers.
func appendFloat(dst []byte, v float64, bits int) []byte {
	switch {
	case math.IsNaN(v):
		return append(dst, `"NaN"`...)
	case math.IsInf(v, 1):
		return append(dst, `"Infinity"`...)
	case math.IsInf(v, -1):
		return append(dst, `"-Infinity"`...)
	}
	return strconv.AppendFloat(dst, v, 'g', -1, bits)
}

const hexDigits = "0123456789abcdef"

// appendString appends s as a JSON string literal; invalid UTF-8 bytes become
// U+FFFD.
func appendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		b := s[i]
		if b >= 0x20 && b != '"' && b != '\\' && b < utf8.RuneSelf {
			i++
			continue
		}
		if b >= utf8.RuneSelf {
			r, size := utf8.DecodeRuneInString(s[i:])
			if r != utf8.RuneError || size != 1 {
				i += size
				continue
			}
			dst = append(dst, s[start:i]...)
			dst = utf8.AppendRune(dst, utf8.RuneError)
			i++
			start = i
			continue
		}
		dst = append(dst, s[start:i]...)
		switch b {
		case '"', '\\':
			dst = append(dst, '\\', b)
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, '\\', 'u', '0', '0', hexDigits[b>>4], hexDigits[b&0xF])
		}
		i++
		start = i
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}
