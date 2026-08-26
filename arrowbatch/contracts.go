package arrowbatch

import (
	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament/rowmodel"
)

// Inlet is the source-facing entry point for opening typed row writers.
type Inlet interface {
	Builder(resource string, part int, schema rowmodel.Schema) (RowWriter, error)
}

// Receiver takes the owned batches a Builder produces. Chunk runs on the
// appending goroutine and may block to apply backpressure. On success ownership
// transfers to Receiver; on error the Builder retains ownership and releases
// the batch. Drained reports completion of the part after rows total rows.
type Receiver interface {
	Chunk(batch *Batch) error
	Drained(meta rowmodel.Meta, rows int) error
}

// RowWriter appends one schema-ordered row at a time.
type RowWriter interface {
	Null()
	Bool(bool)
	Int16(int16)
	Int32(int32)
	Int64(int64)
	Float32(float32)
	Float64(float64)
	Decimal(decimal128.Num)
	String(string)
	StringBytes([]byte)
	Bytes([]byte)
	Date(int32)
	Time(int64)
	Timestamp(int64)
	EndRow(rowmodel.Meta) error
	Flush() error
	Drain(rowmodel.Meta) error
	Close() error
}

var _ RowWriter = (*Builder)(nil)
