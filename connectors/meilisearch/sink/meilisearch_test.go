package sink

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		raw     map[string]any
		wantURL string
		wantErr bool
	}{
		{
			name:    "valid full url",
			raw:     map[string]any{"url": "http://localhost:7700/"},
			wantURL: "http://localhost:7700",
		},
		{
			name:    "url without scheme defaults to http",
			raw:     map[string]any{"url": "127.0.0.1:7700"},
			wantURL: "http://127.0.0.1:7700",
		},
		{
			name:    "host alias for url",
			raw:     map[string]any{"host": "https://meili.example.com"},
			wantURL: "https://meili.example.com",
		},
		{
			name:    "missing url",
			raw:     map[string]any{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := ParseConfig(filament.NewConfig(tt.raw))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && cfg.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", cfg.URL, tt.wantURL)
			}
		})
	}
}

func TestSinkSpec(t *testing.T) {
	sink := New()
	spec := sink.Spec()

	if spec.Name != "meilisearch" {
		t.Errorf("Spec.Name = %q, want meilisearch", spec.Name)
	}
	if !spec.Capabilities.EncodedIntegrity {
		t.Error("Spec does not advertise EncodedIntegrity")
	}
	if !spec.Capabilities.Schematized {
		t.Error("Spec does not advertise Schematized")
	}
	if !spec.Capabilities.Upsertable {
		t.Error("Spec does not advertise Upsertable")
	}
	if spec.Capabilities.PreferredBatchRows != defaultBatchSize {
		t.Errorf("PreferredBatchRows = %d, want %d", spec.Capabilities.PreferredBatchRows, defaultBatchSize)
	}

	expectedModes := map[filament.WriteMode]bool{
		filament.WriteReplace: false,
		filament.WriteAppend:  false,
		filament.WriteUpsert:  false,
		filament.WriteMerge:   false,
	}
	for _, p := range spec.Capabilities.WritePolicies {
		expectedModes[p.Mode] = true
		if p.Durability != filament.DurabilityAfterCommit {
			t.Errorf("policy %q durability = %q, want %q", p.Mode, p.Durability, filament.DurabilityAfterCommit)
		}
	}
	for mode, found := range expectedModes {
		if !found {
			t.Errorf("write mode %q not found in sink capabilities", mode)
		}
	}
}

func TestSanitizeIndexUID(t *testing.T) {
	cases := []struct {
		prefix   string
		resource string
		want     string
	}{
		{"", "users", "users"},
		{"pg_", "users", "pg_users"},
		{"postgres", "products", "postgres_products"},
		{"", "public.users", "public_users"},
		{"", "my-db.schema/table", "my-db_schema_table"},
		{"", "???", "default"},
	}

	for _, c := range cases {
		got := sanitizeIndexUID(c.prefix, c.resource)
		if got != c.want {
			t.Errorf("sanitizeIndexUID(%q, %q) = %q, want %q", c.prefix, c.resource, got, c.want)
		}
	}
}

