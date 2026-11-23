package s3

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/apache/arrow-go/v18/parquet/file"
	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// TestRolloverAcrossEpochs replays a continuous run the way the runner drives
// a single-write sink: every epoch is a fresh sink with its own write ID and
// start time, and each epoch writes past max_file_size_mib several times. The
// middle epoch fails after rolling and is aborted. Nothing may be visible
// before Commit, every committed object must be a complete file on its own,
// no key may repeat or be overwritten across epochs, every multipart upload
// must complete or abort inside the S3 part rules, and each epoch's manifest
// must account for exactly its own files and rows.
func TestRolloverAcrossEpochs(t *testing.T) {
	for _, test := range []struct {
		format, compression string
		writes              int // enough input for several 5 MiB objects per epoch
	}{
		{"ndjson", "none", 24},
		{"ndjson", "gzip", 40},
		{"json", "gzip", 40},
		{"parquet", "snappy", 96},
	} {
		t.Run(test.format+"-"+test.compression, func(t *testing.T) {
			ctx := context.Background()
			store := newLimitStore()
			store.keep = true
			// Distinct batches, so Parquet dictionaries and gzip do not shrink
			// the repeated input under the roll size.
			batches := make([]*arrowbatch.Batch, 16)
			for i := range batches {
				batches[i] = testBatch(t, i)
				defer batches[i].Release()
			}

			const epochs, failed = 3, 2
			seen := map[string]int{} // key -> epoch that wrote it
			committed := 0
			for epoch := 1; epoch <= epochs; epoch++ {
				run := rollRun()
				run.SinkWriteID = filament.RunID(uuid.NewString())
				run.SinkWriteStartedAt = run.StartedAt.Add(time.Duration(epoch) * 30 * time.Second)
				sink, policy := openTestSink(t, ctx, store, test.format, test.compression, run)
				if err := sink.Begin(ctx); err != nil {
					t.Fatal(err)
				}
				for n := range test.writes {
					if _, err := sink.Apply(ctx, batches[n%len(batches)], filament.ApplyOptions{Policy: policy}); err != nil {
						t.Fatalf("epoch %d: %v", epoch, err)
					}
				}
				if store.pending() < 2 {
					t.Fatalf("epoch %d rolled into %d open uploads, want at least 2", epoch, store.pending())
				}
				if epoch == failed {
					if err := sink.Abort(ctx); err != nil {
						t.Fatal(err)
					}
					if store.pending() != 0 {
						t.Fatalf("abort left %d multipart uploads open", store.pending())
					}
					continue
				}
				if got := len(store.keys()); got != len(seen)+committed {
					t.Fatalf("epoch %d made %d objects visible before Commit", epoch, got-len(seen)-committed)
				}
				if err := sink.Commit(ctx); err != nil {
					t.Fatalf("epoch %d commit: %v", epoch, err)
				}
				committed++
				if store.pending() != 0 {
					t.Fatalf("epoch %d left %d multipart uploads open", epoch, store.pending())
				}

				manifest := readRollManifest(t, store, sink.layout.Success())
				if manifest.Version != 2 || len(manifest.Resources) != 1 || len(manifest.Resources[0].Files) < 2 {
					t.Fatalf("epoch %d manifest = %+v, want one resource rolled into several files", epoch, manifest)
				}
				resource := manifest.Resources[0]
				var rows, size int64
				for i, entry := range resource.Files {
					if want, err := sink.layout.Object("events", i); err != nil || entry.Key != want {
						t.Fatalf("epoch %d file %d key = %s, want %s (%v)", epoch, i, entry.Key, want, err)
					}
					if !strings.Contains(entry.Key, string(run.SinkWriteID)) {
						t.Fatalf("epoch %d file %s does not carry write ID %s", epoch, entry.Key, run.SinkWriteID)
					}
					if previous, dup := seen[entry.Key]; dup {
						t.Fatalf("epoch %d reused key %s from epoch %d", epoch, entry.Key, previous)
					}
					seen[entry.Key] = epoch
					stored, ok := store.object(entry.Key)
					if !ok {
						t.Fatalf("epoch %d manifest lists missing object %s", epoch, entry.Key)
					}
					if entry.Bytes != stored.size {
						t.Fatalf("%s manifest bytes=%d, stored %d", entry.Key, entry.Bytes, stored.size)
					}
					for n, part := range stored.partSizes {
						if part > maxPartSize || (n < len(stored.partSizes)-1 && part < minPartSize) {
							t.Fatalf("%s part %d is %d bytes, outside S3 bounds", entry.Key, n+1, part)
						}
					}
					if i < len(resource.Files)-1 && stored.size < sink.maxFileSize {
						t.Fatalf("%s rolled at %d bytes, under the %d target", entry.Key, stored.size, sink.maxFileSize)
					}
					if decoded := decodeRows(t, test.format, test.compression, stored.body); decoded != entry.Rows {
						t.Fatalf("%s decodes %d rows, manifest says %d", entry.Key, decoded, entry.Rows)
					}
					rows += entry.Rows
					size += entry.Bytes
				}
				if want := int64(test.writes * testBatchRows); rows != want || resource.Rows != rows || resource.Bytes != size {
					t.Fatalf("epoch %d resource rows=%d bytes=%d, files rows=%d bytes=%d, want %d rows", epoch, resource.Rows, resource.Bytes, rows, size, want)
				}
			}
			if keys := store.overwritten(); len(keys) != 0 {
				t.Fatalf("objects overwritten across epochs: %v", keys)
			}
			if got, want := len(store.keys()), len(seen)+committed; got != want {
				t.Fatalf("%d objects stored, want %d files and %d manifests", got, len(seen), committed)
			}
		})
	}
}

