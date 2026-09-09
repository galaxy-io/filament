package gzipencoder

import (
	"bytes"
	"compress/gzip"
	"hash/crc32"
	"io"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
)

func TestEncoderStreamsBatches(t *testing.T) {
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
	}, nil)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer builder.Release()
	builder.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2}, nil)
	builder.Field(1).(*array.StringBuilder).AppendValues([]string{"Ada", ""}, []bool{true, false})
	record := builder.NewRecordBatch()
	defer record.Release()

	first := record.NewSlice(0, 1)
	defer first.Release()
	second := record.NewSlice(1, 2)
	defer second.Release()

	enc := NewEncoder(jsonencoder.NewEncoder(schema))
	var compressed []byte
	for _, batch := range []arrow.RecordBatch{first, second} {
		part, checksum, err := enc.EncodeBatch(nil, batch)
		if err != nil {
			t.Fatal(err)
		}
		if want := crc32.Checksum(part, crc32.MakeTable(crc32.Castagnoli)); checksum != want {
			t.Fatalf("batch CRC = %08x, want %08x", checksum, want)
		}
		compressed = append(compressed, part...)
	}
	tail, checksum, err := enc.Finalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := crc32.Checksum(tail, crc32.MakeTable(crc32.Castagnoli)); checksum != want {
		t.Fatalf("tail CRC = %08x, want %08x", checksum, want)
	}
	compressed = append(compressed, tail...)

	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	want := "{\"id\":1,\"name\":\"Ada\"}\n{\"id\":2,\"name\":null}\n"
	if string(decoded) != want {
		t.Fatalf("decoded gzip JSON = %q, want %q", decoded, want)
	}
	if _, _, err := enc.EncodeBatch(nil, first); err == nil {
		t.Fatal("EncodeBatch accepted a batch after Finalize")
	}
}
