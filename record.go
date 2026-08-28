package filament

import (
	"github.com/galaxy-io/filament/rowmodel"
)

// Operation is the change kind a row carries.
type Operation = rowmodel.Operation

// The row operations.
const (
	OpInsert = rowmodel.OpInsert
	OpUpdate = rowmodel.OpUpdate
	OpDelete = rowmodel.OpDelete
)

// RecordSchema is one resource's column layout, as a source reports it and a
// Schematized sink consumes it.
type RecordSchema = rowmodel.Schema

// SchemaField describes one column: name, nullability, and its portable plus
// native types.
type SchemaField = rowmodel.Field

// WriteReceipt is what a sink returns per Apply: where the data landed, how
// much, and the write-side CRC for integrity comparison.
type WriteReceipt struct {
	URI   string
	Bytes int64
	Rows  int
	// WriteCRC is the sink's independently recomputed checksum over the borrowed
	// in-memory batch, immediately before its write boundary.
	WriteCRC uint32
	// EncodedCRC, when non-nil, is the checksum a sink verified over the exact
	// serialized bytes handed to its transport. Sinks advertising
	// SinkCapabilities.EncodedIntegrity must return it or the run fails.
	EncodedCRC *uint32
	Checkpoint *CheckpointData
}

// CheckpointData is the concrete backing for the Checkpoint interface — a
// resumable cursor. It implements Checkpoint with value-copy semantics on Set.
type CheckpointData = rowmodel.CheckpointData

// NewCheckpoint returns an empty checkpoint for a resource.
func NewCheckpoint(resource string) *CheckpointData {
	return rowmodel.NewCheckpoint(resource)
}

// CheckpointKind describes a cursor's shape without exposing its value.
func CheckpointKind(cp Checkpoint) string {
	if cp == nil {
		return "none"
	}
	if mode, ok := cp.Raw()["mode"].(string); ok && mode != "" {
		return mode
	}
	return "cursor"
}
