package httpapi

// How a JSON value from an HTTP API lands in a typed row, per logical type.
// The manifest declares each field's type; the value arrives as its JSON text.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/http/internal/timestamps"
	"github.com/galaxy-io/filament/internal/arrowtext"
	"github.com/galaxy-io/filament/rowmodel"
)

// valueParser appends one field's raw JSON value into the writer.
type valueParser func(w arrowbatch.RowWriter, raw json.RawMessage) error

// typeFor picks a field's parser from its logical type.
func typeFor(f rowmodel.Field, rounding string) valueParser {
	switch f.Logical {
	case rowmodel.LogicalBool:
		return parseBool
	case rowmodel.LogicalInt16:
		return parseInt(16, func(w arrowbatch.RowWriter, v int64) { w.Int16(int16(v)) }) //nolint:gosec // parsed at 16 bits
	case rowmodel.LogicalInt32:
		return parseInt(32, func(w arrowbatch.RowWriter, v int64) { w.Int32(int32(v)) }) //nolint:gosec // parsed at 32 bits
	case rowmodel.LogicalInt64:
		return parseInt(64, func(w arrowbatch.RowWriter, v int64) { w.Int64(v) })
	case rowmodel.LogicalFloat32:
		return parseFloat(32, func(w arrowbatch.RowWriter, v float64) { w.Float32(float32(v)) })
	case rowmodel.LogicalFloat64:
		return parseFloat(64, func(w arrowbatch.RowWriter, v float64) { w.Float64(v) })
	case rowmodel.LogicalDecimal:
		if f.Precision > 0 {
			return parseFixedDecimal(f.Precision, f.Scale, rounding)
		}
		return parseNumberText
	case rowmodel.LogicalJSON:
		return parseJSON
	case rowmodel.LogicalDate:
		return parseDate
	case rowmodel.LogicalTime:
		return parseTime
	case rowmodel.LogicalTimestamp, rowmodel.LogicalTimestampTZ:
		return parseTimestamp
	default: // string, uuid, unknown
		return parseString
	}
}

func parseBool(w arrowbatch.RowWriter, raw json.RawMessage) error {
	switch string(raw) {
	case "true", `"true"`:
		w.Bool(true)
	case "false", `"false"`:
		w.Bool(false)
	default:
		return fmt.Errorf("%s is not a boolean", raw)
	}
	return nil
}

// unquoted returns a JSON string's contents, or the raw text of any other value.
func unquoted(raw json.RawMessage) (string, error) {
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return "", err
		}
		return s, nil
	}
	return string(raw), nil
}

func parseInt(bits int, set func(arrowbatch.RowWriter, int64)) valueParser {
	return func(w arrowbatch.RowWriter, raw json.RawMessage) error {
		s, err := unquoted(raw)
		if err != nil {
			return err
		}
		v, err := strconv.ParseInt(s, 10, bits)
		if err != nil {
			return err
		}
		set(w, v)
		return nil
	}
}

func parseFloat(bits int, set func(arrowbatch.RowWriter, float64)) valueParser {
	return func(w arrowbatch.RowWriter, raw json.RawMessage) error {
		s, err := unquoted(raw)
		if err != nil {
			return err
		}
		v, err := strconv.ParseFloat(s, bits)
		if err != nil {
			return err
		}
		set(w, v)
		return nil
	}
}

// maxExpandedDecimalDigits bounds the plain digit expansion of an
// exponent-form decimal. A decimal with more digits than this is not a
// real API value, and expanding it would allocate an unbounded string:
// 1e100000000 would expand to about 100,000,001 characters.
const maxExpandedDecimalDigits = 10000

// parseNumberText keeps an unbounded decimal as its digits. A JSON number in
// exponent form (1e5) is spelled out exactly, since the sinks' numeric parsers
// take plain digits. The expansion works on the digits directly, so no
// significant digit is lost however long the value is; a binary float
// round-trip would discard digits beyond its mantissa.
func parseNumberText(w arrowbatch.RowWriter, raw json.RawMessage) error {
	s, err := unquoted(raw)
	if err != nil {
		return err
	}
	if strings.ContainsAny(s, "eE") {
		expanded, err := expandDecimal(s)
		if err != nil {
			return err
		}
		s = expanded
	}
	w.String(s)
	return nil
}

