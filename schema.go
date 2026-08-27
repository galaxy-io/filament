package filament

import (
	"context"

	"github.com/galaxy-io/filament/rowmodel"
)

// LogicalType is a portable, engine-independent column type. A schema-aware source
// classifies each column into one of these so a sink can map to its own type system;
// alongside it SchemaField.Native carries the source's exact type for a same-engine
// round-trip. Decimal precision/scale and array element types live in Native, not here.
type LogicalType = rowmodel.LogicalType

// The portable column types.
const (
	LogicalUnknown     = rowmodel.LogicalUnknown
	LogicalBool        = rowmodel.LogicalBool
	LogicalInt16       = rowmodel.LogicalInt16
	LogicalInt32       = rowmodel.LogicalInt32
	LogicalInt64       = rowmodel.LogicalInt64
	LogicalFloat32     = rowmodel.LogicalFloat32
	LogicalFloat64     = rowmodel.LogicalFloat64
	LogicalDecimal     = rowmodel.LogicalDecimal
	LogicalString      = rowmodel.LogicalString
	LogicalBytes       = rowmodel.LogicalBytes
	LogicalDate        = rowmodel.LogicalDate
	LogicalTime        = rowmodel.LogicalTime
	LogicalTimestamp   = rowmodel.LogicalTimestamp
	LogicalTimestampTZ = rowmodel.LogicalTimestampTZ
	LogicalJSON        = rowmodel.LogicalJSON
	LogicalUUID        = rowmodel.LogicalUUID
	LogicalArray       = rowmodel.LogicalArray
)

// SchemaProvider is an optional Source capability: it returns the column schema of a
// single resource. It is resource-scoped (lighter than the browse-style Discover) so
// the engine can fetch exactly the requested resources' schemas before extraction to
// drive a Schematized sink's typed DDL.
type SchemaProvider interface {
	Schema(ctx context.Context, resource string) (RecordSchema, error)
}
