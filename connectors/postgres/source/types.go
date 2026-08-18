package postgres

// Every Postgres type this source understands, in one place: how it maps to a
// LogicalType, how a binary wire value (pgx, query reads) appends into a row, how
// that value spells as text (keyset cursor bounds), and how a text-form value
// (pgoutput, CDC) appends into a row. A type absent from the table is read as
// text and travels as utf8.

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

// pgType is one Postgres type's behaviour in the source.
type pgType struct {
	logical    filament.LogicalType
	fromBinary func(w filament.RowWriter, src []byte) error  // binary wire value → row
	toText     func(dst, src []byte) ([]byte, error)         // binary wire value → text form
	fromText   func(w filament.RowWriter, text []byte) error // text-form value → row
}

// typeFor returns the behaviour of a column by type OID, sized to the field
// (numeric needs its scale). wire is false for a type without a binary form:
// it is projected (col)::text, arrives as text and stays text, logically a
// string or an array by its native spelling.
func typeFor(oid uint32, f filament.SchemaField) (t pgType, wire bool) {
	switch oid {
	case pgtype.BoolOID:
		return pgType{filament.LogicalBool, decBool, textBool, parseBool}, true
	case pgtype.Int2OID:
		return pgType{filament.LogicalInt16, decInt2, textInt2, parseInt2}, true
	case pgtype.Int4OID:
		return pgType{filament.LogicalInt32, decInt4, textInt4, parseInt4}, true
	case pgtype.Int8OID:
		return pgType{filament.LogicalInt64, decInt8, textInt8, parseInt8}, true
	case pgtype.Float4OID:
		return pgType{filament.LogicalFloat32, decFloat4, textFloat4, parseFloat4}, true
	case pgtype.Float8OID:
		return pgType{filament.LogicalFloat64, decFloat8, textFloat8, parseFloat8}, true
	case pgtype.NumericOID:
		if f.Precision > 0 {
			return pgType{filament.LogicalDecimal, decDecimal(int32(f.Scale)), textNumeric, parseDecimal(int32(f.Precision), int32(f.Scale))}, true //nolint:gosec // <= 38
		}
		return pgType{filament.LogicalDecimal, decNumericText, textNumeric, decString}, true
	case pgtype.TextOID, pgtype.VarcharOID, pgtype.BPCharOID, pgtype.NameOID:
		return pgType{filament.LogicalString, decString, textRaw, decString}, true
	case pgtype.ByteaOID:
		return pgType{filament.LogicalBytes, decBytes, textBytea, parseBytea}, true
	case pgtype.UUIDOID:
		return pgType{filament.LogicalUUID, decUUID, textUUID, decString}, true
	case pgtype.DateOID:
		return pgType{filament.LogicalDate, decDate, textDate, parseDate}, true
	case pgtype.TimeOID:
		return pgType{filament.LogicalTime, decTime, textTime, parseTime}, true
	case pgtype.TimestampOID:
		return pgType{filament.LogicalTimestamp, decTimestamp, textTimestamp, parseTimestamp}, true
	case pgtype.TimestamptzOID:
		return pgType{filament.LogicalTimestampTZ, decTimestamp, textTimestamptz, parseTimestamp}, true
	case pgtype.JSONOID:
		return pgType{filament.LogicalJSON, decString, textRaw, decString}, true
	case pgtype.JSONBOID:
		return pgType{filament.LogicalJSON, decJSONB, textJSONB, decString}, true
	}
	t = pgType{filament.LogicalString, decString, textRaw, decString}
	if strings.HasSuffix(f.Native, "[]") {
		t.logical = filament.LogicalArray
	}
	return t, false
}