// expandDecimal spells an exponent-form decimal out as plain digits,
// preserving every significand digit exactly. The exponent only moves the
// decimal point; no binary float is involved. A value whose expansion
// would exceed maxExpandedDecimalDigits is an error, so a huge exponent
// cannot allocate an unbounded string. The bound is measured on the
// mantissa-adjusted expansion length, so a value that fits it is
// accepted even when its exponent alone is large.
func expandDecimal(s string) (string, error) {
	orig := s
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	e := strings.IndexAny(s, "eE")
	if e < 0 {
		return "", fmt.Errorf("%q is not a number", orig)
	}
	intPart, fracPart, ok := splitMantissa(s[:e])
	if !ok {
		return "", fmt.Errorf("%q is not a number", orig)
	}
	// Validate the exponent before the zero shortcut below: "0e" and
	// "0efoo" are not numbers, even though the mantissa is zero. The
	// exponent is parsed exactly up to len(fracPart) plus the bound,
	// which is enough to decide the expansion size; a larger magnitude
	// saturates, and the bound check below rejects it.
	exp, err := parseExponent(s[e+1:], len(fracPart)+maxExpandedDecimalDigits+1)
	if err != nil {
		return "", fmt.Errorf("%q is not a number", orig)
	}
	digits := strings.TrimLeft(intPart+fracPart, "0")
	if digits == "" {
		if neg {
			return "-0", nil
		}
		return "0", nil
	}
	// The value is digits * 10^(exp - len(fracPart)): the exponent moves
	// the decimal point by shift places to the right.
	shift := exp - len(fracPart)
	expandedLen := len(digits)
	switch {
	case shift >= 0:
		expandedLen += shift
	case len(digits) <= -shift:
		// "0." plus the leading zeros: the digit count cancels against
		// the leading-zero count, so the length is 2-shift.
		expandedLen = 2 - shift
	default:
		expandedLen++ // the decimal point
	}
	if neg {
		expandedLen++
	}
	if expandedLen > maxExpandedDecimalDigits {
		return "", fmt.Errorf("%q expands to more than %d digits", orig, maxExpandedDecimalDigits)
	}
	var b strings.Builder
	b.Grow(expandedLen)
	if neg {
		b.WriteByte('-')
	}
	switch {
	case shift >= 0:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", shift))
	case len(digits) > -shift:
		b.WriteString(digits[:len(digits)+shift])
		b.WriteByte('.')
		b.WriteString(digits[len(digits)+shift:])
	default:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -shift-len(digits)))
		b.WriteString(digits)
	}
	out := b.String()
	// Drop trailing zeros after the decimal point: 1.50e0 spells as 1.5.
	if strings.IndexByte(out, '.') >= 0 {
		out = strings.TrimRight(out, "0")
		out = strings.TrimSuffix(out, ".")
	}
	return out, nil
}

// splitMantissa splits a decimal mantissa into its integer and fraction
// digit strings. Either side may be empty (".5", "1."), but at least one
// digit must be present and every character must be a digit.
func splitMantissa(s string) (intPart, fracPart string, ok bool) {
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return "", "", false
	}
	intPart = parts[0]
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	for _, p := range parts {
		for i := 0; i < len(p); i++ {
			if p[i] < '0' || p[i] > '9' {
				return "", "", false
			}
		}
	}
	if intPart == "" && fracPart == "" {
		return "", "", false
	}
	return intPart, fracPart, true
}

