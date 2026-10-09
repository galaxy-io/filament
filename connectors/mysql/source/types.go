package mysql

// Every MySQL type this source understands, in one place: how a column maps to
// a LogicalType (with a decimal's precision and scale) and how its text form —
// the driver's text protocol for query reads, the binlog value's text for CDC —
// appends into a row. A type absent from the table travels as utf8.

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/arrowtext"
	"github.com/galaxy-io/filament/rowmodel"
)

// mysqlType is one column type's behaviour in the source.
type mysqlType struct {
	logical          rowmodel.LogicalType
	precision, scale int
	parse            func(w arrowbatch.RowWriter, text []byte) error
	// binlog renders a binlog row value as the text parse reads, for the types
	// the binlog carries as integers (enum, set, bit); nil uses valueBytes.
	binlog func(v any) ([]byte, error)
}

// typeFor classifies a column by its information_schema DATA_TYPE (the bare
// keyword) and COLUMN_TYPE (the full declaration, e.g. "bigint unsigned",
// "decimal(10,2)").
func typeFor(dataType, fullType string) mysqlType {
	t := strings.ToLower(dataType)
	full := strings.ToLower(fullType)
	unsigned := strings.Contains(full, "unsigned")
	switch t {
	case "tinyint":
		if strings.HasPrefix(full, "tinyint(1)") && !unsigned { // MySQL's boolean idiom
			return mysqlType{logical: rowmodel.LogicalBool, parse: parseBool}
		}
		return mysqlType{logical: rowmodel.LogicalInt16, parse: parseInt16}
	case "smallint":
		if unsigned {
			return mysqlType{logical: rowmodel.LogicalInt32, parse: parseInt32}
		}
		return mysqlType{logical: rowmodel.LogicalInt16, parse: parseInt16}
	case "mediumint", "int", "integer":
		if unsigned {
			return mysqlType{logical: rowmodel.LogicalInt64, parse: parseInt64}
		}
		return mysqlType{logical: rowmodel.LogicalInt32, parse: parseInt32}
	case "bigint":
		if unsigned { // may exceed int64
			return mysqlType{logical: rowmodel.LogicalDecimal, precision: 20, parse: parseDecimal(20, 0)}
		}
		return mysqlType{logical: rowmodel.LogicalInt64, parse: parseInt64}
	case "float":
		return mysqlType{logical: rowmodel.LogicalFloat32, parse: parseFloat32}
	case "double", "real":
		return mysqlType{logical: rowmodel.LogicalFloat64, parse: parseFloat64}
	case "decimal", "numeric":
		p, s := decimalPrecScale(full)
		if p > 0 {
			return mysqlType{logical: rowmodel.LogicalDecimal, precision: p, scale: s, parse: parseDecimal(int32(p), int32(s))} //nolint:gosec // <= 38
		}
		return mysqlType{logical: rowmodel.LogicalDecimal, parse: parseText}
	case "json":
		return mysqlType{logical: rowmodel.LogicalJSON, parse: parseText}
	case "binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob":
		return mysqlType{logical: rowmodel.LogicalBytes, parse: parseBytes}
	case "bit":
		return mysqlType{logical: rowmodel.LogicalBytes, parse: parseBytes, binlog: binlogBit(bitWidth(full))}
	case "enum":
		return mysqlType{logical: rowmodel.LogicalString, parse: parseText, binlog: binlogEnum(members(fullType))}
	case "set":
		return mysqlType{logical: rowmodel.LogicalString, parse: parseText, binlog: binlogSet(members(fullType))}
	case "date":
		return mysqlType{logical: rowmodel.LogicalDate, parse: parseDate}
	case "datetime":
		return mysqlType{logical: rowmodel.LogicalTimestamp, parse: parseDatetime}
	case "timestamp":
		return mysqlType{logical: rowmodel.LogicalTimestampTZ, parse: parseDatetime}
	case "time":
		return mysqlType{logical: rowmodel.LogicalTime, parse: parseTime}
	default: // char, varchar, text, year, and anything else
		return mysqlType{logical: rowmodel.LogicalString, parse: parseText}
	}
}

