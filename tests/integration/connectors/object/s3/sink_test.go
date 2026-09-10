//go:build integration

package s3_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/galaxy-io/filament"
	s3sink "github.com/galaxy-io/filament/connectors/object/s3"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestS3SinkCommitAndAbort(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	store := testcontainers.MinIOContainer(t)
	const bucket = "filament-integration"
	if err := store.Client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: "us-east-1"}); err != nil {
		t.Fatal(err)
	}
	cfg := map[string]any{
		"bucket": bucket, "prefix": "exports", "region": "us-east-1",
		"endpoint": "http://" + store.Endpoint, "path_style": true,
		"auth_method": "iam_credentials", "access_key_id": store.AccessKey, "secret_access_key": store.SecretKey,
		"part_size_mib": 5, "upload_concurrency": 2,
	}
	startedAt := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)

	t.Run("commit publishes exact NDJSON then success marker", func(t *testing.T) {
		batches := objectTestRows(t, false)
		defer batches.Release()
		policy := filament.WritePolicyForIngestion(filament.IngestionFullReplace)
		policy.Resource = "accounts"
		dst := s3sink.New()
		if err := dst.TestConnection(ctx, filament.NewConfig(cfg)); err != nil {
			t.Fatalf("S3 connection: %v", err)
		}
		if err := dst.Open(ctx, filament.RunSpec{
			Run: "s3-commit", StartedAt: startedAt, Resources: []string{"accounts"}, Sink: filament.Ref{Config: cfg},
			WritePolicies: map[string]filament.WritePolicy{"accounts": policy},
		}); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = dst.Abort(ctx) }()
		var rows, bytes int64
		for _, batch := range batches.BatchesFor("accounts") {
			receipt, err := dst.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
			if err != nil {
				t.Fatal(err)
			}
			if receipt.EncodedCRC == nil || receipt.Rows != batch.NumRows() || receipt.Bytes <= 0 {
				t.Fatalf("incomplete write receipt: %#v", receipt)
			}
			rows += int64(receipt.Rows)
			bytes += receipt.Bytes
		}
		if _, err := store.Client.StatObject(ctx, bucket, "exports/_runs/s3-commit/_SUCCESS.json", minio.StatObjectOptions{}); err == nil {
			t.Fatal("success marker became visible before Commit")
		}
		if err := dst.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		body := readMinIOObject(t, ctx, store, bucket, "exports/accounts/dt=2026-09-10/s3-commit.ndjson")
		var got []string
		scanner := bufio.NewScanner(strings.NewReader(string(body)))
		for scanner.Scan() {
			decoder := json.NewDecoder(strings.NewReader(scanner.Text()))
			decoder.UseNumber()
			var row map[string]any
			if err := decoder.Decode(&row); err != nil {
				t.Fatalf("decode NDJSON row: %v", err)
			}
			got = append(got, fmt.Sprintf("%s|%s|%v|%v", row["id"], row["name"], row["active"], row["note"]))
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		want := []string{"1|Ada|true|<nil>", "2|Grace|false|present"}
		if !slices.Equal(got, want) {
			t.Fatalf("NDJSON rows = %v, want %v; body=%s", got, want, body)
		}

		manifestBody := readMinIOObject(t, ctx, store, bucket, "exports/_runs/s3-commit/_SUCCESS.json")
		var manifest struct {
			Version   int    `json:"version"`
			Run       string `json:"run"`
			Resources []struct {
				Name   string `json:"name"`
				Key    string `json:"key"`
				Rows   int64  `json:"rows"`
				Bytes  int64  `json:"bytes"`
				CRC32C string `json:"crc32c"`
			} `json:"resources"`
		}
		if err := json.Unmarshal(manifestBody, &manifest); err != nil {
			t.Fatal(err)
		}
		if manifest.Version != 1 || manifest.Run != "s3-commit" || len(manifest.Resources) != 1 {
			t.Fatalf("success manifest = %s", manifestBody)
		}
		resource := manifest.Resources[0]
		if resource.Name != "accounts" || resource.Key != "exports/accounts/dt=2026-09-10/s3-commit.ndjson" || resource.Rows != rows || resource.Bytes != bytes || len(resource.CRC32C) != 8 {
			t.Fatalf("success manifest resource = %#v, receipts rows=%d bytes=%d", resource, rows, bytes)
		}
	})

	t.Run("abort removes a started multipart upload", func(t *testing.T) {
		batches := objectTestRows(t, true)
		defer batches.Release()
		policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
		policy.Resource = "large"
		dst := s3sink.New()
		if err := dst.Open(ctx, filament.RunSpec{
			Run: "s3-abort", StartedAt: startedAt, Resources: []string{"large"}, Sink: filament.Ref{Config: cfg},
			WritePolicies: map[string]filament.WritePolicy{"large": policy},
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Apply(ctx, batches.BatchesFor("large")[0], filament.ApplyOptions{Policy: policy}); err != nil {
			t.Fatal(err)
		}
		if err := dst.Abort(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Client.StatObject(ctx, bucket, "exports/large/dt=2026-09-10/s3-abort.ndjson", minio.StatObjectOptions{}); err == nil {
			t.Fatal("aborted object is visible")
		}
		for upload := range store.Client.ListIncompleteUploads(ctx, bucket, "exports", true) {
			if upload.Err != nil {
				t.Fatal(upload.Err)
			}
			t.Fatalf("multipart upload leaked after Abort: %#v", upload)
		}
		if _, err := store.Client.StatObject(ctx, bucket, "exports/_runs/s3-abort/_SUCCESS.json", minio.StatObjectOptions{}); err == nil {
			t.Fatal("aborted run published a success marker")
		}
	})
}

func objectTestRows(t testing.TB, large bool) *testutil.CollectSink {
	t.Helper()
	resource := "accounts"
	schema := filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "name", Logical: filament.LogicalString},
			{Name: "active", Logical: filament.LogicalBool},
			{Name: "note", Logical: filament.LogicalString, Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
	if large {
		resource = "large"
	}
	out := &testutil.CollectSink{}
	writer, err := out.Builder(resource, 0, schema)
	if err != nil {
		t.Fatal(err)
	}
	appendRow := func(id int64, name string, active bool, note *string) {
		writer.Int64(id)
		writer.String(name)
		writer.Bool(active)
		if note == nil {
			writer.Null()
		} else {
			writer.String(*note)
		}
		if err := writer.EndRow(filament.RowMeta{Op: filament.OpInsert}); err != nil {
			t.Fatal(err)
		}
	}
	if large {
		appendRow(1, strings.Repeat("x", 6<<20), true, nil)
	} else {
		note := "present"
		appendRow(1, "Ada", true, nil)
		appendRow(2, "Grace", false, &note)
	}
	if err := writer.Drain(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	return out
}

func readMinIOObject(t testing.TB, ctx context.Context, store *testcontainers.MinIO, bucket, key string) []byte {
	t.Helper()
	object, err := store.Client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer object.Close()
	body, err := io.ReadAll(object)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
