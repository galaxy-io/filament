// Package ndjson renders Arrow rows as JSON objects, one per line, for sinks
// that write text such as stdout and object storage.
package ndjson

import (
	"fmt"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Encoder renders rows of one Arrow schema. Column keys and value functions are
// prepared once and reused for every batch with that schema.
type Encoder struct {
	keys []string // `"name":` per column
	vals []valueFn
}

// NewEncoder prepares an encoder for schema.
func NewEncoder(schema *arrow.Schema) *Encoder {
	e := &Encoder{keys: make([]string, schema.NumFields()), vals: make([]valueFn, schema.NumFields())}
	for i, field := range schema.Fields() {
		e.keys[i] = string(appendString(nil, field.Name)) + ":"
		e.vals[i] = valueFor(field)
	}
	return e
}

// AppendRow appends row i as one JSON object without a trailing newline.
func (e *Encoder) AppendRow(dst []byte, rows arrow.RecordBatch, i int) []byte {
	dst = append(dst, '{')
	for column, values := range rows.Columns() {
		if column > 0 {
			dst = append(dst, ',')
		}
		dst = append(dst, e.keys[column]...)
		if values.IsNull(i) {
			dst = append(dst, "null"...)
			continue
		}
		dst = e.vals[column](dst, values, i)
	}
	return append(dst, '}')
}

// AppendBatch appends every row as NDJSON, including one newline per row, and
// returns the incrementally accumulated CRC32C of the newly appended bytes.
func (e *Encoder) AppendBatch(dst []byte, rows arrow.RecordBatch) ([]byte, uint32) {
	var crc uint32
	for i := range int(rows.NumRows()) {
		start := len(dst)
		dst = e.AppendRow(dst, rows, i)
		dst = append(dst, '\n')
		crc = crc32.Update(crc, crcTable, dst[start:])
	}
	return dst, crc
}

// EncodeBatch appends and verifies one batch, returning the checksum over the
// exact newly encoded bytes. Byte-oriented sinks use this immediately before
// handing the returned buffer to their transport.
func (e *Encoder) EncodeBatch(dst []byte, rows arrow.RecordBatch) ([]byte, uint32, error) {
	start := len(dst)
	encoded, expected := e.AppendBatch(dst, rows)
	actual, err := VerifyChecksum(encoded[start:], expected)
	if err != nil {
		return nil, 0, err
	}
	return encoded, actual, nil
}

// VerifyChecksum checks encoded against the checksum accumulated while it was
// produced. It is exposed for transports that stage or transform buffers before
// their final write boundary.
func VerifyChecksum(encoded []byte, expected uint32) (uint32, error) {
	actual := Checksum(encoded)
	if actual != expected {
		return 0, fmt.Errorf("ndjson: serialized CRC divergence: encoded %08x, expected %08x", actual, expected)
	}
	return actual, nil
}

// Checksum returns the CRC32C of encoded bytes.
func Checksum(encoded []byte) uint32 { return crc32.Checksum(encoded, crcTable) }