// members reads the quoted member list of an enum(...) or set(...) declaration.
// MySQL doubles a quote inside a member and backslash-escapes a backslash, NUL,
// newline and carriage return.
func members(fullType string) []string {
	open := strings.IndexByte(fullType, '(')
	if open < 0 {
		return nil
	}
	var out []string
	var cur []byte
	quoted := false
	for i := open + 1; i < len(fullType); i++ {
		c := fullType[i]
		switch {
		case !quoted && c == '\'':
			quoted, cur = true, cur[:0]
		case !quoted:
			// separators and the closing parenthesis
		case c == '\'' && i+1 < len(fullType) && fullType[i+1] == '\'':
			cur = append(cur, '\'')
			i++
		case c == '\'':
			out = append(out, string(cur))
			quoted = false
		case c == '\\' && i+1 < len(fullType):
			i++
			switch fullType[i] {
			case '0':
				cur = append(cur, 0)
			case 'n':
				cur = append(cur, '\n')
			case 'r':
				cur = append(cur, '\r')
			default:
				cur = append(cur, fullType[i])
			}
		default:
			cur = append(cur, c)
		}
	}
	return out
}

// bitWidth reads M out of a bit(M) declaration; MySQL's default is 1.
func bitWidth(full string) int {
	open := strings.IndexByte(full, '(')
	end := strings.IndexByte(full, ')')
	if open < 0 || end < open {
		return 1
	}
	m, err := strconv.Atoi(strings.TrimSpace(full[open+1 : end]))
	if err != nil || m < 1 || m > 64 {
		return 1
	}
	return m
}

// binlogEnum maps the binlog's 1-based member index to the member; 0 is the
// empty string MySQL stores for an invalid value.
func binlogEnum(members []string) func(any) ([]byte, error) {
	return func(v any) ([]byte, error) {
		i, ok := v.(int64)
		if !ok {
			return valueBytes(v), nil
		}
		if i == 0 {
			return []byte{}, nil
		}
		if i < 0 || i > int64(len(members)) {
			return nil, fmt.Errorf("enum index %d out of range for %d members", i, len(members))
		}
		return []byte(members[i-1]), nil
	}
}

// binlogSet maps the binlog's member bitmap to the comma-separated members, in
// declaration order as the query path reads them.
func binlogSet(members []string) func(any) ([]byte, error) {
	return func(v any) ([]byte, error) {
		i, ok := v.(int64)
		if !ok {
			return valueBytes(v), nil
		}
		bits := uint64(i) //nolint:gosec // a 64-member set uses the sign bit
		if len(members) < 64 && bits>>len(members) != 0 {
			return nil, fmt.Errorf("set bitmap %#x has bits past %d members", bits, len(members))
		}
		out := []byte{}
		for m, member := range members {
			if bits&(1<<m) == 0 {
				continue
			}
			// MySQL adds the comma only after nonempty text, so an empty
			// first member leaves no leading comma.
			if len(out) > 0 {
				out = append(out, ',')
			}
			out = append(out, member...)
		}
		return out, nil
	}
}

// binlogBit renders the binlog's bit(M) integer as the ceil(M/8) big-endian
// bytes the query path reads.
func binlogBit(width int) func(any) ([]byte, error) {
	n := (width + 7) / 8
	return func(v any) ([]byte, error) {
		i, ok := v.(int64)
		if !ok {
			return valueBytes(v), nil
		}
		var buf [8]byte
		binary.BigEndian.PutUint64(buf[:], uint64(i)) //nolint:gosec // bit pattern
		return buf[8-n:], nil
	}
}

// decimalPrecScale reads "(p,s)" out of a decimal declaration; 0 precision when
// absent or wider than decimal128 (the column then travels as text).
func decimalPrecScale(full string) (prec, scale int) {
	open := strings.IndexByte(full, '(')
	end := strings.IndexByte(full, ')')
	if open < 0 || end < open {
		return 10, 0 // MySQL's default decimal
	}
	parts := strings.SplitN(full[open+1:end], ",", 2)
	p, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || p <= 0 || p > 38 {
		return 0, 0
	}
	if len(parts) == 2 {
		if s, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && s >= 0 && s <= p {
			return p, s
		}
		return 0, 0
	}
	return p, 0
}

func parseBool(w arrowbatch.RowWriter, text []byte) error {
	w.Bool(len(text) != 1 || text[0] != '0')
	return nil
}

func parseInt16(w arrowbatch.RowWriter, text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 16)
	if err != nil {
		return err
	}
	w.Int16(int16(v))
	return nil
}

