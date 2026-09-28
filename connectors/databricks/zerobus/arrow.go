package zerobus

import (
	"bytes"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// conformSchema rewrites a Filament Arrow schema into the shape Zerobus validates
// against a Delta table: field metadata stripped, and Delta's wide string/binary
// Arrow types (LargeUtf8/LargeBinary) in place of Utf8/Binary. Field nullability
// is preserved and must match the target table's columns.
func conformSchema(src *arrow.Schema) *arrow.Schema {
	fields := make([]arrow.Field, len(src.Fields()))
	for i, f := range src.Fields() {
		fields[i] = arrow.Field{Name: f.Name, Type: conformType(f.Type), Nullable: f.Nullable}
	}
	return arrow.NewSchema(fields, nil)
}

// conformType maps Utf8 to LargeUtf8 and Binary to LargeBinary (the Arrow types
// Delta expects), leaving every other type unchanged.
func conformType(dt arrow.DataType) arrow.DataType {
	switch dt.ID() {
	case arrow.STRING:
		return arrow.BinaryTypes.LargeString
	case arrow.BINARY:
		return arrow.BinaryTypes.LargeBinary
	default:
		return dt
	}
}

// conformRecord returns rec reshaped to target: Utf8 columns widened to LargeUtf8
// and Binary to LargeBinary, other columns reused as-is. The caller owns the
// returned batch and must Release it; rec itself is left untouched.
func conformRecord(target *arrow.Schema, rec arrow.RecordBatch) arrow.RecordBatch {
	cols := make([]arrow.Array, rec.NumCols())
	var built []arrow.Array
	for i := 0; i < int(rec.NumCols()); i++ {
		src := rec.Column(i)
		switch a := src.(type) {
		case *array.String:
			w := toLargeString(a)
			cols[i] = w
			built = append(built, w)
		case *array.Binary:
			w := toLargeBinary(a)
			cols[i] = w
			built = append(built, w)
		default:
			cols[i] = src
		}
	}
	out := array.NewRecordBatch(target, cols, rec.NumRows())
	// NewRecordBatch retains each column, so drop our references to the widened ones.
	for _, a := range built {
		a.Release()
	}
	return out
}

// toLargeString copies a Utf8 array into a LargeUtf8 array, preserving nulls.
func toLargeString(s *array.String) arrow.Array {
	b := array.NewLargeStringBuilder(memory.DefaultAllocator)
	defer b.Release()
	b.Reserve(s.Len())
	for i := 0; i < s.Len(); i++ {
		if s.IsNull(i) {
			b.AppendNull()
			continue
		}
		b.Append(s.Value(i))
	}
	return b.NewArray()
}

// toLargeBinary copies a Binary array into a LargeBinary array, preserving nulls.
func toLargeBinary(s *array.Binary) arrow.Array {
	b := array.NewBinaryBuilder(memory.DefaultAllocator, arrow.BinaryTypes.LargeBinary)
	defer b.Release()
	b.Reserve(s.Len())
	for i := 0; i < s.Len(); i++ {
		if s.IsNull(i) {
			b.AppendNull()
			continue
		}
		b.Append(s.Value(i))
	}
	return b.NewArray()
}

// schemaIPC serializes a schema (no data batches) to Arrow IPC stream bytes.
// CreateArrowStream requires exactly the schema message.
func schemaIPC(schema *arrow.Schema) ([]byte, error) {
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema))
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// batchIPC serializes one RecordBatch (schema + one batch) to Arrow IPC stream
// bytes, the payload IngestBatch expects. Compression is applied by the SDK on
// the wire per the stream's configured codec, not here.
func batchIPC(schema *arrow.Schema, rec arrow.RecordBatch) ([]byte, error) {
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(schema))
	if err := w.Write(rec); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
