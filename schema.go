package filament

import "context"

// LogicalType is a portable, engine-independent column type. A schema-aware source
// classifies each column into one of these so a sink can map to its own type system;
// alongside it SchemaField.Native carries the source's exact type for a same-engine
// round-trip. Decimal precision/scale and array element types live in Native, not here.
type LogicalType string

// The portable column types.
const (
	LogicalUnknown     LogicalType = ""
	LogicalBool        LogicalType = "bool"
	LogicalInt16       LogicalType = "int16"
	LogicalInt32       LogicalType = "int32"
	LogicalInt64       LogicalType = "int64"
	LogicalFloat32     LogicalType = "float32"
	LogicalFloat64     LogicalType = "float64"
	LogicalDecimal     LogicalType = "decimal"
	LogicalString      LogicalType = "string"
	LogicalBytes       LogicalType = "bytes"
	LogicalDate        LogicalType = "date"
	LogicalTime        LogicalType = "time"
	LogicalTimestamp   LogicalType = "timestamp"
	LogicalTimestampTZ LogicalType = "timestamptz"
	LogicalJSON        LogicalType = "json"
	LogicalUUID        LogicalType = "uuid"
	LogicalArray       LogicalType = "array"
)

// SchemaProvider is an optional Source capability: it returns the column schema of a
// single resource. It is resource-scoped (lighter than the browse-style Discover) so
// the engine can fetch exactly the requested resources' schemas before extraction to
// drive a Schematized sink's typed DDL.
type SchemaProvider interface {
	Schema(ctx context.Context, resource string) (RecordSchema, error)
}
