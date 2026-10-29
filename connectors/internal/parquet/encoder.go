// Package parquet incrementally renders Arrow record batches as one Parquet file.
package parquet

import (
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"
	arrowparquet "github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/compress"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"

	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

// Limit buffered row groups independently of source transaction sizes.
const maxRowGroupRows int64 = 64 * 1024

// Encoder combines batches into bounded row groups. EncodeBatch returns any
// bytes emitted so far; Finalize flushes the last group and Parquet footer.
type Encoder struct {
	schema *arrow.Schema
	out    encoder.Buffer
	writer *pqarrow.FileWriter
	closed bool
}

// NewEncoder prepares a compressed Parquet encoder for schema. The
// original Arrow schema is stored in file metadata so Filament logical-type
// metadata and timestamp timezone information survive a round trip.
func NewEncoder(schema *arrow.Schema, compression encoder.Compression) (*Encoder, error) {
	if schema == nil {
		return nil, fmt.Errorf("parquet: schema is required")
	}
	if compression != encoder.CompressionSnappy {
		return nil, fmt.Errorf("parquet: unsupported compression %q", compression)
	}
	e := &Encoder{schema: schema}
	writer, err := pqarrow.NewFileWriter(
		schema,
		&e.out,
		arrowparquet.NewWriterProperties(
			arrowparquet.WithCompression(compress.Codecs.Snappy),
			arrowparquet.WithMaxRowGroupLength(maxRowGroupRows),
		),
		pqarrow.NewArrowWriterProperties(pqarrow.WithStoreSchema()),
	)
	if err != nil {
		return nil, fmt.Errorf("parquet: create writer: %w", err)
	}
	e.writer = writer
	return e, nil
}

// EncodeBatch buffers rows across batches and returns any emitted bytes plus
// their independently verified CRC32C. A batch can emit no bytes.
func (e *Encoder) EncodeBatch(dst []byte, rows arrow.RecordBatch) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("parquet: encode after finalize")
	}
	if !rows.Schema().Equal(e.schema) {
		return nil, 0, fmt.Errorf("parquet: record schema does not match encoder schema")
	}
	if err := e.writer.WriteBuffered(rows); err != nil {
		return nil, 0, fmt.Errorf("parquet: encode batch: %w", err)
	}
	return e.out.Drain(dst, "parquet")
}

// Finalize flushes the last row group and returns it with the Parquet footer.
func (e *Encoder) Finalize(dst []byte) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("parquet: already finalized")
	}
	e.closed = true
	if err := e.writer.Close(); err != nil {
		return nil, 0, fmt.Errorf("parquet: finalize: %w", err)
	}
	return e.out.Drain(dst, "parquet")
}