// numericPrecScale unpacks a numeric typmod. -1 (unbounded), a precision wider
// than decimal128, or a negative scale all read as unbounded: the column travels
// as text.
func numericPrecScale(typmod int32) (prec, scale int) {
	if typmod < 4 {
		return 0, 0
	}
	prec = int((typmod - 4) >> 16 & 0xffff)
	scale = int(int16((typmod - 4) & 0xffff)) //nolint:gosec // masked to 16 bits, sign-extended on purpose
	if prec <= 0 || prec > 38 || scale < 0 || scale > prec {
		return 0, 0
	}
	return prec, scale
}

// Postgres dates count from 2000-01-01, Arrow from 1970-01-01. The int32/int64
// extremes spell infinity in Postgres and pass through unshifted so a Postgres
// sink writes them back as infinity.
const (
	pgEpochDays   = 10957
	pgEpochMicros = pgEpochDays * batch.MicrosPerDay
)

func beInt16(b []byte) int16 { return int16(binary.BigEndian.Uint16(b)) } //nolint:gosec // wire value
func beInt32(b []byte) int32 { return int32(binary.BigEndian.Uint32(b)) } //nolint:gosec // wire value
func beInt64(b []byte) int64 { return int64(binary.BigEndian.Uint64(b)) } //nolint:gosec // wire value

func sized(src []byte, n int, what string) error {
	if len(src) != n {
		return fmt.Errorf("%s value: %d bytes", what, len(src))
	}
	return nil
}

// bool

func decBool(w filament.RowWriter, src []byte) error {
	if err := sized(src, 1, "bool"); err != nil {
		return err
	}
	w.Bool(src[0] != 0)
	return nil
}

func textBool(dst, src []byte) ([]byte, error) {
	if err := sized(src, 1, "bool"); err != nil {
		return nil, err
	}
	if src[0] != 0 {
		return append(dst, 't'), nil
	}
	return append(dst, 'f'), nil
}

func parseBool(w filament.RowWriter, text []byte) error {
	switch string(text) {
	case "t":
		w.Bool(true)
	case "f":
		w.Bool(false)
	default:
		return fmt.Errorf("invalid boolean %q", text)
	}
	return nil
}

// integers

func decInt2(w filament.RowWriter, src []byte) error {
	if err := sized(src, 2, "int2"); err != nil {
		return err
	}
	w.Int16(beInt16(src))
	return nil
}

func decInt4(w filament.RowWriter, src []byte) error {
	if err := sized(src, 4, "int4"); err != nil {
		return err
	}
	w.Int32(beInt32(src))
	return nil
}

func decInt8(w filament.RowWriter, src []byte) error {
	if err := sized(src, 8, "int8"); err != nil {
		return err
	}
	w.Int64(beInt64(src))
	return nil
}

func textInt2(dst, src []byte) ([]byte, error) {
	if err := sized(src, 2, "int2"); err != nil {
		return nil, err
	}
	return strconv.AppendInt(dst, int64(beInt16(src)), 10), nil
}

func textInt4(dst, src []byte) ([]byte, error) {
	if err := sized(src, 4, "int4"); err != nil {
		return nil, err
	}
	return strconv.AppendInt(dst, int64(beInt32(src)), 10), nil
}

func textInt8(dst, src []byte) ([]byte, error) {
	if err := sized(src, 8, "int8"); err != nil {
		return nil, err
	}
	return strconv.AppendInt(dst, beInt64(src), 10), nil
}

func parseInt2(w filament.RowWriter, text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 16)
	if err != nil {
		return err
	}
	w.Int16(int16(v))
	return nil
}

func parseInt4(w filament.RowWriter, text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 32)
	if err != nil {
		return err
	}
	w.Int32(int32(v))
	return nil
}

func parseInt8(w filament.RowWriter, text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}
	w.Int64(v)
	return nil
}

// floats

func decFloat4(w filament.RowWriter, src []byte) error {
	if err := sized(src, 4, "float4"); err != nil {
		return err
	}
	w.Float32(math.Float32frombits(binary.BigEndian.Uint32(src)))
	return nil
}

