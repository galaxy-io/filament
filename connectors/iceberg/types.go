package iceberg

// Every per-type behaviour of the sink, in one place: the Iceberg type a schema
// field lands as, how a batch's Arrow columns conform to the table's storage
// types, and how a key value binds into an Iceberg equality expression.

import (
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/extensions"
	"github.com/apache/arrow-go/v18/arrow/memory"
	iceberg "github.com/apache/iceberg-go"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament/rowmodel"
)

// logicalToIceType maps a schema field to an Iceberg type.
func logicalToIceType(f rowmodel.Field) iceberg.Type {
	switch f.Logical {
	case rowmodel.LogicalBool:
		return iceberg.PrimitiveTypes.Bool
	case rowmodel.LogicalInt16, rowmodel.LogicalInt32:
		return iceberg.PrimitiveTypes.Int32
	case rowmodel.LogicalInt64:
		return iceberg.PrimitiveTypes.Int64
	case rowmodel.LogicalFloat32:
		return iceberg.PrimitiveTypes.Float32
	case rowmodel.LogicalFloat64:
		return iceberg.PrimitiveTypes.Float64
	case rowmodel.LogicalDecimal:
		if f.Precision > 0 {
			return iceberg.DecimalTypeOf(f.Precision, f.Scale)
		}
		return iceberg.DecimalTypeOf(defaultDecimalPrec, defaultDecimalScale)
	case rowmodel.LogicalBytes:
		return iceberg.PrimitiveTypes.Binary
	case rowmodel.LogicalDate:
		return iceberg.PrimitiveTypes.Date
	case rowmodel.LogicalTime:
		return iceberg.PrimitiveTypes.Time
	case rowmodel.LogicalTimestamp:
		return iceberg.PrimitiveTypes.Timestamp
	case rowmodel.LogicalTimestampTZ:
		return iceberg.PrimitiveTypes.TimestampTz
	case rowmodel.LogicalUUID:
		return iceberg.PrimitiveTypes.UUID
	default:
		// string, json, array (native literal text), unknown
		return iceberg.PrimitiveTypes.String
	}
}

// An unbounded decimal has no declared precision; Iceberg requires one, so it
// lands as decimal(38,9) and its text is parsed to that scale.
const (
	defaultDecimalPrec  = 38
	defaultDecimalScale = 9
)

// conform rebuilds rows in the table's Arrow schema (field ids in metadata, the
// Iceberg storage types): columns already of the right type are shared, the few
// that differ are cast — int16 to int32 (Iceberg has no int16), utf8 to the uuid
// extension, and utf8 numeric text to decimal — and columns the batch lacks are
// null. The result is the caller's to release.
func conform(rows arrow.RecordBatch, target *arrow.Schema) (arrow.RecordBatch, error) {
	src := rows.Schema()
	n := int(rows.NumRows())
	cols := make([]arrow.Array, target.NumFields())
	for i, f := range target.Fields() {
		idx := src.FieldIndices(f.Name)
		if len(idx) == 0 {
			cols[i] = array.MakeArrayOfNull(memory.DefaultAllocator, f.Type, n)
			continue
		}
		col := rows.Column(idx[0])
		out, err := cast(col, f.Type)
		if err != nil {
			releaseAll(cols[:i])
			return nil, fmt.Errorf("column %q: %w", f.Name, err)
		}
		cols[i] = out
	}
	rec := array.NewRecordBatch(target, cols, int64(n))
	releaseAll(cols)
	return rec, nil
}

func releaseAll(cols []arrow.Array) {
	for _, c := range cols {
		if c != nil {
			c.Release()
		}
	}
}

// cast returns col as type want, retained: the same array when the types agree,
// else a converted copy.
func cast(col arrow.Array, want arrow.DataType) (arrow.Array, error) {
	if arrow.TypeEqual(col.DataType(), want) {
		col.Retain()
		return col, nil
	}
	mem := memory.DefaultAllocator
	n := col.Len()
	switch src := col.(type) {
	case *array.Int16:
		if want.ID() == arrow.INT32 {
			b := array.NewInt32Builder(mem)
			defer b.Release()
			b.Reserve(n)
			for i := range n {
				if src.IsNull(i) {
					b.AppendNull()
				} else {
					b.Append(int32(src.Value(i)))
				}
			}
			return b.NewArray(), nil
		}
	case *array.String:
		switch w := want.(type) {
		case *extensions.UUIDType:
			b := extensions.NewUUIDBuilder(mem)
			defer b.Release()
			b.Reserve(n)
			for i := range n {
				if src.IsNull(i) {
					b.AppendNull()
					continue
				}
				u, err := uuid.Parse(src.Value(i))
				if err != nil {
					return nil, fmt.Errorf("%q is not a uuid", src.Value(i))
				}
				b.Append(u)
			}
			return b.NewArray(), nil
		case *arrow.Decimal128Type:
			b := array.NewDecimal128Builder(mem, w)
			defer b.Release()
			b.Reserve(n)
			for i := range n {
				if src.IsNull(i) {
					b.AppendNull()
					continue
				}
				v, err := decimal128.FromString(src.Value(i), w.Precision, w.Scale)
				if err != nil {
					return nil, fmt.Errorf("%q has no decimal(%d,%d) form", src.Value(i), w.Precision, w.Scale)
				}
				b.Append(v)
			}
			return b.NewArray(), nil
		}
	case *array.Timestamp:
		// Same unit, different zone spelling: share the buffers under the wanted type.
		if w, ok := want.(*arrow.TimestampType); ok && w.Unit == src.DataType().(*arrow.TimestampType).Unit {
			data := array.NewData(w, n, src.Data().Buffers(), nil, src.NullN(), src.Data().Offset())
			defer data.Release()
			return array.MakeFromData(data), nil
		}
	}
	return nil, fmt.Errorf("cannot write %s as %s", col.DataType(), want)
}

// keyLiteral returns row i's value of a key column as the Go literal an Iceberg
// equality expression accepts.
func keyLiteral(col arrow.Array, i int) (any, error) {
	if col.IsNull(i) {
		return nil, fmt.Errorf("primary key value is null")
	}
	switch c := col.(type) {
	case *array.Boolean:
		return c.Value(i), nil
	case *array.Int16:
		return int32(c.Value(i)), nil
	case *array.Int32:
		return c.Value(i), nil
	case *array.Int64:
		return c.Value(i), nil
	case *array.Float32:
		return c.Value(i), nil
	case *array.Float64:
		return c.Value(i), nil
	case *array.Binary:
		return c.Value(i), nil
	case *array.String:
		return c.Value(i), nil
	default:
		return c.ValueStr(i), nil
	}
}

func equalExpr(key string, value any) (iceberg.BooleanExpression, error) {
	ref := iceberg.Reference(key)
	switch v := value.(type) {
	case bool:
		return iceberg.EqualTo(ref, v), nil
	case int32:
		return iceberg.EqualTo(ref, v), nil
	case int64:
		return iceberg.EqualTo(ref, v), nil
	case float32:
		return iceberg.EqualTo(ref, v), nil
	case float64:
		return iceberg.EqualTo(ref, v), nil
	case string:
		return iceberg.EqualTo(ref, v), nil
	case []byte:
		return iceberg.EqualTo(ref, v), nil
	case uuid.UUID:
		return iceberg.EqualTo(ref, v), nil
	default:
		return nil, fmt.Errorf("unsupported iceberg key literal %T for %q", value, key)
	}
}