var testSchema = filament.RecordSchema{Fields: []filament.SchemaField{
	{Name: "id", Logical: filament.LogicalInt64},
	{Name: "account", Logical: filament.LogicalString},
	{Name: "active", Logical: filament.LogicalBool},
	{Name: "amount", Logical: filament.LogicalFloat64},
	{Name: "note", Logical: filament.LogicalString, Nullable: true},
}}

const testBatchRows = 8_192

// testBatch builds events rows whose values vary with seed, so neither
// Parquet dictionaries nor gzip collapse repeated batches.
func testBatch(t *testing.T, seed int) *arrowbatch.Batch {
	t.Helper()
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(testSchema))
	defer builder.Release()
	words := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel", "india", "juliet"}
	random := rand.New(rand.NewPCG(uint64(seed), 0x5eed))
	var note strings.Builder
	for row := range testBatchRows {
		builder.Field(0).(*array.Int64Builder).Append(int64(row + seed*testBatchRows))
		builder.Field(1).(*array.StringBuilder).Append("acct-" + strconv.Itoa(random.IntN(50_000)))
		builder.Field(2).(*array.BooleanBuilder).Append(random.IntN(3) != 0)
		builder.Field(3).(*array.Float64Builder).Append(float64(random.IntN(100_000_000)) / 100)
		if random.IntN(11) == 0 {
			builder.Field(4).(*array.StringBuilder).AppendNull()
			continue
		}
		note.Reset()
		for range 8 + random.IntN(24) {
			note.WriteString(words[random.IntN(len(words))])
			note.WriteByte(' ')
		}
		builder.Field(4).(*array.StringBuilder).Append(note.String())
	}
	batch := arrowbatch.NewBatch(builder.NewRecordBatch(), arrowbatch.Operations{})
	batch.Resource = "events"
	return batch
}

func rollRun() filament.RunSpec {
	return filament.RunSpec{Run: "roll", StartedAt: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC), Resources: []string{"events"}}
}

// openTestSink opens a sink on store that rolls at the smallest file size.
func openTestSink(t *testing.T, ctx context.Context, store multipartStore, format, compression string, run filament.RunSpec) (*Sink, filament.WritePolicy) {
	t.Helper()
	cfg, err := parseConfig(filament.NewConfig(map[string]any{
		"bucket": "bucket", "prefix": "exports", "file_format": format, "compression": compression,
		"part_size_mib": minPartSizeMiB, "max_file_size_mib": minFileSizeMiB, "upload_concurrency": 4,
	}))
	if err != nil {
		t.Fatal(err)
	}
	policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
	policy.Resource = "events"
	run.WritePolicies = map[string]filament.WritePolicy{"events": policy}
	sink := New()
	if err := sink.open(ctx, run, cfg, store); err != nil {
		t.Fatal(err)
	}
	return sink, policy
}

type rollManifest struct {
	Version   int `json:"version"`
	Resources []struct {
		Name  string `json:"name"`
		Rows  int64  `json:"rows"`
		Bytes int64  `json:"bytes"`
		Files []struct {
			Key    string `json:"key"`
			Rows   int64  `json:"rows"`
			Bytes  int64  `json:"bytes"`
			CRC32C string `json:"crc32c"`
		} `json:"files"`
	} `json:"resources"`
}

func readRollManifest(t *testing.T, store *limitStore, key string) rollManifest {
	t.Helper()
	stored, ok := store.object(key)
	if !ok {
		t.Fatal("success manifest missing")
	}
	var manifest rollManifest
	if err := json.Unmarshal(stored.body, &manifest); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	return manifest
}

// decodeRows reads one object as a standalone file and returns its row count.
func decodeRows(t *testing.T, format, compression string, body []byte) int64 {
	t.Helper()
	if format == "parquet" {
		reader, err := file.NewParquetReader(bytes.NewReader(body))
		if err != nil {
			t.Fatalf("open parquet object: %v", err)
		}
		defer reader.Close()
		return reader.NumRows()
	}
	var plain io.Reader = bytes.NewReader(body)
	if compression == "gzip" {
		gz, err := gzip.NewReader(plain)
		if err != nil {
			t.Fatalf("open gzip object: %v", err)
		}
		gz.Multistream(false)
		data, err := io.ReadAll(gz)
		if err != nil {
			t.Fatalf("read gzip object: %v", err)
		}
		if rest, _ := io.ReadAll(plain); len(rest) != 0 {
			t.Fatalf("%d bytes after the gzip stream", len(rest))
		}
		plain = bytes.NewReader(data)
	}
	if format == "json" {
		var rows []json.RawMessage
		if err := json.NewDecoder(plain).Decode(&rows); err != nil {
			t.Fatalf("decode JSON array object: %v", err)
		}
		return int64(len(rows))
	}
	var rows int64
	scanner := bufio.NewScanner(plain)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for scanner.Scan() {
		if !json.Valid(scanner.Bytes()) {
			t.Fatalf("invalid NDJSON line %d", rows+1)
		}
		rows++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}