func parseInt32(w arrowbatch.RowWriter, text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 32)
	if err != nil {
		return err
	}
	w.Int32(int32(v))
	return nil
}

func parseInt64(w arrowbatch.RowWriter, text []byte) error {
	v, err := strconv.ParseInt(string(text), 10, 64)
	if err != nil {
		return err
	}
	w.Int64(v)
	return nil
}

func parseFloat32(w arrowbatch.RowWriter, text []byte) error {
	v, err := strconv.ParseFloat(string(text), 32)
	if err != nil {
		return err
	}
	w.Float32(float32(v))
	return nil
}

func parseFloat64(w arrowbatch.RowWriter, text []byte) error {
	v, err := strconv.ParseFloat(string(text), 64)
	if err != nil {
		return err
	}
	w.Float64(v)
	return nil
}

func parseDecimal(prec, scale int32) func(arrowbatch.RowWriter, []byte) error {
	return func(w arrowbatch.RowWriter, text []byte) error {
		v, err := decimal128.FromString(string(text), prec, scale)
		if err != nil {
			return err
		}
		w.Decimal(v)
		return nil
	}
}

func parseText(w arrowbatch.RowWriter, text []byte) error {
	w.StringBytes(text)
	return nil
}

func parseBytes(w arrowbatch.RowWriter, text []byte) error {
	w.Bytes(text)
	return nil
}

// parseDate reads "YYYY-MM-DD"; MySQL's zero date, which has no calendar
// meaning, lands as null.
func parseDate(w arrowbatch.RowWriter, text []byte) error {
	if string(text) == "0000-00-00" || string(text) == "0000-00-00T00:00:00Z" {
		w.Null()
		return nil
	}
	// Prepared statements use MySQL's binary protocol. go-sql-driver/mysql
	// renders its typed date value as RFC3339 when it is scanned into RawBytes,
	// even though text-protocol reads use YYYY-MM-DD.
	if len(text) > len("2006-01-02") && text[len("2006-01-02")] == 'T' {
		if _, err := time.Parse(time.RFC3339Nano, string(text)); err != nil {
			return fmt.Errorf("invalid date %q", text)
		}
		text = text[:len("2006-01-02")]
	}
	days, rest, err := arrowtext.ReadDate(text)
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("invalid date %q", text)
	}
	w.Date(int32(days)) //nolint:gosec // date range
	return nil
}

// parseDatetime reads "YYYY-MM-DD HH:MM:SS[.ffffff]" as microseconds since the
// Unix epoch; a timestamp column arrives in UTC (the session and the binlog
// syncer are both pinned to it). The zero datetime lands as null.
func parseDatetime(w arrowbatch.RowWriter, text []byte) error {
	if strings.HasPrefix(string(text), "0000-00-00") {
		w.Null()
		return nil
	}
	// Prepared-statement rows are typed by the driver and render as RFC3339 on
	// their way into RawBytes. Text-protocol and binlog rows retain MySQL's
	// space-separated form, so support both without changing cursor semantics.
	if len(text) > len("2006-01-02") && text[len("2006-01-02")] == 'T' {
		instant, err := time.Parse(time.RFC3339Nano, string(text))
		if err != nil {
			return fmt.Errorf("invalid datetime %q", text)
		}
		w.Timestamp(instant.UnixMicro())
		return nil
	}
	days, rest, err := arrowtext.ReadDate(text)
	if err != nil || len(rest) == 0 || rest[0] != ' ' {
		return fmt.Errorf("invalid datetime %q", text)
	}
	tod, rest, err := arrowtext.ReadTimeOfDay(rest[1:])
	if err != nil || len(rest) != 0 {
		return fmt.Errorf("invalid datetime %q", text)
	}
	w.Timestamp(days*arrowtext.MicrosPerDay + tod)
	return nil
}

// parseTime reads "[-]HH:MM:SS[.ffffff]" (hours may exceed 23) as microseconds;
// a negative or over-a-day value has no time-of-day form and is rejected.
func parseTime(w arrowbatch.RowWriter, text []byte) error {
	us, rest, err := arrowtext.ReadTimeOfDay(text)
	if err != nil || len(rest) != 0 || us >= arrowtext.MicrosPerDay {
		return fmt.Errorf("time %q is not a time of day", text)
	}
	w.Time(us)
	return nil
}
