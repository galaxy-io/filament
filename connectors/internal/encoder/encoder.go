// Package encoder defines the streaming encoder contract and object formats.
package encoder

import (
	"bytes"
	"fmt"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow"
)

// Encoder emits successive byte ranges for batches and a final byte range for
// the stream trailer. Callers must concatenate ranges in call order.
type Encoder interface {
	EncodeBatch([]byte, arrow.RecordBatch) ([]byte, uint32, error)
	Finalize([]byte) ([]byte, uint32, error)
}

var crcTable = crc32.MakeTable(crc32.Castagnoli)

// Buffer captures bytes emitted by a stateful stream writer and accumulates
// their CRC32C as they are produced.
type Buffer struct {
	buf bytes.Buffer
	crc uint32
}

// Write implements io.Writer.
func (b *Buffer) Write(p []byte) (int, error) {
	n, err := b.buf.Write(p)
	b.crc = crc32.Update(b.crc, crcTable, p[:n])
	return n, err
}

// Drain appends all pending bytes to dst, resets the buffer, and independently
// verifies the checksum over exactly the newly appended bytes.
func (b *Buffer) Drain(dst []byte, format string) ([]byte, uint32, error) {
	start := len(dst)
	dst = append(dst, b.buf.Bytes()...)
	expected := b.crc
	b.buf.Reset()
	b.crc = 0

	actual := crc32.Checksum(dst[start:], crcTable)
	if actual != expected {
		return nil, 0, fmt.Errorf("%s: serialized CRC divergence: encoded %08x, expected %08x", format, actual, expected)
	}
	return dst, actual, nil
}
