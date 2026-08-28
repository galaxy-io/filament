package arrowbatch

import "github.com/apache/arrow-go/v18/arrow"

// Bytes returns the memory a record batch's column buffers occupy.
func Bytes(rows arrow.RecordBatch) int64 {
	var size int64
	for _, column := range rows.Columns() {
		for _, buffer := range column.Data().Buffers() {
			if buffer != nil {
				size += int64(buffer.Len())
			}
		}
	}
	return size
}

// Bytes returns the memory occupied by b's Arrow buffers.
func (b *Batch) Bytes() int64 {
	if b.rows == nil {
		return 0
	}
	return Bytes(b.rows)
}
