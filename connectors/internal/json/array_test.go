package jsonencoder

import (
	"hash/crc32"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
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

	enc := NewArrayEncoder(schema)
	var encoded []byte
	for _, bounds := range [][2]int64{{0, 1}, {1, 2}} {
		batch := record.NewSlice(bounds[0], bounds[1])
		part, checksum, err := enc.EncodeBatch(nil, batch)
		batch.Release()
		if err != nil {
			t.Fatal(err)
		}
		if want := crc32.Checksum(part, crcTable); checksum != want {
			t.Fatalf("batch CRC = %08x, want %08x", checksum, want)
		}
		encoded = append(encoded, part...)
	}
	tail, _, err := enc.Finalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, tail...)
	if want := `[{"id":1,"name":"Ada"},{"id":2,"name":null}]`; string(encoded) != want {
		t.Fatalf("JSON = %q, want %q", encoded, want)
	}
}

func TestEncoderEmptyArray(t *testing.T) {
	schema := arrow.NewSchema(nil, nil)
	encoded, _, err := NewArrayEncoder(schema).Finalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != "[]" {
		t.Fatalf("empty JSON = %q, want []", encoded)
	}
}