func TestClientNDJSONWithGzip(t *testing.T) {
	var receivedBody []byte
	var receivedContentEncoding string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"available"}`))
		case "/indexes/test/documents":
			receivedContentEncoding = r.Header.Get("Content-Encoding")
			receivedContentType = r.Header.Get("Content-Type")

			var reader io.Reader = r.Body
			if receivedContentEncoding == "gzip" {
				gz, err := gzip.NewReader(r.Body)
				if err != nil {
					t.Fatalf("failed to create gzip reader: %v", err)
				}
				defer gz.Close()
				reader = gz
			}
			var err error
			receivedBody, err = io.ReadAll(reader)
			if err != nil {
				t.Fatalf("failed to read body: %v", err)
			}

			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(TaskResponse{
				TaskUID:  42,
				IndexUID: "test",
				Status:   "enqueued",
				Type:     "documentAdditionOrUpdate",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key", true)

	// Verify health check
	if err := client.Health(context.Background()); err != nil {
		t.Fatalf("Health() error = %v", err)
	}

	// Send NDJSON document
	payload := []byte("{\"id\":1,\"name\":\"Alice\"}\n{\"id\":2,\"name\":\"Bob\"}\n")
	task, err := client.AddDocumentsNDJSON(context.Background(), "test", "id", payload, false)
	if err != nil {
		t.Fatalf("AddDocumentsNDJSON() error = %v", err)
	}

	if task == nil || task.TaskUID != 42 {
		t.Fatalf("expected task UID 42, got %+v", task)
	}

	if receivedContentType != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want application/x-ndjson", receivedContentType)
	}
	if receivedContentEncoding != "gzip" {
		t.Errorf("Content-Encoding = %q, want gzip", receivedContentEncoding)
	}
	if string(receivedBody) != string(payload) {
		t.Errorf("received decompressed payload:\ngot  %q\nwant %q", string(receivedBody), string(payload))
	}
}

func TestClientWaitForTask(t *testing.T) {
	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/tasks/100") {
			count := pollCount.Add(1)
			status := "processing"
			if count >= 3 {
				status = "succeeded"
			}
			_ = json.NewEncoder(w).Encode(TaskResult{
				UID:      100,
				IndexUID: "test",
				Status:   status,
				Type:     "documentAdditionOrUpdate",
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient(server.URL, "", false)
	res, err := client.WaitForTask(context.Background(), 100, 2*time.Second)
	if err != nil {
		t.Fatalf("WaitForTask error = %v", err)
	}
	if res.Status != "succeeded" {
		t.Fatalf("Task status = %q, want succeeded", res.Status)
	}
	if pollCount.Load() < 3 {
		t.Errorf("poll count = %d, expected at least 3", pollCount.Load())
	}
}

func TestSinkLifecycleWithMockServer(t *testing.T) {
	var receivedDocs bytes.Buffer

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"available"}`))
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/indexes/"):
			w.WriteHeader(http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/indexes":
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(TaskResponse{TaskUID: 1, Status: "enqueued"})
		case strings.HasSuffix(r.URL.Path, "/documents"):
			reader := r.Body
			if r.Header.Get("Content-Encoding") == "gzip" {
				gz, _ := gzip.NewReader(r.Body)
				reader = gz
			}
			_, _ = io.Copy(&receivedDocs, reader)
			w.WriteHeader(http.StatusAccepted)
			_ = json.NewEncoder(w).Encode(TaskResponse{TaskUID: 2, Status: "enqueued"})
		case strings.HasPrefix(r.URL.Path, "/tasks/"):
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(TaskResult{
				UID:    2,
				Status: "succeeded",
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	sink := New()
	ctx := context.Background()

	// Open
	err := sink.Open(ctx, filament.RunSpec{
		Run: "run-1",
		Sink: filament.Ref{
			Config: map[string]any{
				"url":            server.URL,
				"gzip":           false,
				"wait_for_tasks": true,
			},
		},
	})
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// EnsureSchema
	schema := rowmodel.Schema{
		Resource:   "products",
		PrimaryKey: []string{"id"},
		Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt64},
			{Name: "title", Logical: rowmodel.LogicalString},
			{Name: "price", Logical: rowmodel.LogicalFloat64},
		},
	}
	if err := sink.EnsureSchema(ctx, "products", schema); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Build Arrow Batch
	arrowSchema := arrowbatch.Schema(schema)
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowSchema)
	builder.Field(0).(*array.Int64Builder).Append(101)
	builder.Field(1).(*array.StringBuilder).Append("Widget")
	builder.Field(2).(*array.Float64Builder).Append(19.99)
	rows := builder.NewRecordBatch()
	builder.Release()

	b := arrowbatch.NewBatch(rows, arrowbatch.Operations{})
	b.Resource = "products"
	defer b.Release()

	// Apply
	opts := filament.ApplyOptions{
		Policy: filament.WritePolicy{
			Capability: filament.WritePolicyCapability{
				Mode: filament.WriteAppend,
			},
		},
	}
	receipt, err := sink.Apply(ctx, b, opts)
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	if receipt.Rows != 1 {
		t.Errorf("receipt.Rows = %d, want 1", receipt.Rows)
	}
	if receipt.WriteCRC != b.IntegrityCRC() {
		t.Errorf("receipt.WriteCRC = %08x, want %08x", receipt.WriteCRC, b.IntegrityCRC())
	}
	if receipt.EncodedCRC == nil {
		t.Fatal("receipt.EncodedCRC is nil")
	}

	expectedNDJSON := "{\"id\":101,\"title\":\"Widget\",\"price\":19.99}\n"
	if receivedDocs.String() != expectedNDJSON {
		t.Errorf("received NDJSON:\ngot  %q\nwant %q", receivedDocs.String(), expectedNDJSON)
	}

	expectedCRC := crc32.Checksum([]byte(expectedNDJSON), crc32.MakeTable(crc32.Castagnoli))
	if *receipt.EncodedCRC != expectedCRC {
		t.Errorf("receipt.EncodedCRC = %08x, want %08x", *receipt.EncodedCRC, expectedCRC)
	}

	// Commit
	if err := sink.Commit(ctx); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
}
