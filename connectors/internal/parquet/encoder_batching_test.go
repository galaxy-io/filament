package parquet

import (
	"bytes"
	"context"
	"fmt"
	"hash/crc32"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	arrowparquet "github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"

	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

func TestEncoderBatchesRowGroups(t *testing.T) {
	const limit = 64 * 1024
	small := make([]int, 100)
	for i := range small {
		small[i] = 1
	}
	for _, tt := range []struct {
		name    string
		batches []int
		groups  []int64
	}{
		{name: "small transactions", batches: small, groups: []int64{100}},
		{name: "empty batches", batches: []int{0, 7, 0, 13, 0}, groups: []int64{20}},
		{name: "crossing the limit", batches: []int{limit - 1, 2, 1}, groups: []int64{limit, 2}},
		{name: "large batch", batches: []int{2*limit + 7}, groups: []int64{limit, limit, 7}},
		{name: "exact groups", batches: []int{limit, limit}, groups: []int64{limit, limit}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
			defer alloc.AssertSize(t, 0)
			schema := arrow.NewSchema([]arrow.Field{
				{Name: "id", Type: arrow.PrimitiveTypes.Int64},
				{Name: "name", Type: arrow.BinaryTypes.String, Nullable: true},
			}, nil)
			builder := array.NewRecordBuilder(alloc, schema)
			defer builder.Release()
			enc, err := NewEncoder(schema, encoder.CompressionSnappy)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if !enc.closed {
					_, _, _ = enc.Finalize(nil)
				}
			}()
			var encoded []byte
			appendPart := func(part []byte, checksum uint32, err error) {
				t.Helper()
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.HasPrefix(part, []byte("prefix")) {
					t.Fatal("encoder did not preserve the destination prefix")
				}
				payload := part[len("prefix"):]
				if want := crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)); checksum != want {
					t.Fatalf("emitted CRC = %08x, want %08x", checksum, want)
				}
				encoded = append(encoded, payload...)
			}
			var total int64
			for _, count := range tt.batches {
				for range count {
					builder.Field(0).(*array.Int64Builder).Append(total)
					if total%3 == 0 {
						builder.Field(1).(*array.StringBuilder).AppendNull()
					} else {
						builder.Field(1).(*array.StringBuilder).Append(fmt.Sprintf("row-%d", total))
					}
					total++
				}
				batch := builder.NewRecordBatch()
				part, checksum, err := enc.EncodeBatch([]byte("prefix"), batch)
				batch.Release()
				appendPart(part, checksum, err)
			}
			appendPart(enc.Finalize([]byte("prefix")))

			reader, err := file.NewParquetReader(bytes.NewReader(encoded))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			if reader.NumRows() != total || reader.NumRowGroups() != len(tt.groups) {
				t.Fatalf("Parquet has %d rows in %d groups, want %d rows in %d groups", reader.NumRows(), reader.NumRowGroups(), total, len(tt.groups))
			}
			for i, want := range tt.groups {
				if got := reader.RowGroup(i).NumRows(); got != want {
					t.Fatalf("row group %d has %d rows, want %d", i, got, want)
				}
			}
			table, err := pqarrow.ReadTable(context.Background(), bytes.NewReader(encoded),
				arrowparquet.NewReaderProperties(alloc), pqarrow.ArrowReadProperties{}, alloc)
			if err != nil {
				t.Fatal(err)
			}
			defer table.Release()
			rows := array.NewTableReader(table, 1024)
			defer rows.Release()
			var seen int64
			for rows.Next() {
				batch := rows.RecordBatch()
				ids := batch.Column(0).(*array.Int64)
				names := batch.Column(1).(*array.String)
				for i := 0; i < int(batch.NumRows()); i++ {
					if ids.Value(i) != seen || names.IsNull(i) != (seen%3 == 0) {
						t.Fatalf("row %d did not round-trip", seen)
					}
					if seen%3 != 0 && names.Value(i) != fmt.Sprintf("row-%d", seen) {
						t.Fatalf("name at row %d did not round-trip", seen)
					}
					seen++
				}
			}
			if rows.Err() != nil || seen != total {
				t.Fatalf("read %d of %d rows: %v", seen, total, rows.Err())
			}
		})
	}
}
