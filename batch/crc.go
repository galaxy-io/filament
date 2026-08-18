package batch

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"

	"github.com/galaxy-io/filament"
)

// crcTable uses the Castagnoli polynomial, which has hardware acceleration on
// both ARM64 and x86 (SSE4.2).
var crcTable = crc32.MakeTable(crc32.Castagnoli)

// CRC returns the CRC32C over a batch's rows and ops. It hashes each column's
// buffers as laid out (validity bitmap, offsets, values, over the batch's own row
// range), each prefixed with its length, and the row and column counts, so
// regrouping rows or columns changes the result. The pipeline writer computes it
// before Apply and the sink recomputes it over the same batch for the receipt; it
// guards the batch in memory, not a re-encoding of it.
func CRC(rows arrow.RecordBatch, ops []filament.Operation) uint32 {
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[:4], uint32(rows.NumRows())) //nolint:gosec // batch rows never near 4Gi
	binary.LittleEndian.PutUint32(buf[4:], uint32(rows.NumCols())) //nolint:gosec // column count
	crc := crc32.Update(0, crcTable, buf[:])
	for _, col := range rows.Columns() {
		crc = crcArray(crc, col)
	}
	for _, op := range ops {
		buf[0] = byte(op)
		crc = crc32.Update(crc, crcTable, buf[:1])
	}
	return crc
}

func crcArray(crc uint32, a arrow.Array) uint32 {
	data := a.Data()
	off, n := data.Offset(), data.Len()
	if a.NullN() > 0 {
		crc = crcChunk(crc, bitRange(data.Buffers()[0].Bytes(), off, n))
	}
	switch v := a.(type) {
	case *array.Boolean:
		crc = crcChunk(crc, bitRange(data.Buffers()[1].Bytes(), off, n))
	case *array.String:
		crc = crcChunk(crc, arrow.Int32Traits.CastToBytes(v.ValueOffsets()))
		crc = crcChunk(crc, v.ValueBytes())
	case *array.Binary:
		crc = crcChunk(crc, arrow.Int32Traits.CastToBytes(v.ValueOffsets()))
		crc = crcChunk(crc, v.ValueBytes())
	default:
		w := a.DataType().(arrow.FixedWidthDataType).Bytes()
		crc = crcChunk(crc, data.Buffers()[1].Bytes()[off*w:(off+n)*w])
	}
	return crc
}

// bitRange returns the bytes of a bitmap covering bits [off, off+n).
func bitRange(bits []byte, off, n int) []byte {
	if len(bits) == 0 {
		return nil
	}
	return bits[off/8 : (off+n+7)/8]
}

func crcChunk(crc uint32, p []byte) uint32 {
	var l [4]byte
	binary.LittleEndian.PutUint32(l[:], uint32(len(p))) //nolint:gosec // buffer sizes are far below 4GiB
	crc = crc32.Update(crc, crcTable, l[:])
	return crc32.Update(crc, crcTable, p)
}

// Bytes returns the memory a batch's column buffers occupy.
func Bytes(rows arrow.RecordBatch) int64 {
	var n int64
	for _, col := range rows.Columns() {
		for _, buf := range col.Data().Buffers() {
			if buf != nil {
				n += int64(buf.Len())
			}
		}
	}
	return n
}
