package alchemy

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

// newReader caps every message at the stream's own size, so a hostile header cannot force a large allocation.
func newReader(raw []byte, mem memory.Allocator) (*ipc.Reader, error) {
	limit := max(int64(len(raw)), 1)
	return ipc.NewReader(bytes.NewReader(raw),
		ipc.WithAllocator(mem), ipc.WithMetadataSizeLimit(limit), ipc.WithBodySizeLimit(limit))
}

// decodeSchema reads a schema-only IPC stream.
func decodeSchema(raw []byte) (*arrow.Schema, error) {
	if len(raw) == 0 {
		return nil, errors.New("schema is empty")
	}
	r, err := newReader(raw, memory.DefaultAllocator)
	if err != nil {
		return nil, err
	}
	defer r.Release()
	return r.Schema(), nil
}

// EncodeSchema writes a schema-only IPC stream, the form an Arrow contract takes.
func EncodeSchema(s *arrow.Schema) ([]byte, error) {
	var buf bytes.Buffer
	if err := ipc.NewWriter(&buf, ipc.WithSchema(s)).Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// encodeBatch writes rec as an IPC stream of exactly one record batch.
func encodeBatch(rec arrow.RecordBatch) ([]byte, error) {
	var buf bytes.Buffer
	w := ipc.NewWriter(&buf, ipc.WithSchema(rec.Schema()))
	if err := w.Write(rec); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decodeBatch reads an IPC stream that must hold exactly one record batch.
func decodeBatch(raw []byte, mem memory.Allocator) (arrow.RecordBatch, error) {
	if len(raw) == 0 {
		return nil, errors.New("output is empty")
	}
	r, err := newReader(raw, mem)
	if err != nil {
		return nil, err
	}
	defer r.Release()
	if !r.Next() {
		if err := r.Err(); err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		return nil, errors.New("output holds no record batch")
	}
	rec := r.RecordBatch()
	rec.Retain()
	if r.Next() {
		rec.Release()
		return nil, errors.New("output holds more than one record batch")
	}
	if err := r.Err(); err != nil && !errors.Is(err, io.EOF) {
		rec.Release()
		return nil, fmt.Errorf("output: %w", err)
	}
	return rec, nil
}

// concatBatches joins same-schema batches into one new batch.
func concatBatches(mem memory.Allocator, recs []arrow.RecordBatch) (arrow.RecordBatch, error) {
	schema := recs[0].Schema()
	cols := make([]arrow.Array, schema.NumFields())
	defer func() {
		for _, col := range cols {
			if col != nil {
				col.Release()
			}
		}
	}()
	var rows int64
	for _, rec := range recs {
		if !rec.Schema().Equal(schema) {
			return nil, errors.New("split invocations returned different schemas")
		}
		rows += rec.NumRows()
	}
	for i := range cols {
		parts := make([]arrow.Array, len(recs))
		for j, rec := range recs {
			parts[j] = rec.Column(i)
		}
		col, err := array.Concatenate(parts, mem)
		if err != nil {
			return nil, err
		}
		cols[i] = col
	}
	return array.NewRecordBatch(schema, cols, rows), nil
}
