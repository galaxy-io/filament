package parquet

import (
	"bytes"
	"context"
	"hash/crc32"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	arrowparquet "github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"

	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

func TestEncoderStreamsValidParquetFile(t *testing.T) {
	metadata := arrow.NewMetadata([]string{"filament.logical"}, []string{"string"})
	schema := arrow.NewSchema([]arrow.Field{
		{Name: "id", Type: arrow.PrimitiveTypes.Int64},
		{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true, Metadata: metadata},
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

	enc, err := NewEncoder(schema, encoder.CompressionSnappy)
	if err != nil {
		t.Fatal(err)
	}
	var file []byte
	for _, batch := range []arrow.RecordBatch{first, second} {
		part, checksum, err := enc.EncodeBatch(nil, batch)
		if err != nil {
			t.Fatal(err)
		}
		if want := crc32.Checksum(part, crc32.MakeTable(crc32.Castagnoli)); checksum != want {
			t.Fatalf("batch CRC = %08x, want %08x", checksum, want)
		}
		file = append(file, part...)
	}
	tail, checksum, err := enc.Finalize(nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := crc32.Checksum(tail, crc32.MakeTable(crc32.Castagnoli)); checksum != want {
		t.Fatalf("tail CRC = %08x, want %08x", checksum, want)
	}
	file = append(file, tail...)
	if !bytes.HasPrefix(file, []byte("PAR1")) || !bytes.HasSuffix(file, []byte("PAR1")) {
		t.Fatal("encoded data is missing Parquet magic bytes")
	}

	table, err := pqarrow.ReadTable(
		context.Background(), bytes.NewReader(file),
		arrowparquet.NewReaderProperties(memory.DefaultAllocator),
		pqarrow.ArrowReadProperties{}, memory.DefaultAllocator,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer table.Release()
	if table.NumRows() != 2 || table.NumCols() != 2 {
		t.Fatalf("Parquet table shape = %d rows x %d cols", table.NumRows(), table.NumCols())
	}
	if got := table.Column(0).Data().Chunk(0).(*array.Int64).Value(0); got != 1 {
		t.Fatalf("first id = %d, want 1", got)
	}
	if logical, ok := table.Schema().Field(1).Metadata.GetValue("filament.logical"); !ok || logical != "string" {
		t.Fatalf("logical metadata = %q, %v", logical, ok)
	}
}
