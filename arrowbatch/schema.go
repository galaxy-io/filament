// Package arrowbatch owns Arrow row batches, builders, schema mapping, and
// in-memory integrity checks.
package arrowbatch

import (
	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/rowmodel"
)

// Field metadata keys carrying the source-side type of each column, so a sink can
// map back from the Arrow storage type (uuid, json and unbounded numeric all travel
// as utf8) and a same-engine sink can reuse the native spelling.
const (
	MetaLogical = "filament.logical"
	MetaNative  = "filament.native"
)

// maxDecimalPrecision is the widest decimal that fits decimal128.
const maxDecimalPrecision = 38

// Schema maps a RecordSchema to the Arrow schema its rows are built in.
func Schema(rs rowmodel.Schema) *arrow.Schema {
	fields := make([]arrow.Field, len(rs.Fields))
	for i, f := range rs.Fields {
		fields[i] = Field(f)
	}
	return arrow.NewSchema(fields, nil)
}

// Field maps one schema field, keeping its logical and native types as metadata.
func Field(f rowmodel.Field) arrow.Field {
	return arrow.Field{
		Name:     f.Name,
		Type:     Type(f),
		Nullable: f.Nullable,
		Metadata: arrow.NewMetadata([]string{MetaLogical, MetaNative}, []string{string(f.Logical), f.Native}),
	}
}

// Type maps a field's logical type to Arrow storage. Everything a sink cannot
// address natively (json, uuid, array, unknown, unbounded decimal) travels as
// utf8 so it is never reinterpreted in flight.
func Type(f rowmodel.Field) arrow.DataType {
	switch f.Logical {
	case rowmodel.LogicalBool:
		return arrow.FixedWidthTypes.Boolean
	case rowmodel.LogicalInt16:
		return arrow.PrimitiveTypes.Int16
	case rowmodel.LogicalInt32:
		return arrow.PrimitiveTypes.Int32
	case rowmodel.LogicalInt64:
		return arrow.PrimitiveTypes.Int64
	case rowmodel.LogicalFloat32:
		return arrow.PrimitiveTypes.Float32
	case rowmodel.LogicalFloat64:
		return arrow.PrimitiveTypes.Float64
	case rowmodel.LogicalDecimal:
		if f.Precision > 0 && f.Precision <= maxDecimalPrecision {
			return &arrow.Decimal128Type{Precision: int32(f.Precision), Scale: int32(f.Scale)} //nolint:gosec // bounded above
		}
		return arrow.BinaryTypes.String
	case rowmodel.LogicalBytes:
		return arrow.BinaryTypes.Binary
	case rowmodel.LogicalDate:
		return arrow.FixedWidthTypes.Date32
	case rowmodel.LogicalTime:
		return arrow.FixedWidthTypes.Time64us
	case rowmodel.LogicalTimestamp:
		return &arrow.TimestampType{Unit: arrow.Microsecond}
	case rowmodel.LogicalTimestampTZ:
		return &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}
	default:
		return arrow.BinaryTypes.String
	}
}

// LogicalOf returns the logical type a field was built from.
func LogicalOf(f arrow.Field) rowmodel.LogicalType {
	v, _ := f.Metadata.GetValue(MetaLogical)
	return rowmodel.LogicalType(v)
}

// NativeOf returns the source's native type spelling for a field.
func NativeOf(f arrow.Field) string {
	v, _ := f.Metadata.GetValue(MetaNative)
	return v
}
