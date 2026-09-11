// Package rowmodel defines the connector-neutral row schema and per-row metadata.
// It deliberately has no dependency on Arrow or the root filament package.
package rowmodel

import "slices"

// LogicalType is a portable, engine-independent column type.
type LogicalType string

// Portable column types.
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

// Operation is the change kind carried by a row.
type Operation uint8

// Row operations.
const (
	OpInsert Operation = iota
	OpUpdate
	OpDelete
)

// Schema is one resource's column layout.
type Schema struct {
	Resource   string
	Fields     []Field
	PrimaryKey []string
	Engine     string
}

// Equal reports whether two supplied schemas describe the same resource model.
func (s Schema) Equal(other Schema) bool {
	return s.Resource == other.Resource && s.Engine == other.Engine &&
		slices.Equal(s.PrimaryKey, other.PrimaryKey) && slices.Equal(s.Fields, other.Fields)
}

// Clone returns an independently owned schema. Connectors commonly reuse and
// mutate discovery buffers, so registries must not retain their backing slices.
func (s Schema) Clone() Schema {
	s.Fields = slices.Clone(s.Fields)
	s.PrimaryKey = slices.Clone(s.PrimaryKey)
	return s
}

// Field describes one portable column and its source-native spelling.
type Field struct {
	Name      string
	Nullable  bool
	Logical   LogicalType
	Native    string
	Precision int
	Scale     int
}

// Meta accompanies a row without becoming an Arrow column.
type Meta struct {
	Domain   DomainKey
	Position Position
	Ordinal  uint64
	Op       Operation
	Key      []string
	Coarse   bool
	LSN      string
	Seq      uint64
}