func decFloat8(w filament.RowWriter, src []byte) error {
	if err := sized(src, 8, "float8"); err != nil {
		return err
	}
	w.Float64(math.Float64frombits(binary.BigEndian.Uint64(src)))
	return nil
}

func textFloat4(dst, src []byte) ([]byte, error) {
	if err := sized(src, 4, "float4"); err != nil {
		return nil, err
	}
	return appendFloatText(dst, float64(math.Float32frombits(binary.BigEndian.Uint32(src))), 32), nil
}

func textFloat8(dst, src []byte) ([]byte, error) {
	if err := sized(src, 8, "float8"); err != nil {
		return nil, err
	}
	return appendFloatText(dst, math.Float64frombits(binary.BigEndian.Uint64(src)), 64), nil
}

// appendFloatText renders a float shortest-round-trip; the special values keep
// the spellings float8in accepts.
func appendFloatText(dst []byte, f float64, bits int) []byte {
	switch {
	case math.IsNaN(f):
		return append(dst, "NaN"...)
	case math.IsInf(f, 1):
		return append(dst, "Infinity"...)
	case math.IsInf(f, -1):
		return append(dst, "-Infinity"...)
	}
	return strconv.AppendFloat(dst, f, 'g', -1, bits)
}

func parseFloat4(w filament.RowWriter, text []byte) error {
	v, err := strconv.ParseFloat(string(text), 32)
	if err != nil {
		return err
	}
	w.Float32(float32(v))
	return nil
}

func parseFloat8(w filament.RowWriter, text []byte) error {
	v, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return err
	}
	w.Float64(v)
	return nil
}

// numeric: int16 ndigits, weight, sign, dscale, then ndigits base-10000 groups.
// Special sign values mark NaN/±Infinity.

const (
	numericNeg  = 0x4000
	numericNaN  = 0xC000
	numericPInf = 0xD000
	numericNInf = 0xF000
)

type numeric struct {
	ndigits, weight, dscale int
	sign                    uint16
	digits                  []byte
}

func parseNumeric(src []byte) (numeric, error) {
	if len(src) < 8 {
		return numeric{}, fmt.Errorf("numeric value: %d bytes", len(src))
	}
	n := numeric{
		ndigits: int(binary.BigEndian.Uint16(src[0:2])),
		weight:  int(beInt16(src[2:4])),
		sign:    binary.BigEndian.Uint16(src[4:6]),
		dscale:  int(binary.BigEndian.Uint16(src[6:8])),
		digits:  src[8:],
	}
	if len(n.digits) != 2*n.ndigits {
		return numeric{}, fmt.Errorf("numeric value: %d bytes for %d digits", len(src), n.ndigits)
	}
	switch n.sign {
	case 0, numericNeg, numericNaN, numericPInf, numericNInf:
		return n, nil
	}
	return numeric{}, fmt.Errorf("numeric sign %#x", n.sign)
}

func (n numeric) digit(i int) int64 {
	if i < 0 || i >= n.ndigits {
		return 0
	}
	return int64(binary.BigEndian.Uint16(n.digits[2*i:]))
}

func (n numeric) special() string {
	switch n.sign {
	case numericNaN:
		return "NaN"
	case numericPInf:
		return "Infinity"
	case numericNInf:
		return "-Infinity"
	}
	return ""
}

