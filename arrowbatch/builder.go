package arrowbatch

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/rowmodel"
)

// Options bounds the chunks a Builder emits.
type Options struct {
	MaxRows  int   // rows per chunk
	MaxBytes int64 // approximate value bytes per chunk; 0 = unbounded
}

// Builder is the filament.RowWriter for one (resource, part): typed column
// builders in schema order that become an Arrow record batch when a chunk fills.
// It belongs to the one goroutine that appends to it; the only thing another
// goroutine may do is RequestFlush, which EndRow honors once the row is closed.
// Errors are sticky: after the first, appends are no-ops and every method
// returns it.
type Builder struct {
	schema *arrow.Schema
	alloc  memory.Allocator
	opts   Options
	recv   Receiver

	cols     []array.Builder
	cur      int   // columns appended to the open row
	rows     int   // rows in the open chunk
	bytes    int64 // value bytes in the open chunk
	ops      []rowmodel.Operation
	last     rowmodel.Meta
	total    int // rows ever completed, for the coarse completion marker
	err      error
	flushReq atomic.Bool
	closed   atomic.Bool
}

// NewBuilder returns a builder for schema that hands owned batches to recv.
func NewBuilder(schema *arrow.Schema, alloc memory.Allocator, opts Options, recv Receiver) *Builder {
	if opts.MaxRows <= 0 {
		panic("arrowbatch: MaxRows must be positive")
	}
	if alloc == nil {
		alloc = memory.DefaultAllocator
	}
	cols := make([]array.Builder, schema.NumFields())
	for i, f := range schema.Fields() {
		cols[i] = array.NewBuilder(alloc, f.Type)
	}
	b := &Builder{schema: schema, alloc: alloc, opts: opts, recv: recv, cols: cols}
	b.reserve()
	return b
}

// Schema returns the Arrow schema rows are built in.
func (b *Builder) Schema() *arrow.Schema { return b.schema }

func (b *Builder) reserve() {
	for _, c := range b.cols {
		c.Reserve(b.opts.MaxRows)
	}
}

// open returns the column builder the next append targets, or nil once the
// builder has failed or the row is already full.
func (b *Builder) open() array.Builder {
	if b.closed.Load() {
		b.err = ErrClosed
		return nil
	}
	if b.err != nil {
		return nil
	}
	if b.cur >= len(b.cols) {
		b.err = fmt.Errorf("batch: row has %d columns, append %d", len(b.cols), b.cur+1)
		return nil
	}
	return b.cols[b.cur]
}

func (b *Builder) mismatch(want string) {
	f := b.schema.Field(b.cur)
	b.err = fmt.Errorf("batch: %s append on column %q (%s)", want, f.Name, f.Type)
}

// Null appends a null to the current column.
func (b *Builder) Null() {
	if c := b.open(); c != nil {
		c.AppendNull()
		b.cur++
	}
}

// Bool appends a boolean.
func (b *Builder) Bool(v bool) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.BooleanBuilder); ok {
		t.Append(v)
		b.cur++
		b.bytes++
		return
	}
	b.mismatch("bool")
}

// Int16 appends a 16-bit integer.
func (b *Builder) Int16(v int16) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Int16Builder); ok {
		t.Append(v)
		b.cur++
		b.bytes += 2
		return
	}
	b.mismatch("int16")
}

// Int32 appends a 32-bit integer.
func (b *Builder) Int32(v int32) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Int32Builder); ok {
		t.Append(v)
		b.cur++
		b.bytes += 4
		return
	}
	b.mismatch("int32")
}

// Int64 appends a 64-bit integer.
func (b *Builder) Int64(v int64) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Int64Builder); ok {
		t.Append(v)
		b.cur++
		b.bytes += 8
		return
	}
	b.mismatch("int64")
}

// Float32 appends a single-precision float.
func (b *Builder) Float32(v float32) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Float32Builder); ok {
		t.Append(v)
		b.cur++
		b.bytes += 4
		return
	}
	b.mismatch("float32")
}

// Float64 appends a double-precision float.
func (b *Builder) Float64(v float64) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Float64Builder); ok {
		t.Append(v)
		b.cur++
		b.bytes += 8
		return
	}
	b.mismatch("float64")
}

// Decimal appends a decimal128 value at the column's scale.
func (b *Builder) Decimal(v decimal128.Num) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Decimal128Builder); ok {
		t.Append(v)
		b.cur++
		b.bytes += 16
		return
	}
	b.mismatch("decimal")
}

// String appends a utf8 value.
func (b *Builder) String(v string) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.StringBuilder); ok {
		t.Append(v)
		b.cur++
		b.bytes += int64(len(v)) + 4
		return
	}
	b.mismatch("string")
}

// StringBytes appends a utf8 value held as bytes.
func (b *Builder) StringBytes(v []byte) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.StringBuilder); ok {
		t.BinaryBuilder.Append(v)
		b.cur++
		b.bytes += int64(len(v)) + 4
		return
	}
	b.mismatch("string")
}

