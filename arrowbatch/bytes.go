package arrowbatch

import (
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
)

// Bytes returns the size of a record batch's column buffers over its own row range, so a slice counts only its rows.
func Bytes(rows arrow.RecordBatch) int64 {
	var size int64
	for _, col := range rows.Columns() {
		size += arrayBytes(col)
	}
	return size
}

func arrayBytes(a arrow.Array) int64 {
	data := a.Data()
	off, n := data.Offset(), data.Len()
	var size int
	if bitmap := data.Buffers()[0]; a.NullN() > 0 && bitmap != nil {
		size += len(bitRange(bitmap.Bytes(), off, n))
	}
	switch v := a.(type) {
	case *array.Boolean:
		size += len(bitRange(data.Buffers()[1].Bytes(), off, n))
	case *array.String:
		size += arrow.Int32SizeBytes*len(v.ValueOffsets()) + len(v.ValueBytes())
	case *array.Binary:
		size += arrow.Int32SizeBytes*len(v.ValueOffsets()) + len(v.ValueBytes())
	default:
		fw, ok := a.DataType().(arrow.FixedWidthDataType)
		if !ok {
			// Lists, large strings and other types count their whole buffers.
			return bufferBytes(data)
		}
		size += n * fw.Bytes()
	}
	return int64(size)
}

func bufferBytes(data arrow.ArrayData) int64 {
	var size int64
	for _, buf := range data.Buffers() {
		if buf != nil {
			size += int64(buf.Len())
		}
	}
	return size
}

// Bytes returns the size of b's Arrow buffers over its own rows.
func (b *Batch) Bytes() int64 {
	if b.rows == nil {
		return 0
	}
	return Bytes(b.rows)
}