// decDecimal decodes a bounded numeric into decimal128 at the column's scale.
// The wire form is base-10000 groups with trailing zero groups stripped, so the
// value is digits × 10^(4·(weight−ndigits+1)); the scaled result is that times
// 10^scale. Every partial value stays below the final one, so 128-bit arithmetic
// never overflows for a column of at most 38 digits. NaN has no decimal form
// and lands as null; ±Infinity is not storable in a bounded column.
func decDecimal(scale int32) func(filament.RowWriter, []byte) error {
	return func(w filament.RowWriter, src []byte) error {
		n, err := parseNumeric(src)
		if err != nil {
			return err
		}
		if s := n.special(); s != "" {
			if n.sign == numericNaN {
				w.Null()
				return nil
			}
			return fmt.Errorf("numeric %s has no decimal form", s)
		}
		// exp is the power of ten the Horner accumulation over all groups is
		// short of (or, when negative, past) the target scale.
		exp := 4*(n.weight-n.ndigits+1) + int(scale)
		if exp < -3 || exp > 38 { // more fraction digits than the scale, or wider than 38 digits
			return fmt.Errorf("numeric does not fit decimal(%d)", scale)
		}
		groups := n.ndigits
		if exp < 0 {
			groups-- // the last group carries digits past the scale
		}
		var acc decimal128.Num
		tenK := decimal128.FromU64(10000)
		for i := range groups {
			acc = acc.Mul(tenK).Add(decimal128.FromU64(uint64(n.digit(i)))) //nolint:gosec // digit < 10000
		}
		if exp >= 0 {
			acc = acc.Mul(decimal128.GetScaleMultiplier(exp))
		} else {
			// exp is -1..-3: fold the last group's leading digits in and require
			// the rest to be zero, as they are for a value stored at this scale.
			last := decimal128.FromU64(uint64(n.digit(groups))) //nolint:gosec // digit < 10000
			q, r := last.Div(decimal128.GetScaleMultiplier(-exp))
			if r.Sign() != 0 {
				return fmt.Errorf("numeric has more than %d fraction digits", scale)
			}
			acc = acc.Mul(decimal128.GetScaleMultiplier(4 + exp)).Add(q)
		}
		if n.sign == numericNeg {
			acc = acc.Negate()
		}
		w.Decimal(acc)
		return nil
	}
}

// decNumericText renders an unbounded numeric as numeric_out text.
func decNumericText(w filament.RowWriter, src []byte) error {
	var buf [64]byte
	out, err := textNumeric(buf[:0], src)
	if err != nil {
		return err
	}
	w.StringBytes(out)
	return nil
}

// textNumeric renders the exact decimal text numeric_out produces (1.5000 stays
// 1.5000).
func textNumeric(dst, src []byte) ([]byte, error) {
	n, err := parseNumeric(src)
	if err != nil {
		return nil, err
	}
	if s := n.special(); s != "" {
		return append(dst, s...), nil
	}
	if n.sign == numericNeg {
		dst = append(dst, '-')
	}
	// Integer part: the first group unpadded, the rest zero-padded to 4.
	if n.weight < 0 {
		dst = append(dst, '0')
	} else {
		dst = strconv.AppendInt(dst, n.digit(0), 10)
		for i := 1; i <= n.weight; i++ {
			dst = batch.AppendPadded(dst, n.digit(i), 4)
		}
	}
	// Fraction: exactly dscale digits from the groups after the decimal point.
	if n.dscale > 0 {
		dst = append(dst, '.')
		start := len(dst)
		for i := n.weight + 1; len(dst)-start < n.dscale; i++ {
			dst = batch.AppendPadded(dst, n.digit(i), 4)
		}
		dst = dst[:start+n.dscale]
	}
	return dst, nil
}

// parseDecimal reads numeric text into decimal128; NaN lands as null, as it
// does on the binary path.
func parseDecimal(prec, scale int32) func(filament.RowWriter, []byte) error {
	return func(w filament.RowWriter, text []byte) error {
		if string(text) == "NaN" {
			w.Null()
			return nil
		}
		v, err := decimal128.FromString(string(text), prec, scale)
		if err != nil {
			return err
		}
		w.Decimal(v)
		return nil
	}
}

// text, json, jsonb

func decString(w filament.RowWriter, src []byte) error {
	w.StringBytes(src)
	return nil
}

func textRaw(dst, src []byte) ([]byte, error) { return append(dst, src...), nil }

var errJSONBVersion = errors.New("jsonb version byte")

func decJSONB(w filament.RowWriter, src []byte) error {
	if len(src) == 0 || src[0] != 1 {
		return errJSONBVersion
	}
	w.StringBytes(src[1:])
	return nil
}

