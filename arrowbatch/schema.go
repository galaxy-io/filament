// Package arrowbatch owns Arrow row batches, builders, schema mapping, and
// in-memory integrity checks.
package arrowbatch

import (
	"fmt"

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

// FieldOf inverts Field; a filament.logical hint must match the storage type.
func FieldOf(af arrow.Field) (rowmodel.Field, error) {
	f := rowmodel.Field{Name: af.Name, Nullable: af.Nullable, Logical: LogicalOf(af), Native: NativeOf(af)}
	if dec, ok := af.Type.(*arrow.Decimal128Type); ok {
		f.Precision, f.Scale = int(dec.Precision), int(dec.Scale)
	}
	if f.Logical != rowmodel.LogicalUnknown && !f.Logical.Known() {
		return rowmodel.Field{}, fmt.Errorf("field %q: %q is not a filament logical type", af.Name, f.Logical)
	}
	if f.Logical == rowmodel.LogicalUnknown {
		logical, ok := logicalForStorage(af.Type)
		if !ok {
			return rowmodel.Field{}, fmt.Errorf("field %q: Arrow type %s has no filament type", af.Name, af.Type)
		}
		f.Logical = logical
	}
	if want := Type(f); !arrow.TypeEqual(want, af.Type) {
		return rowmodel.Field{}, fmt.Errorf("field %q: logical type %q is stored as %s, not %s", af.Name, f.Logical, want, af.Type)
	}
	return f, nil
}

// logicalForStorage is the logical type Type maps to s when no hint says otherwise.
func logicalForStorage(s arrow.DataType) (rowmodel.LogicalType, bool) {
	switch s.ID() {
	case arrow.BOOL:
		return rowmodel.LogicalBool, true
	case arrow.INT16:
		return rowmodel.LogicalInt16, true
	case arrow.INT32:
		return rowmodel.LogicalInt32, true
	case arrow.INT64:
		return rowmodel.LogicalInt64, true
	case arrow.FLOAT32:
		return rowmodel.LogicalFloat32, true
	case arrow.FLOAT64:
		return rowmodel.LogicalFloat64, true
	case arrow.DECIMAL128:
		return rowmodel.LogicalDecimal, true
	case arrow.STRING:
		return rowmodel.LogicalString, true
	case arrow.BINARY:
		return rowmodel.LogicalBytes, true
	case arrow.DATE32:
		return rowmodel.LogicalDate, true
	case arrow.TIME64:
		return rowmodel.LogicalTime, true
	case arrow.TIMESTAMP:
		if s.(*arrow.TimestampType).TimeZone == "" {
			return rowmodel.LogicalTimestamp, true
		}
		return rowmodel.LogicalTimestampTZ, true
	}
	return rowmodel.LogicalUnknown, false
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
