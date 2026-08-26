package httpapi

// How a JSON value from an HTTP API lands in a typed row, per logical type.
// The manifest declares each field's type; the value arrives as its JSON text.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/arrowtext"
	"github.com/galaxy-io/filament/rowmodel"
)

// valueParser appends one field's raw JSON value into the writer.
type valueParser func(w arrowbatch.RowWriter, raw json.RawMessage) error

// typeFor picks a field's parser from its logical type.
func typeFor(f rowmodel.Field) valueParser {
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
	case rowmodel.LogicalDecimal: // a manifest declares no precision: the digits travel as text
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
			// A JSON number may carry a fraction of zeros (1.0) or an exponent.
			f, ferr := strconv.ParseFloat(s, 64)
			if ferr != nil || f != float64(int64(f)) {
				return err
			}
			v = int64(f)
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

// parseNumberText keeps an unbounded decimal as its digits. A JSON number in
// exponent form (1e5) is spelled out, since the sinks' numeric parsers take
// plain digits; at that point it is a float anyway.
func parseNumberText(w arrowbatch.RowWriter, raw json.RawMessage) error {
	s, err := unquoted(raw)
	if err != nil {
		return err
	}
	if strings.ContainsAny(s, "eE") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("%q is not a number", s)
		}
		s = strconv.FormatFloat(f, 'f', -1, 64)
	}
	w.String(s)
	return nil
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

// timestampLayouts are the spellings APIs commonly use, tried in order.
var timestampLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05.999999999Z07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02",
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
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, strings.TrimSpace(s)); err == nil {
			w.Timestamp(t.UnixMicro())
			return nil
		}
	}
	return fmt.Errorf("%q is not a timestamp", s)
}