func textJSONB(dst, src []byte) ([]byte, error) {
	if len(src) == 0 || src[0] != 1 {
		return nil, errJSONBVersion
	}
	return append(dst, src[1:]...), nil
}

// bytea

func decBytes(w filament.RowWriter, src []byte) error {
	w.Bytes(src)
	return nil
}

// textBytea renders bytea as \x-prefixed hex, matching bytea_output=hex.
func textBytea(dst, src []byte) ([]byte, error) {
	dst = append(dst, '\\', 'x')
	return hex.AppendEncode(dst, src), nil
}

func parseBytea(w filament.RowWriter, text []byte) error {
	if len(text) < 2 || text[0] != '\\' || text[1] != 'x' {
		return fmt.Errorf("bytea %q is not hex form", text)
	}
	buf := make([]byte, hex.DecodedLen(len(text)-2))
	n, err := hex.Decode(buf, text[2:])
	if err != nil {
		return err
	}
	w.Bytes(buf[:n])
	return nil
}

// uuid

func decUUID(w filament.RowWriter, src []byte) error {
	if err := sized(src, 16, "uuid"); err != nil {
		return err
	}
	var buf [36]byte
	w.StringBytes(appendUUID(buf[:0], src))
	return nil
}

func textUUID(dst, src []byte) ([]byte, error) {
	if err := sized(src, 16, "uuid"); err != nil {
		return nil, err
	}
	return appendUUID(dst, src), nil
}

func appendUUID(dst, src []byte) []byte {
	dst = hex.AppendEncode(dst, src[0:4])
	dst = append(dst, '-')
	dst = hex.AppendEncode(dst, src[4:6])
	dst = append(dst, '-')
	dst = hex.AppendEncode(dst, src[6:8])
	dst = append(dst, '-')
	dst = hex.AppendEncode(dst, src[8:10])
	dst = append(dst, '-')
	return hex.AppendEncode(dst, src[10:16])
}

// date

func decDate(w filament.RowWriter, src []byte) error {
	if err := sized(src, 4, "date"); err != nil {
		return err
	}
	v := beInt32(src)
	if v != math.MaxInt32 && v != math.MinInt32 {
		v += pgEpochDays
	}
	w.Date(v)
	return nil
}

func textDate(dst, src []byte) ([]byte, error) {
	if err := sized(src, 4, "date"); err != nil {
		return nil, err
	}
	v := beInt32(src)
	switch v {
	case math.MaxInt32:
		return append(dst, "infinity"...), nil
	case math.MinInt32:
		return append(dst, "-infinity"...), nil
	}
	return batch.AppendDate(dst, int64(v)+pgEpochDays), nil
}

func parseDate(w filament.RowWriter, text []byte) error {
	switch string(text) {
	case "infinity":
		w.Date(math.MaxInt32)
		return nil
	case "-infinity":
		w.Date(math.MinInt32)
		return nil
	}
	days, rest, err := readDate(era(text))
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("invalid date %q", text)
	}
	w.Date(int32(days)) //nolint:gosec // date range
	return nil
}

// time

func decTime(w filament.RowWriter, src []byte) error {
	if err := sized(src, 8, "time"); err != nil {
		return err
	}
	w.Time(beInt64(src))
	return nil
}

func textTime(dst, src []byte) ([]byte, error) {
	if err := sized(src, 8, "time"); err != nil {
		return nil, err
	}
	return batch.AppendTimeOfDay(dst, beInt64(src)), nil
}

func parseTime(w filament.RowWriter, text []byte) error {
	us, rest, err := batch.ReadTimeOfDay(text)
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("invalid time %q", text)
	}
	w.Time(us)
	return nil
}

// timestamp, timestamptz

func decTimestamp(w filament.RowWriter, src []byte) error {
	if err := sized(src, 8, "timestamp"); err != nil {
		return err
	}
	v := beInt64(src)
	if v != math.MaxInt64 && v != math.MinInt64 {
		v += pgEpochMicros
	}
	w.Timestamp(v)
	return nil
}

