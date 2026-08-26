package postgres

// Every value rendering the sink knows, in one place: for each Arrow storage
// type, its binary send form for the destination Postgres type (when one
// exists) and its COPY text form (which every type accepts).

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament/internal/arrowtext"
)

// baseType normalizes a Postgres type spelling to the name its send function is
// keyed by: lowercased, modifiers dropped, synonyms folded. Arrays keep their
// "[]" so they never match a scalar.
func baseType(pgType string) string {
	t := strings.ToLower(strings.TrimSpace(pgType))
	if i := strings.IndexByte(t, '('); i >= 0 {
		end := strings.IndexByte(t[i:], ')')
		if end < 0 {
			return t
		}
		t = strings.TrimSpace(t[:i] + t[i+end+1:])
	}
	switch t {
	case "int2":
		return "smallint"
	case "int4", "int":
		return "integer"
	case "int8":
		return "bigint"
	case "float4":
		return "real"
	case "float8":
		return "double precision"
	case "bool":
		return "boolean"
	case "character varying":
		return "varchar"
	case "character", "char":
		return "bpchar"
	case "timestamp without time zone":
		return "timestamp"
	case "timestamp with time zone":
		return "timestamptz"
	case "time without time zone":
		return "time"
	case "decimal":
		return "numeric"
	}
	return t
}

// Postgres counts dates from 2000-01-01 and Arrow from 1970-01-01. The int32 and
// int64 extremes are Postgres infinity and pass through unshifted.
const (
	pgEpochDays   = 10957
	pgEpochMicros = pgEpochDays * arrowtext.MicrosPerDay
)

// binaryFor picks the binary send renderer for an Arrow field landing in a
// column of pgType, or nil when there is none.
func binaryFor(f arrow.Field, pgType string) valueFn {
	pgt := baseType(pgType)
	switch f.Type.ID() {
	case arrow.BOOL:
		if pgt == "boolean" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				if col.(*array.Boolean).Value(i) {
					return append(dst, 1)
				}
				return append(dst, 0)
			}
		}
	case arrow.INT16:
		if pgt == "smallint" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return binary.BigEndian.AppendUint16(dst, uint16(col.(*array.Int16).Value(i))) //nolint:gosec // wire
			}
		}
	case arrow.INT32:
		if pgt == "integer" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return binary.BigEndian.AppendUint32(dst, uint32(col.(*array.Int32).Value(i))) //nolint:gosec // wire
			}
		}
	case arrow.INT64:
		if pgt == "bigint" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return binary.BigEndian.AppendUint64(dst, uint64(col.(*array.Int64).Value(i))) //nolint:gosec // wire
			}
		}
	case arrow.FLOAT32:
		if pgt == "real" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return binary.BigEndian.AppendUint32(dst, math.Float32bits(col.(*array.Float32).Value(i)))
			}
		}
	case arrow.FLOAT64:
		if pgt == "double precision" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return binary.BigEndian.AppendUint64(dst, math.Float64bits(col.(*array.Float64).Value(i)))
			}
		}
	case arrow.DECIMAL128:
		if pgt == "numeric" {
			scale := int(f.Type.(*arrow.Decimal128Type).Scale)
			return func(dst []byte, col arrow.Array, i int) []byte {
				return appendDecimal(dst, col.(*array.Decimal128).Value(i), scale)
			}
		}
	case arrow.STRING:
		return binaryString(pgt)
	case arrow.BINARY:
		if pgt == "bytea" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return append(dst, col.(*array.Binary).Value(i)...)
			}
		}
	case arrow.DATE32:
		if pgt == "date" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				v := col.(*array.Date32).Value(i)
				if v != math.MaxInt32 && v != math.MinInt32 {
					v -= pgEpochDays
				}
				return binary.BigEndian.AppendUint32(dst, uint32(v)) //nolint:gosec // wire
			}
		}
	case arrow.TIME64:
		if pgt == "time" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				return binary.BigEndian.AppendUint64(dst, uint64(col.(*array.Time64).Value(i))) //nolint:gosec // wire
			}
		}
	case arrow.TIMESTAMP:
		if pgt == "timestamp" || pgt == "timestamptz" {
			return func(dst []byte, col arrow.Array, i int) []byte {
				v := int64(col.(*array.Timestamp).Value(i))
				if v != math.MaxInt64 && v != math.MinInt64 {
					v -= pgEpochMicros
				}
				return binary.BigEndian.AppendUint64(dst, uint64(v)) //nolint:gosec // wire
			}
		}
	}
	return nil
}

