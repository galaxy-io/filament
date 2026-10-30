//go:build integration

package s3_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	arrowparquet "github.com/apache/arrow-go/v18/parquet"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/apache/arrow-go/v18/parquet/pqarrow"
	"github.com/minio/minio-go/v7"

	"github.com/galaxy-io/filament"
	s3sink "github.com/galaxy-io/filament/connectors/object/s3"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// TestS3SinkParquetSmallBatches verifies buffered output and its committed manifest.
func TestS3SinkParquetSmallBatches(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	store := testcontainers.MinIOContainer(t)
	const bucket = "filament-parquet-batching"
	if err := store.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{
		"bucket": bucket, "prefix": "exports", "region": "us-east-1",
		"endpoint": "http://" + store.Endpoint, "path_style": true,
		"auth_method": "iam_credentials", "access_key_id": store.AccessKey, "secret_access_key": store.SecretKey,
		"file_format": "parquet", "compression": "snappy",
	}
	schema := filament.RecordSchema{Fields: []filament.SchemaField{{Name: "id", Logical: filament.LogicalInt64}}}
	batches := &testutil.CollectSink{}
	defer batches.Release()
	writer, err := batches.Builder("accounts", 0, schema)
	if err != nil {
		t.Fatal(err)
	}
	for id := int64(0); id < 100; id++ {
		writer.Int64(id)
		if err := writer.EndRow(filament.RowMeta{Op: filament.OpInsert}); err != nil {
			t.Fatal(err)
		}
		if err := writer.Drain(filament.RowMeta{}); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(batches.BatchesFor("accounts")); got != 100 {
		t.Fatalf("built %d batches, want 100", got)
	}
	policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
	policy.Resource = "accounts"
	dst := s3sink.New()
	if err := dst.Open(ctx, filament.RunSpec{
		Run: "parquet-batches", StartedAt: time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC),
		Resources: []string{"accounts"}, Sink: filament.Ref{Config: cfg},
		WritePolicies: map[string]filament.WritePolicy{"accounts": policy},
	}); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = dst.Abort(ctx) }()
	var rows, emitted int64
	var emptyWrites int
	for _, batch := range batches.BatchesFor("accounts") {
		receipt, err := dst.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
		if err != nil {
			t.Fatal(err)
		}
		if receipt.Rows != 1 || receipt.EncodedCRC == nil {
			t.Fatalf("incomplete write receipt: %#v", receipt)
		}
		if receipt.Bytes == 0 {
			emptyWrites++
			if *receipt.EncodedCRC != 0 {
				t.Fatalf("empty emission has nonzero CRC: %08x", *receipt.EncodedCRC)
			}
		}
		rows += int64(receipt.Rows)
		emitted += receipt.Bytes
	}
	if emptyWrites == 0 {
		t.Fatal("small batches were not buffered")
	}
	if err := dst.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	body := readMinIOObject(t, ctx, store, bucket, "exports/accounts/dt=2026-09-10/parquet-batches.parquet")
	reader, err := file.NewParquetReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if reader.NumRows() != rows || reader.NumRowGroups() != 1 {
		t.Fatalf("Parquet has %d rows in %d groups, want %d rows in one group", reader.NumRows(), reader.NumRowGroups(), rows)
	}
	table, err := pqarrow.ReadTable(ctx, bytes.NewReader(body), arrowparquet.NewReaderProperties(memory.DefaultAllocator),
		pqarrow.ArrowReadProperties{}, memory.DefaultAllocator)
	if err != nil {
		t.Fatal(err)
	}
	defer table.Release()
	var seen int64
	for _, chunk := range table.Column(0).Data().Chunks() {
		ids := chunk.(*array.Int64)
		for i := 0; i < ids.Len(); i++ {
			if ids.Value(i) != seen {
				t.Fatalf("row %d has id %d", seen, ids.Value(i))
			}
			seen++
		}
	}
	if seen != rows || int64(len(body)) <= emitted {
		t.Fatalf("finalized %d rows and %d bytes, receipts had %d rows and %d bytes", seen, len(body), rows, emitted)
	}
	manifestBody := readMinIOObject(t, ctx, store, bucket, "exports/_runs/parquet-batches/_SUCCESS.json")
	var manifest struct {
		Resources []struct {
			Rows   int64  `json:"rows"`
			Bytes  int64  `json:"bytes"`
			CRC32C string `json:"crc32c"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatal(err)
	}
	if len(manifest.Resources) != 1 {
		t.Fatalf("success manifest = %s", manifestBody)
	}
	resource := manifest.Resources[0]
	wantCRC := fmt.Sprintf("%08x", crc32.Checksum(body, crc32.MakeTable(crc32.Castagnoli)))
	if resource.Rows != rows || resource.Bytes != int64(len(body)) || resource.CRC32C != wantCRC {
		t.Fatalf("manifest resource = %#v, want rows=%d bytes=%d CRC=%s", resource, rows, len(body), wantCRC)
	}
}
