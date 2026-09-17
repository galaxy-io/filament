package pipeline

import (
	"errors"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

// streamWriter ensures resource switching happens before appending the next row,
// even when the source retains writers instead of reopening them at each switch.
type streamWriter struct {
	arrowbatch.RowWriter
	in       *streamInlet
	resource string
	inRow    bool
	closed   bool
}

func (w *streamWriter) beforeWrite() bool {
	if w.in.ready() != nil {
		return false
	}
	if w.closed {
		_ = w.in.fail(arrowbatch.ErrClosed)
		return false
	}
	if w.in.strict && w.in.active != nil && w.in.active != w {
		if err := w.in.active.Flush(); err != nil {
			return false
		}
	}
	w.in.active = w
	w.inRow = true
	return true
}

func (w *streamWriter) Null() {
	if w.beforeWrite() {
		w.RowWriter.Null()
	}
}

func (w *streamWriter) Bool(v bool) {
	if w.beforeWrite() {
		w.RowWriter.Bool(v)
	}
}

func (w *streamWriter) Int16(v int16) {
	if w.beforeWrite() {
		w.RowWriter.Int16(v)
	}
}

func (w *streamWriter) Int32(v int32) {
	if w.beforeWrite() {
		w.RowWriter.Int32(v)
	}
}

func (w *streamWriter) Int64(v int64) {
	if w.beforeWrite() {
		w.RowWriter.Int64(v)
	}
}

func (w *streamWriter) Float32(v float32) {
	if w.beforeWrite() {
		w.RowWriter.Float32(v)
	}
}

func (w *streamWriter) Float64(v float64) {
	if w.beforeWrite() {
		w.RowWriter.Float64(v)
	}
}

func (w *streamWriter) Decimal(v decimal128.Num) {
	if w.beforeWrite() {
		w.RowWriter.Decimal(v)
	}
}

func (w *streamWriter) String(v string) {
	if w.beforeWrite() {
		w.RowWriter.String(v)
	}
}

func (w *streamWriter) StringBytes(v []byte) {
	if w.beforeWrite() {
		w.RowWriter.StringBytes(v)
	}
}

func (w *streamWriter) Bytes(v []byte) {
	if w.beforeWrite() {
		w.RowWriter.Bytes(v)
	}
}

func (w *streamWriter) Date(v int32) {
	if w.beforeWrite() {
		w.RowWriter.Date(v)
	}
}

func (w *streamWriter) Time(v int64) {
	if w.beforeWrite() {
		w.RowWriter.Time(v)
	}
}

func (w *streamWriter) Timestamp(v int64) {
	if w.beforeWrite() {
		w.RowWriter.Timestamp(v)
	}
}

func (w *streamWriter) EndRow(meta rowmodel.Meta) error {
	if !w.beforeWrite() {
		return w.in.failure()
	}
	if err := w.RowWriter.EndRow(meta); err != nil {
		return w.in.fail(err)
	}
	w.inRow = false
	return nil
}

func (w *streamWriter) Flush() error {
	if err := w.in.p.Err(); err != nil {
		return err
	}
	if w.closed {
		return w.in.fail(arrowbatch.ErrClosed)
	}
	if w.inRow {
		return w.in.fail(errors.New("pipeline: stream flush inside row"))
	}
	if err := w.RowWriter.Flush(); err != nil {
		return w.in.fail(err)
	}
	return nil
}

// Drain flushes rows without producing a bounded shard-completion checkpoint.
func (w *streamWriter) Drain(_ rowmodel.Meta) error { return w.Flush() }

func (w *streamWriter) Close() error {
	if w.closed {
		return nil
	}
	err := w.Flush()
	w.closed = true
	if w.in.active == w {
		w.in.active = nil
	}
	closeErr := w.RowWriter.Close()
	if err != nil {
		return err
	}
	return closeErr
}