// binaryString picks the binary send renderer for a utf8 column by destination
// type: text-like types take the bytes, jsonb adds its version byte, uuid and
// unbounded numeric parse the text into their wire forms.
func binaryString(pgt string) valueFn {
	switch pgt {
	case "text", "varchar", "bpchar", "name", "json", "xml":
		return func(dst []byte, col arrow.Array, i int) []byte {
			return append(dst, col.(*array.String).Value(i)...)
		}
	case "jsonb":
		return func(dst []byte, col arrow.Array, i int) []byte {
			return append(append(dst, 1), col.(*array.String).Value(i)...)
		}
	case "uuid":
		return func(dst []byte, col arrow.Array, i int) []byte {
			u, err := uuid.Parse(col.(*array.String).Value(i))
			if err != nil {
				// Malformed: hand the text through so the server reports it.
				return append(dst, col.(*array.String).Value(i)...)
			}
			return append(dst, u[:]...)
		}
	case "numeric":
		return func(dst []byte, col arrow.Array, i int) []byte {
			out, err := appendNumericText(dst, col.(*array.String).Value(i))
			if err != nil {
				return append(dst, col.(*array.String).Value(i)...)
			}
			return out
		}
	}
	return nil
}

// textFor picks the COPY text renderer for an Arrow field: the input form every
// Postgres type accepts, escaped for the text format.
func textFor(f arrow.Field) valueFn {
	switch f.Type.ID() {
	case arrow.BOOL:
		return func(dst []byte, col arrow.Array, i int) []byte {
			if col.(*array.Boolean).Value(i) {
				return append(dst, 't')
			}
			return append(dst, 'f')
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
			return appendFloatText(dst, float64(col.(*array.Float32).Value(i)), 32)
		}
	case arrow.FLOAT64:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendFloatText(dst, col.(*array.Float64).Value(i), 64)
		}
	case arrow.DECIMAL128:
		scale := int(f.Type.(*arrow.Decimal128Type).Scale)
		return func(dst []byte, col arrow.Array, i int) []byte {
			return arrowtext.AppendDecimal(dst, col.(*array.Decimal128).Value(i), scale)
		}
	case arrow.STRING:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendEscaped(dst, col.(*array.String).Value(i))
		}
	case arrow.BINARY:
		return func(dst []byte, col arrow.Array, i int) []byte {
			dst = append(dst, '\\', '\\', 'x')
			return hex.AppendEncode(dst, col.(*array.Binary).Value(i))
		}
	case arrow.DATE32:
		return func(dst []byte, col arrow.Array, i int) []byte {
			switch v := col.(*array.Date32).Value(i); v {
			case math.MaxInt32:
				return append(dst, "infinity"...)
			case math.MinInt32:
				return append(dst, "-infinity"...)
			default:
				return arrowtext.AppendDate32(dst, int32(v))
			}
		}
	case arrow.TIME64:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return arrowtext.AppendTimeOfDay(dst, int64(col.(*array.Time64).Value(i)))
		}
	case arrow.TIMESTAMP:
		utc := f.Type.(*arrow.TimestampType).TimeZone != ""
		return func(dst []byte, col arrow.Array, i int) []byte {
			switch v := int64(col.(*array.Timestamp).Value(i)); v {
			case math.MaxInt64:
				return append(dst, "infinity"...)
			case math.MinInt64:
				return append(dst, "-infinity"...)
			default:
				dst = arrowtext.AppendTimestamp(dst, v, ' ')
				if utc {
					dst = append(dst, "+00"...)
				}
				return dst
			}
		}
	default:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendEscaped(dst, col.ValueStr(i))
		}
	}
}

// appendFloatText renders a float shortest-round-trip, with the special values
// in the spellings float8in accepts.
func appendFloatText(dst []byte, v float64, bitSize int) []byte {
	switch {
	case math.IsNaN(v):
		return append(dst, "NaN"...)
	case math.IsInf(v, 1):
		return append(dst, "Infinity"...)
	case math.IsInf(v, -1):
		return append(dst, "-Infinity"...)
	}
	return strconv.AppendFloat(dst, v, 'g', -1, bitSize)
}

// appendEscaped appends s in COPY text form: backslash, tab, newline and
// carriage return escaped.
func appendEscaped(dst []byte, s string) []byte {
	start := 0
	for i := range len(s) {
		var esc byte
		switch s[i] {
		case '\\':
			esc = '\\'
		case '\t':
			esc = 't'
		case '\n':
			esc = 'n'
		case '\r':
			esc = 'r'
		default:
			continue
		}
		dst = append(dst, s[start:i]...)
		dst = append(dst, '\\', esc)
		start = i + 1
	}
	return append(dst, s[start:]...)
}

// numeric wire form: int16 ndigits, weight, sign, dscale, then ndigits
// base-10000 groups, most significant first, with weight the exponent of the
// first group. Built from a decimal128 (bounded numerics) or from numeric_out
// text (unbounded ones) without going through strings or big.Int.