// parseExponent reads the exponent of an exponent-form decimal. It
// validates the syntax, an optional sign followed by digits, and parses
// the magnitude exactly up to limit. A larger magnitude saturates at
// limit+1, which is enough for the expansion bound check to reject the
// value, while a zero mantissa still returns zero for any exponent.
func parseExponent(s string, limit int) (int, error) {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	} else if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("empty exponent")
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("bad exponent")
		}
		if n <= limit {
			n = n*10 + int(s[i]-'0')
			if n > limit {
				n = limit + 1
			}
		}
	}
	if neg {
		return -n, nil
	}
	return n, nil
}

func parseString(w arrowbatch.RowWriter, raw json.RawMessage) error {
	s, err := unquoted(raw)
	if err != nil {
		return err
	}
	w.String(s)
	return nil
}

// parseJSON keeps the value as JSON text (a JSON string stays quoted).
func parseJSON(w arrowbatch.RowWriter, raw json.RawMessage) error {
	w.StringBytes(bytes.TrimSpace(raw))
	return nil
}

func parseDate(w arrowbatch.RowWriter, raw json.RawMessage) error {
	s, err := unquoted(raw)
	if err != nil {
		return err
	}
	days, rest, err := arrowtext.ReadDate([]byte(s))
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("%q is not a date", s)
	}
	w.Date(int32(days)) //nolint:gosec // date range
	return nil
}

func parseTime(w arrowbatch.RowWriter, raw json.RawMessage) error {
	s, err := unquoted(raw)
	if err != nil {
		return err
	}
	us, rest, err := arrowtext.ReadTimeOfDay([]byte(s))
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("%q is not a time", s)
	}
	w.Time(us)
	return nil
}

// parseTimestamp reads an ISO 8601 / RFC 3339 instant (or a Unix epoch number,
// seconds or milliseconds, integer or fractional) as microseconds since the
// epoch, UTC.
func parseTimestamp(w arrowbatch.RowWriter, raw json.RawMessage) error {
	if len(raw) > 0 && raw[0] != '"' {
		f, err := strconv.ParseFloat(string(raw), 64)
		if err != nil {
			return fmt.Errorf("%s is not a timestamp", raw)
		}
		if f > 1e12 { // milliseconds
			f *= 1000
		} else {
			f *= 1_000_000
		}
		w.Timestamp(int64(math.Round(f)))
		return nil
	}
	s, err := unquoted(raw)
	if err != nil {
		return err
	}
	t, err := timestamps.Parse(s)
	if err != nil {
		return err
	}
	w.Timestamp(t.UnixMicro())
	return nil
}

// parseFixedDecimal keeps strict conversion by default and rounds only when declared.
func parseFixedDecimal(precision, scale int, rounding string) valueParser {
	return func(w arrowbatch.RowWriter, raw json.RawMessage) error {
		value, err := unquoted(raw)
		if err != nil {
			return err
		}
		invalid := fmt.Errorf("decimal exceeds declared precision or scale")
		if len(value) > 128 || !json.Valid([]byte(value)) {
			return invalid
		}
		if i := strings.IndexAny(value, "eE"); i >= 0 {
			exponent, err := strconv.Atoi(value[i+1:])
			if err != nil || exponent < -128 || exponent > 128 {
				return invalid
			}
		}
		number, ok := new(big.Rat).SetString(value)
		if !ok {
			return invalid
		}
		factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
		number.Mul(number, new(big.Rat).SetInt(factor))
		if !number.IsInt() && rounding != "half_even" {
			return invalid
		}
		unscaled := new(big.Int)
		if number.IsInt() {
			unscaled.Set(number.Num())
		} else {
			remainder := new(big.Int)
			unscaled.QuoRem(number.Num(), number.Denom(), remainder)
			comparison := new(big.Int).Mul(new(big.Int).Abs(remainder), big.NewInt(2)).Cmp(number.Denom())
			if comparison > 0 || comparison == 0 && new(big.Int).Abs(unscaled).Bit(0) == 1 {
				unscaled.Add(unscaled, big.NewInt(int64(number.Sign())))
			}
		}
		if len(new(big.Int).Abs(unscaled).String()) > precision {
			return invalid
		}
		w.Decimal(decimal128.FromBigInt(unscaled))
		return nil
	}
}
