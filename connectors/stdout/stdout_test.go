package stdout

import (
	"bytes"
	"context"
	"hash/crc32"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestWriteReceiptSeparatesArrowAndEncodedIntegrity(t *testing.T) {
	schema := arrowbatch.Schema(rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}})
	builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	builder.Field(0).(*array.Int64Builder).Append(7)
	rows := builder.NewRecordBatch()
	builder.Release()
	b := arrowbatch.NewBatch(rows, arrowbatch.Operations{})
	b.Resource = "users"
	defer b.Release()

	var output bytes.Buffer
	sink := New(WithWriter(&output))
	if !sink.Spec().Capabilities.EncodedIntegrity {
		t.Fatal("stdout does not advertise encoded integrity")
	}
	receipt, err := sink.Write(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.WriteCRC != b.IntegrityCRC() {
		t.Fatalf("Arrow CRC = %08x, want %08x", receipt.WriteCRC, b.IntegrityCRC())
	}
	if receipt.EncodedCRC == nil {
		t.Fatal("encoded CRC is nil")
	}
	want := crc32.Checksum(output.Bytes(), crc32.MakeTable(crc32.Castagnoli))
	if *receipt.EncodedCRC != want {
		t.Fatalf("encoded CRC = %08x, want %08x", *receipt.EncodedCRC, want)
	}
}