const (
	numericPos  = 0x0000
	numericNeg  = 0x4000
	numericNaN  = 0xC000
	numericPInf = 0xD000
	numericNInf = 0xF000
)

// appendDecimal appends the wire form of v at scale, without a length prefix.
func appendDecimal(dst []byte, v decimal128.Num, scale int) []byte {
	neg := v.Sign() < 0
	if neg {
		v = v.Negate()
	}
	var buf [40]byte
	digits := arrowtext.DecimalDigits(buf[:], uint64(v.HighBits()), v.LowBits()) //nolint:gosec // magnitude, sign handled above
	return appendNumeric(dst, neg, digits, scale)
}

// appendNumericText appends the wire form of numeric_out text: an optional
// sign, digits, an optional fraction, or NaN/Infinity/-Infinity.
func appendNumericText(dst []byte, text string) ([]byte, error) {
	switch text {
	case "NaN":
		return appendNumericSpecial(dst, numericNaN), nil
	case "Infinity":
		return appendNumericSpecial(dst, numericPInf), nil
	case "-Infinity":
		return appendNumericSpecial(dst, numericNInf), nil
	}
	neg := false
	if text != "" && (text[0] == '-' || text[0] == '+') {
		neg = text[0] == '-'
		text = text[1:]
	}
	if text == "" {
		return nil, fmt.Errorf("numeric %q: no digits", text)
	}
	digits := make([]byte, 0, len(text))
	scale, dot := 0, -1
	for i := range len(text) {
		switch c := text[i]; {
		case c >= '0' && c <= '9':
			digits = append(digits, c)
		case c == '.' && dot < 0:
			dot = i
		default:
			return nil, fmt.Errorf("numeric %q: bad character %q", text, c)
		}
	}
	if dot >= 0 {
		scale = len(text) - dot - 1
	}
	return appendNumeric(dst, neg, digits, scale), nil
}

func appendNumericSpecial(dst []byte, sign uint16) []byte {
	dst = binary.BigEndian.AppendUint16(dst, 0)
	dst = binary.BigEndian.AppendUint16(dst, 0)
	dst = binary.BigEndian.AppendUint16(dst, sign)
	return binary.BigEndian.AppendUint16(dst, 0)
}

// appendNumeric appends the wire form of the magnitude digits with the decimal
// point scale places from the right (fewer digits than scale means leading
// fraction zeros), negated by neg, at dscale scale. Groups are written straight
// into dst and normalized in place: no leading or trailing zero groups, zero is
// ndigits 0.
func appendNumeric(dst []byte, neg bool, digits []byte, scale int) []byte {
	for len(digits) > scale && digits[0] == '0' {
		digits = digits[1:]
	}
	intLen := len(digits) - scale // < 0: that many implicit fraction zeros
	hdr := len(dst)
	dst = append(dst, 0, 0, 0, 0, 0, 0, 0, 0)
	weight := -1
	if intLen > 0 {
		first := intLen % 4 // the leading group is short
		if first == 0 {
			first = 4
		}
		dst = binary.BigEndian.AppendUint16(dst, group(digits[:first]))
		for i := first; i < intLen; i += 4 {
			dst = binary.BigEndian.AppendUint16(dst, group(digits[i:i+4]))
		}
		weight = (intLen - 1) / 4
	}
	lead := max(-intLen, 0)
	frac := digits[max(intLen, 0):]
	for pos := 0; pos < scale; pos += 4 { // the last group is right-padded
		var g uint16
		for j := pos; j < pos+4; j++ {
			c := byte('0')
			if j >= lead && j-lead < len(frac) {
				c = frac[j-lead]
			}
			g = g*10 + uint16(c-'0')
		}
		dst = binary.BigEndian.AppendUint16(dst, g)
	}
	groups := dst[hdr+8:]
	start, end := 0, len(groups)/2
	for start < end && groups[2*start] == 0 && groups[2*start+1] == 0 {
		start++
		weight--
	}
	for end > start && groups[2*end-2] == 0 && groups[2*end-1] == 0 {
		end--
	}
	sign := uint16(numericPos)
	if start == end {
		weight = 0
	} else if neg {
		sign = numericNeg
	}
	copy(groups, groups[2*start:2*end])
	dst = dst[:hdr+8+2*(end-start)]
	binary.BigEndian.PutUint16(dst[hdr:], uint16(end-start)) //nolint:gosec // group count
	binary.BigEndian.PutUint16(dst[hdr+2:], uint16(weight))  //nolint:gosec // two's complement int16
	binary.BigEndian.PutUint16(dst[hdr+4:], sign)
	binary.BigEndian.PutUint16(dst[hdr+6:], uint16(scale)) //nolint:gosec // dscale
	return dst
}

func group(d []byte) uint16 {
	var g uint16
	for _, c := range d {
		g = g*10 + uint16(c-'0')
	}
	return g
}
