package mysql

// Every value rendering the sink knows, in one place: the destination column
// type for each logical type, and the LOAD DATA text form of each Arrow storage
// type (tab-separated fields, backslash escapes, \N for null).

import (
	"math"
	"strconv"
	"strings"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"

	"github.com/galaxy-io/filament/internal/arrowtext"
	"github.com/galaxy-io/filament/rowmodel"
)

// engine is the source engine whose native type spellings this sink reuses.
const engine = "mysql"

// columnType picks a destination column type: the source's own spelling when it
// came from MySQL, else a portable mapping from the logical type.
func columnType(f rowmodel.Field, sameEngine bool) string {
	if sameEngine && f.Native != "" {
		return f.Native
	}
	switch f.Logical {
	case rowmodel.LogicalBool:
		return "tinyint(1)"
	case rowmodel.LogicalInt16:
		return "smallint"
	case rowmodel.LogicalInt32:
		return "int"
	case rowmodel.LogicalInt64:
		return "bigint"
	case rowmodel.LogicalFloat32:
		return "float"
	case rowmodel.LogicalFloat64:
		return "double"
	case rowmodel.LogicalDecimal:
		if f.Precision > 0 {
			return "decimal(" + strconv.Itoa(f.Precision) + "," + strconv.Itoa(f.Scale) + ")"
		}
		return "decimal(65,30)"
	case rowmodel.LogicalBytes:
		return "longblob"
	case rowmodel.LogicalDate:
		return "date"
	case rowmodel.LogicalTime:
		return "time(6)"
	case rowmodel.LogicalTimestamp, rowmodel.LogicalTimestampTZ:
		// timestamp's range stops at 2038; datetime holds any instant (UTC).
		return "datetime(6)"
	case rowmodel.LogicalJSON:
		return "json"
	case rowmodel.LogicalUUID:
		return "char(36)"
	default: // string, array (native literal text), unknown
		return "longtext"
	}
}

// MySQL's calendar runs from 0001-01-01 to 9999-12-31; a value outside it
// (BC, or a Postgres infinity sentinel) lands as null.
var (
	minDays = arrowtext.DateToDays(1, 1, 1)
	maxDays = arrowtext.DateToDays(9999, 12, 31)
)

// valueFn appends one value's LOAD DATA text.
type valueFn func(dst []byte, col arrow.Array, i int) []byte

// textFor picks the LOAD DATA renderer for an Arrow field.
func textFor(f arrow.Field) valueFn {
	switch f.Type.ID() {
	case arrow.BOOL:
		return func(dst []byte, col arrow.Array, i int) []byte {
			if col.(*array.Boolean).Value(i) {
				return append(dst, '1')
			}
			return append(dst, '0')
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
			return appendEscaped(dst, col.(*array.Binary).Value(i))
		}
	case arrow.DATE32:
		return func(dst []byte, col arrow.Array, i int) []byte {
			v := int64(col.(*array.Date32).Value(i))
			if v < minDays || v > maxDays { // BC, past 9999, or Postgres infinity: no MySQL form
				return append(dst, '\\', 'N')
			}
			return arrowtext.AppendDate32(dst, int32(v)) //nolint:gosec // checked against MySQL's Date32 range
		}
	case arrow.TIME64:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return arrowtext.AppendTimeOfDay(dst, int64(col.(*array.Time64).Value(i)))
		}
	case arrow.TIMESTAMP:
		return func(dst []byte, col arrow.Array, i int) []byte {
			v := int64(col.(*array.Timestamp).Value(i))
			if v < minDays*arrowtext.MicrosPerDay || v >= (maxDays+1)*arrowtext.MicrosPerDay { // BC, past 9999, or Postgres infinity: no MySQL form
				return append(dst, '\\', 'N')
			}
			return arrowtext.AppendTimestamp(dst, v, ' ')
		}
	default:
		return func(dst []byte, col arrow.Array, i int) []byte {
			return appendEscaped(dst, col.ValueStr(i))
		}
	}
}

// appendFloat appends v; NaN and the infinities have no MySQL form and land as
// null (the server would otherwise coerce them to 0).
func appendFloat(dst []byte, v float64, bits int) []byte {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return append(dst, '\\', 'N')
	}
	return strconv.AppendFloat(dst, v, 'g', -1, bits)
}

// appendEscaped appends s for a LOAD DATA field: backslash, tab, newline,
// carriage return and NUL escaped.
func appendEscaped[T string | []byte](dst []byte, s T) []byte {
	start := 0
	for i := range len(s) {
		esc, ok := escapeOf(s[i])
		if !ok {
			continue
		}
		dst = append(dst, s[start:i]...)
		dst = append(dst, '\\', esc)
		start = i + 1
	}
	return append(dst, s[start:]...)
}

func escapeOf(c byte) (byte, bool) {
	switch c {
	case '\\':
		return '\\', true
	case '\t':
		return 't', true
	case '\n':
		return 'n', true
	case '\r':
		return 'r', true
	case 0:
		return '0', true
	}
	return 0, false
}

// quoteIdent renders s as a backtick-quoted MySQL identifier.
func quoteIdent(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}
