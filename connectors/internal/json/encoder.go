// Package jsonencoder renders Arrow rows as line-delimited JSON or one JSON
// array for sinks that write text such as stdout and object storage.
package jsonencoder

import (
	"fmt"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow"
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Encoder renders rows of one Arrow schema. Column keys and value functions are
// prepared once and reused for every batch with that schema.
type Encoder struct {
	schema *arrow.Schema
	keys   []string // `"name":` per column
	vals   []valueFn
	array  bool
	wrote  bool
	closed bool
}

// NewEncoder prepares an encoder for schema.
func NewEncoder(schema *arrow.Schema) *Encoder {
	return newEncoder(schema, false)
}

// NewArrayEncoder prepares an encoder that frames rows in one JSON array.
func NewArrayEncoder(schema *arrow.Schema) *Encoder {
	return newEncoder(schema, true)
}

func newEncoder(schema *arrow.Schema, array bool) *Encoder {
	e := &Encoder{schema: schema, keys: make([]string, schema.NumFields()), vals: make([]valueFn, schema.NumFields()), array: array}
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
	if e.closed {
		return nil, 0, fmt.Errorf("json: encode after finalize")
	}
	if e.schema == nil || !rows.Schema().Equal(e.schema) {
		return nil, 0, fmt.Errorf("json: record schema does not match encoder schema")
	}
	if e.array {
		return e.encodeArrayBatch(dst, rows)
	}
	start := len(dst)
	encoded, expected := e.AppendBatch(dst, rows)
	actual, err := VerifyChecksum(encoded[start:], expected)
	if err != nil {
		return nil, 0, err
	}
	return encoded, actual, nil
}

func (e *Encoder) encodeArrayBatch(dst []byte, rows arrow.RecordBatch) ([]byte, uint32, error) {
	start := len(dst)
	for i := range int(rows.NumRows()) {
		if !e.wrote {
			dst = append(dst, '[')
			e.wrote = true
		} else {
			dst = append(dst, ',')
		}
		dst = e.AppendRow(dst, rows, i)
	}
	return dst, crc32.Checksum(dst[start:], crcTable), nil
}

// Finalize closes an array-framed stream. Line-delimited streams have no footer.
func (e *Encoder) Finalize(dst []byte) ([]byte, uint32, error) {
	if e.closed {
		return nil, 0, fmt.Errorf("json: already finalized")
	}
	e.closed = true
	if !e.array {
		return dst, 0, nil
	}
	start := len(dst)
	if e.wrote {
		dst = append(dst, ']')
	} else {
		dst = append(dst, '[', ']')
	}
	return dst, crc32.Checksum(dst[start:], crcTable), nil
}

// VerifyChecksum checks encoded against the checksum accumulated while it was
// produced. It is exposed for transports that stage or transform buffers before
// their final write boundary.
func VerifyChecksum(encoded []byte, expected uint32) (uint32, error) {
	actual := Checksum(encoded)
	if actual != expected {
		return 0, fmt.Errorf("json: serialized CRC divergence: encoded %08x, expected %08x", actual, expected)
	}
	return actual, nil
}

// Checksum returns the CRC32C of encoded bytes.
func Checksum(encoded []byte) uint32 { return crc32.Checksum(encoded, crcTable) }
