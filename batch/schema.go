// Package batch turns rows into Arrow record batches: the LogicalType to Arrow
// mapping, the Builder a source appends into, and the integrity CRC over a batch.
package batch

import (
	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament"
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
func Schema(rs filament.RecordSchema) *arrow.Schema {
	fields := make([]arrow.Field, len(rs.Fields))
	for i, f := range rs.Fields {
		fields[i] = Field(f)
	}
	return arrow.NewSchema(fields, nil)
}

// Field maps one schema field, keeping its logical and native types as metadata.
func Field(f filament.SchemaField) arrow.Field {
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
func Type(f filament.SchemaField) arrow.DataType {
	switch f.Logical {
	case filament.LogicalBool:
		return arrow.FixedWidthTypes.Boolean
	case filament.LogicalInt16:
		return arrow.PrimitiveTypes.Int16
	case filament.LogicalInt32:
		return arrow.PrimitiveTypes.Int32
	case filament.LogicalInt64:
		return arrow.PrimitiveTypes.Int64
	case filament.LogicalFloat32:
		return arrow.PrimitiveTypes.Float32
	case filament.LogicalFloat64:
		return arrow.PrimitiveTypes.Float64
	case filament.LogicalDecimal:
		if f.Precision > 0 && f.Precision <= maxDecimalPrecision {
			return &arrow.Decimal128Type{Precision: int32(f.Precision), Scale: int32(f.Scale)} //nolint:gosec // bounded above
		}
		return arrow.BinaryTypes.String
	case filament.LogicalBytes:
		return arrow.BinaryTypes.Binary
	case filament.LogicalDate:
		return arrow.FixedWidthTypes.Date32
	case filament.LogicalTime:
		return arrow.FixedWidthTypes.Time64us
	case filament.LogicalTimestamp:
		return &arrow.TimestampType{Unit: arrow.Microsecond}
	case filament.LogicalTimestampTZ:
		return &arrow.TimestampType{Unit: arrow.Microsecond, TimeZone: "UTC"}
	default:
		return arrow.BinaryTypes.String
	}
}

// LogicalOf returns the logical type a field was built from.
func LogicalOf(f arrow.Field) filament.LogicalType {
	v, _ := f.Metadata.GetValue(MetaLogical)
	return filament.LogicalType(v)
}

// NativeOf returns the source's native type spelling for a field.
func NativeOf(f arrow.Field) string {
	v, _ := f.Metadata.GetValue(MetaNative)
	return v
}
