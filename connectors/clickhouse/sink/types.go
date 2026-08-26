package clickhouse

// Every per-type behaviour of the sink, in one place: the ClickHouse column
// type a schema field lands as, and for each Arrow storage type the Go value
// clickhouse-go appends for it.

import (
	"fmt"
	"math"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/shopspring/decimal"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/arrowtext"
	"github.com/galaxy-io/filament/rowmodel"
)

// columnType maps a schema field onto a conservative ClickHouse type. JSON,
// arrays, and unknown source-native types land as their text in String until
// the portable schema carries enough nested type information to create a
// lossless ClickHouse JSON/Array declaration.
func columnType(f rowmodel.Field) string {
	var typ string
	switch f.Logical {
	case rowmodel.LogicalBool:
		typ = "Bool"
	case rowmodel.LogicalInt16:
		typ = "Int16"
	case rowmodel.LogicalInt32:
		typ = "Int32"
	case rowmodel.LogicalInt64:
		typ = "Int64"
	case rowmodel.LogicalFloat32:
		typ = "Float32"
	case rowmodel.LogicalFloat64:
		typ = "Float64"
	case rowmodel.LogicalDecimal:
		typ = decimalType(f)
	case rowmodel.LogicalString, rowmodel.LogicalBytes, rowmodel.LogicalTime,
		rowmodel.LogicalJSON, rowmodel.LogicalArray, rowmodel.LogicalUnknown:
		typ = "String"
	case rowmodel.LogicalDate:
		typ = "Date32"
	case rowmodel.LogicalTimestamp:
		typ = "DateTime64(6)"
	case rowmodel.LogicalTimestampTZ:
		typ = "DateTime64(6, 'UTC')"
	case rowmodel.LogicalUUID:
		typ = "UUID"
	default:
		typ = "String"
	}
	if f.Nullable {
		return "Nullable(" + typ + ")"
	}
	return typ
}

// decimalType sizes a Decimal from the field's precision and scale; an
// unbounded decimal (no precision) lands as Decimal(38, 9).
func decimalType(f rowmodel.Field) string {
	// ClickHouse Decimal supports precision 1..76 and scale <= precision.
	if f.Precision < 1 || f.Precision > 76 || f.Scale < 0 || f.Scale > f.Precision {
		return "Decimal(38, 9)"
	}
	return fmt.Sprintf("Decimal(%d, %d)", f.Precision, f.Scale)
}

// valueFn returns row i of a column as the value clickhouse-go appends.
type valueFn func(col arrow.Array, i int) any

// valueFor picks a field's renderer from its Arrow storage type. Naive
// timestamps stay text so the server reads the wall clock in the column's zone;
// instants (timestamptz) go as time.Time; times, bytes, json, uuid and arrays
// are strings.
func valueFor(f arrow.Field) valueFn {
	switch f.Type.ID() {
	case arrow.BOOL:
		return func(col arrow.Array, i int) any { return col.(*array.Boolean).Value(i) }
	case arrow.INT16:
		return func(col arrow.Array, i int) any { return col.(*array.Int16).Value(i) }
	case arrow.INT32:
		return func(col arrow.Array, i int) any { return col.(*array.Int32).Value(i) }
	case arrow.INT64:
		return func(col arrow.Array, i int) any { return col.(*array.Int64).Value(i) }
	case arrow.FLOAT32:
		return func(col arrow.Array, i int) any { return col.(*array.Float32).Value(i) }
	case arrow.FLOAT64:
		return func(col arrow.Array, i int) any { return col.(*array.Float64).Value(i) }
	case arrow.DECIMAL128:
		scale := f.Type.(*arrow.Decimal128Type).Scale
		return func(col arrow.Array, i int) any {
			return decimal.NewFromBigInt(col.(*array.Decimal128).Value(i).BigInt(), -scale)
		}
	case arrow.STRING:
		if arrowbatch.LogicalOf(f) == rowmodel.LogicalDecimal { // unbounded numeric text
			return func(col arrow.Array, i int) any {
				d, err := decimal.NewFromString(col.(*array.String).Value(i))
				if err != nil {
					return col.(*array.String).Value(i) // the driver reports the malformed value
				}
				return d
			}
		}
		return func(col arrow.Array, i int) any { return col.(*array.String).Value(i) }
	case arrow.BINARY:
		return func(col arrow.Array, i int) any { return string(col.(*array.Binary).Value(i)) }
	case arrow.DATE32:
		return func(col arrow.Array, i int) any {
			v := int64(col.(*array.Date32).Value(i))
			if v == math.MaxInt32 || v == math.MinInt32 { // Postgres infinity has no ClickHouse form
				return nil
			}
			return time.Unix(v*86400, 0).UTC()
		}
	case arrow.TIME64:
		return func(col arrow.Array, i int) any {
			return string(arrowtext.AppendTimeOfDay(nil, int64(col.(*array.Time64).Value(i))))
		}
	case arrow.TIMESTAMP:
		if f.Type.(*arrow.TimestampType).TimeZone != "" {
			return func(col arrow.Array, i int) any {
				us := int64(col.(*array.Timestamp).Value(i))
				if us == math.MaxInt64 || us == math.MinInt64 { // Postgres infinity has no ClickHouse form
					return nil
				}
				return time.Unix(us/arrowtext.MicrosPerSecond, us%arrowtext.MicrosPerSecond*1000).UTC()
			}
		}
		return func(col arrow.Array, i int) any {
			us := int64(col.(*array.Timestamp).Value(i))
			if us == math.MaxInt64 || us == math.MinInt64 {
				return nil
			}
			return string(arrowtext.AppendTimestamp(nil, us, ' '))
		}
	default:
		return func(col arrow.Array, i int) any { return col.ValueStr(i) }
	}
}
