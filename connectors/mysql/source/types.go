package mysql

// Every MySQL type this source understands, in one place: how a column maps to
// a LogicalType (with a decimal's precision and scale) and how its text form —
// the driver's text protocol for query reads, the binlog value's text for CDC —
// appends into a row. A type absent from the table travels as utf8.

import (
	"fmt"
	"strconv"
	"strings"

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
	case "binary", "varbinary", "blob", "tinyblob", "mediumblob", "longblob", "bit":
		return mysqlType{logical: rowmodel.LogicalBytes, parse: parseBytes}
	case "date":
		return mysqlType{logical: rowmodel.LogicalDate, parse: parseDate}
	case "datetime":
		return mysqlType{logical: rowmodel.LogicalTimestamp, parse: parseDatetime}
	case "timestamp":
		return mysqlType{logical: rowmodel.LogicalTimestampTZ, parse: parseDatetime}
	case "time":
		return mysqlType{logical: rowmodel.LogicalTime, parse: parseTime}
	default: // char, varchar, text, enum, set, year, and anything else
		return mysqlType{logical: rowmodel.LogicalString, parse: parseText}
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
	if string(text) == "0000-00-00" {
		w.Null()
		return nil
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
