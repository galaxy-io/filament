package motherduck

// Every per-type behaviour of the sink, in one place: the DuckDB column type a
// schema field lands as, and for each Arrow storage type the Go value the
// appender receives for it.

import (
	"database/sql/driver"
	"fmt"
	"math"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	duckdb "github.com/marcboeker/go-duckdb/v2"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

const microsPerSecond = 1_000_000

// columnType maps a portable logical type to DuckDB SQL.
func columnType(field rowmodel.Field) string {
	switch field.Logical {
	case rowmodel.LogicalBool:
		return "BOOLEAN"
	case rowmodel.LogicalInt16:
		return "SMALLINT"
	case rowmodel.LogicalInt32:
		return "INTEGER"
	case rowmodel.LogicalInt64:
		return "BIGINT"
	case rowmodel.LogicalFloat32:
		return "FLOAT"
	case rowmodel.LogicalFloat64:
		return "DOUBLE"
	case rowmodel.LogicalDecimal:
		if field.Precision >= 1 && field.Precision <= 38 && field.Scale >= 0 && field.Scale <= field.Precision {
			return fmt.Sprintf("DECIMAL(%d, %d)", field.Precision, field.Scale)
		}
		return "VARCHAR"
	case rowmodel.LogicalBytes:
		return "BLOB"
	case rowmodel.LogicalDate:
		return "DATE"
	case rowmodel.LogicalTime:
		return "TIME"
	case rowmodel.LogicalTimestamp:
		return "TIMESTAMP"
	case rowmodel.LogicalTimestampTZ:
		return "TIMESTAMPTZ"
	case rowmodel.LogicalUUID:
		return "UUID"
	default:
		return "VARCHAR"
	}
}

// The widest finite timestamps a Postgres infinity sentinel lands as in a
// NOT NULL column. DuckDB timestamp infinity cannot be delivered instead:
// go-duckdb's appender takes time.Time only and rejects the sentinel years,
// and a nil would fail the whole batch at flush.
var (
	maxTimestampValue = time.Date(294246, 12, 31, 23, 59, 59, 999999000, time.UTC)
	minTimestampValue = time.Date(-290307, 1, 1, 0, 0, 0, 0, time.UTC)
)

// valueFn returns row i of a column as the value the appender receives.
type valueFn func(col arrow.Array, i int) driver.Value

// valueFor picks a field's converter from its Arrow storage type. Temporal
// values go as UTC time.Time, decimals keep their unscaled integer and scale,
// UUIDs are parsed, and json, arrays and unknown values stay text.
func valueFor(f arrow.Field) valueFn {
	switch f.Type.ID() {
	case arrow.BOOL:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Boolean).Value(i) }
	case arrow.INT16:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Int16).Value(i) }
	case arrow.INT32:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Int32).Value(i) }
	case arrow.INT64:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Int64).Value(i) }
	case arrow.FLOAT32:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Float32).Value(i) }
	case arrow.FLOAT64:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Float64).Value(i) }
	case arrow.DECIMAL128:
		typ := f.Type.(*arrow.Decimal128Type)
		width, scale := uint8(typ.Precision), uint8(typ.Scale) //nolint:gosec // bounded by the arrow type
		return func(col arrow.Array, i int) driver.Value {
			return duckdb.Decimal{Width: width, Scale: scale, Value: col.(*array.Decimal128).Value(i).BigInt()}
		}
	case arrow.STRING:
		if arrowbatch.LogicalOf(f) == rowmodel.LogicalUUID {
			return func(col arrow.Array, i int) driver.Value {
				var id duckdb.UUID
				text := col.(*array.String).Value(i)
				if err := id.Scan(text); err != nil {
					return text // the appender reports the malformed value
				}
				return id
			}
		}
		return func(col arrow.Array, i int) driver.Value { return col.(*array.String).Value(i) }
	case arrow.BINARY:
		return func(col arrow.Array, i int) driver.Value { return col.(*array.Binary).Value(i) }
	case arrow.DATE32:
		return func(col arrow.Array, i int) driver.Value {
			days := int64(col.(*array.Date32).Value(i))
			if days == math.MinInt32 {
				// Postgres -infinity is int32 min, but DuckDB -infinity
				// is -int32 max. Shift one day so the value lands as
				// -infinity, not as a finite date in 5877642 BC.
				// Postgres infinity is int32 max, which is DuckDB
				// infinity too, so it passes through unchanged.
				days = -math.MaxInt32
			}
			return time.Unix(days*86400, 0).UTC()
		}
	case arrow.TIME64:
		return func(col arrow.Array, i int) driver.Value {
			return time.Unix(0, 0).UTC().Add(time.Duration(col.(*array.Time64).Value(i)) * time.Microsecond)
		}
	case arrow.TIMESTAMP:
		return func(col arrow.Array, i int) driver.Value {
			us := int64(col.(*array.Timestamp).Value(i))
			switch us {
			case math.MaxInt64: // Postgres infinity
				if f.Nullable {
					return nil
				}
				return maxTimestampValue
			case math.MinInt64: // Postgres -infinity
				if f.Nullable {
					return nil
				}
				return minTimestampValue
			}
			return time.Unix(us/microsPerSecond, us%microsPerSecond*1000).UTC()
		}
	default:
		return func(col arrow.Array, i int) driver.Value { return col.ValueStr(i) }
	}
}
