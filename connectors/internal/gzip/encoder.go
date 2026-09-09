// Package gzipencoder wraps an encoder in one streaming gzip member.
package gzipencoder

import (
	"compress/gzip"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

// Encoder incrementally compresses batches belonging to one Arrow schema.
// EncodeBatch flushes the gzip writer so every call returns all bytes produced
// for that batch. Finalize must be called to append the gzip trailer.
type Encoder struct {
	json   encoder.Encoder
	out    encoder.Buffer
	gzip   *gzip.Writer
	raw    []byte
	closed bool
}

// NewEncoder prepares a gzip stream around a JSON encoder.
func NewEncoder(jsonEncoder encoder.Encoder) *Encoder {
	e := &Encoder{json: jsonEncoder}
	e.gzip = gzip.NewWriter(&e.out)
	return e
}

// EncodeBatch appends rows to the gzip stream and returns the compressed bytes
// emitted for this batch plus their independently verified CRC32C.
func (e *Encoder) EncodeBatch(dst []byte, rows arrow.RecordBatch) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("gzip: encode after finalize")
	}
	var err error
	e.raw, _, err = e.json.EncodeBatch(e.raw[:0], rows)
	if err != nil {
		return nil, 0, fmt.Errorf("gzip: encode content: %w", err)
	}
	if _, err := e.gzip.Write(e.raw); err != nil {
		return nil, 0, fmt.Errorf("gzip: compress batch: %w", err)
	}
	if err := e.gzip.Flush(); err != nil {
		return nil, 0, fmt.Errorf("gzip: flush batch: %w", err)
	}
	return e.out.Drain(dst, "gzip")
}

// Finalize closes the gzip stream and returns its trailer bytes.
func (e *Encoder) Finalize(dst []byte) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("gzip: already finalized")
	}
	e.closed = true
	var err error
	e.raw, _, err = e.json.Finalize(e.raw[:0])
	if err != nil {
		return nil, 0, fmt.Errorf("gzip: finalize content: %w", err)
	}
	if len(e.raw) > 0 {
		if _, err := e.gzip.Write(e.raw); err != nil {
			return nil, 0, fmt.Errorf("gzip: compress trailer: %w", err)
		}
	}
	if err := e.gzip.Close(); err != nil {
		return nil, 0, fmt.Errorf("gzip: finalize: %w", err)
	}
	return e.out.Drain(dst, "gzip")
}