// Bytes appends a binary value.
func (b *Builder) Bytes(v []byte) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.BinaryBuilder); ok {
		t.Append(v)
		b.cur++
		b.bytes += int64(len(v)) + 4
		return
	}
	b.mismatch("bytes")
}

// Date appends days since 1970-01-01.
func (b *Builder) Date(days int32) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Date32Builder); ok {
		t.Append(arrow.Date32(days))
		b.cur++
		b.bytes += 4
		return
	}
	b.mismatch("date")
}

// Time appends microseconds since midnight.
func (b *Builder) Time(micros int64) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.Time64Builder); ok {
		t.Append(arrow.Time64(micros))
		b.cur++
		b.bytes += 8
		return
	}
	b.mismatch("time")
}

// Timestamp appends microseconds since the Unix epoch.
func (b *Builder) Timestamp(micros int64) {
	c := b.open()
	if c == nil {
		return
	}
	if t, ok := c.(*array.TimestampBuilder); ok {
		t.Append(arrow.Timestamp(micros))
		b.cur++
		b.bytes += 8
		return
	}
	b.mismatch("timestamp")
}

// EndRow closes the row with its meta and flushes the chunk once it reaches
// MaxRows or MaxBytes, or a flush was requested. It returns the builder's sticky
// error, including a row short of columns.
func (b *Builder) EndRow(meta rowmodel.Meta) error {
	if b.closed.Load() {
		return ErrClosed
	}
	if b.err == nil && b.cur != len(b.cols) {
		b.err = fmt.Errorf("batch: row has %d columns, got %d", len(b.cols), b.cur)
	}
	b.cur = 0
	if b.err != nil {
		return b.err
	}
	if meta.Op != rowmodel.OpInsert && b.ops == nil {
		b.ops = make([]rowmodel.Operation, b.rows, b.opts.MaxRows)
	}
	if b.ops != nil {
		b.ops = append(b.ops, meta.Op)
	}
	// EndRow borrows stream metadata; retain an independent copy. Nil leaves
	// the ordinary bounded path allocation-free.
	meta.Stream = meta.Stream.Clone()
	b.last = meta
	b.rows++
	b.total++
	if b.rows >= b.opts.MaxRows || (b.opts.MaxBytes > 0 && b.bytes >= b.opts.MaxBytes) || b.flushReq.Swap(false) {
		return b.flush()
	}
	return nil
}

// RequestFlush asks the appending goroutine to flush at the next EndRow. Safe
// from any goroutine; the pipeline's flush timer uses it.
func (b *Builder) RequestFlush() { b.flushReq.Store(true) }

// Closed reports whether Close has released the builder's Arrow resources.
func (b *Builder) Closed() bool { return b.closed.Load() }

// Flush hands the open chunk to the receiver, if it has any rows. A source with an
// idle stream calls it so buffered rows do not wait for the next one.
func (b *Builder) Flush() error {
	if b.closed.Load() {
		return ErrClosed
	}
	if b.err != nil {
		return b.err
	}
	return b.flush()
}

// Drain flushes the open chunk and marks the part complete: meta carries a
// change stream's final position, and the receiver gets the part's total rows.
func (b *Builder) Drain(meta rowmodel.Meta) error {
	if b.closed.Load() {
		return ErrClosed
	}
	if b.err != nil {
		return b.err
	}
	if err := b.flush(); err != nil {
		return err
	}
	if err := b.recv.Drained(meta, b.total); err != nil {
		b.err = err
		return err
	}
	return nil
}

var errMidRow = errors.New("batch: flush inside a row")

// ErrClosed reports use of a closed builder.
var ErrClosed = errors.New("arrowbatch: builder closed")

// Close releases the column builders. It is idempotent and discards any
// unflushed or partial row; callers explicitly Flush or Drain before closing
// when those rows are valid.
func (b *Builder) Close() error {
	if b.closed.Swap(true) {
		return nil
	}
	for _, col := range b.cols {
		col.Release()
	}
	b.cols = nil
	b.ops = nil
	b.rows = 0
	b.cur = 0
	return nil
}

// flush builds and hands off the open chunk. No row may be open.
func (b *Builder) flush() error {
	if b.cur != 0 {
		b.err = errMidRow
		return b.err
	}
	if b.rows == 0 {
		return nil
	}
	arrs := make([]arrow.Array, len(b.cols))
	for i, c := range b.cols {
		arrs[i] = c.NewArray()
	}
	rec := array.NewRecordBatch(b.schema, arrs, int64(b.rows))
	for _, a := range arrs {
		a.Release()
	}
	out := NewBatch(rec, takeOperations(b.ops))
	out.Last = b.last
	b.ops = nil
	b.rows = 0
	b.bytes = 0
	b.reserve()
	if err := b.recv.Chunk(out); err != nil {
		out.Release()
		b.err = err
		return err
	}
	return nil
}