func textTimestamp(dst, src []byte) ([]byte, error) { return appendTimestampText(dst, src, false) }

// textTimestamptz renders at UTC (+00): a different spelling of the same
// instant than the session zone, which timestamptz_in resolves identically.
func textTimestamptz(dst, src []byte) ([]byte, error) { return appendTimestampText(dst, src, true) }

func appendTimestampText(dst, src []byte, utc bool) ([]byte, error) {
	if err := sized(src, 8, "timestamp"); err != nil {
		return nil, err
	}
	us := beInt64(src)
	switch us {
	case math.MaxInt64:
		return append(dst, "infinity"...), nil
	case math.MinInt64:
		return append(dst, "-infinity"...), nil
	}
	dst = batch.AppendTimestamp(dst, us+pgEpochMicros, ' ')
	if utc {
		dst = append(dst, "+00"...)
	}
	return dst, nil
}

// parseTimestamp reads "YYYY-MM-DD HH:MM:SS[.ffffff][+HH[:MM]][ BC]"; a zone
// offset is folded into UTC.
func parseTimestamp(w filament.RowWriter, text []byte) error {
	switch string(text) {
	case "infinity":
		w.Timestamp(math.MaxInt64)
		return nil
	case "-infinity":
		w.Timestamp(math.MinInt64)
		return nil
	}
	days, rest, err := readDate(era(text))
	if err != nil || len(rest) == 0 || rest[0] != ' ' {
		return fmt.Errorf("invalid timestamp %q", text)
	}
	tod, rest, err := batch.ReadTimeOfDay(rest[1:])
	if err != nil {
		return fmt.Errorf("invalid timestamp %q", text)
	}
	us := days*batch.MicrosPerDay + tod
	if len(rest) > 0 && (rest[0] == '+' || rest[0] == '-') {
		neg := rest[0] == '-'
		var off int64
		if off, rest, err = readOffset(rest[1:]); err != nil {
			return fmt.Errorf("invalid timestamp %q", text)
		}
		if neg {
			off = -off
		}
		us -= off
	}
	if len(rest) != 0 {
		return fmt.Errorf("invalid timestamp %q", text)
	}
	w.Timestamp(us)
	return nil
}

// era splits a trailing " BC" off a date or timestamp text.
func era(text []byte) (body []byte, bc bool) {
	if bytes.HasSuffix(text, []byte(" BC")) {
		return text[:len(text)-3], true
	}
	return text, false
}

// readDate reads "YYYY-MM-DD" and returns days since the Unix epoch and the
// unread tail. bc flips the year: it parsed as year y AD, and year y BC is 1-y.
func readDate(text []byte, bc bool) (int64, []byte, error) {
	days, rest, err := batch.ReadDate(text)
	if err != nil {
		return 0, nil, err
	}
	if bc {
		y, m, d := batch.DaysToDate(days)
		days = batch.DateToDays(1-y, m, d)
	}
	return days, rest, nil
}

// readOffset reads "HH[:MM[:SS]]" as microseconds.
func readOffset(text []byte) (int64, []byte, error) {
	h, rest, ok := batch.Digits(text, 2)
	if !ok {
		return 0, nil, batch.ErrBadDate
	}
	off := h * batch.MicrosPerHour
	if len(rest) > 0 && rest[0] == ':' {
		m, r, ok := batch.Digits(rest[1:], 2)
		if !ok {
			return 0, nil, batch.ErrBadDate
		}
		off += m * batch.MicrosPerMinute
		rest = r
		if len(rest) > 0 && rest[0] == ':' {
			s, r, ok := batch.Digits(rest[1:], 2)
			if !ok {
				return 0, nil, batch.ErrBadDate
			}
			off += s * batch.MicrosPerSecond
			rest = r
		}
	}
	return off, rest, nil
}
