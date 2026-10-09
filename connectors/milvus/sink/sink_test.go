package sink

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// TestMilvusSink_Spec verifies that the Milvus sink returns a non-empty name and display title.
func TestMilvusSink_Spec(t *testing.T) {
	s := New()
	spec := s.Spec()

	if spec.Name != "milvus" {
		t.Errorf("expected spec.Name to be 'milvus', got %q", spec.Name)
	}

	if spec.DisplayName != "Milvus" {
		t.Errorf("expected DisplayName to be 'Milvus', got %q", spec.DisplayName)
	}
}

// TestMilvusSink_ParseConfig verifies parsing of raw configuration options into a Config struct.
func TestMilvusSink_ParseConfig(t *testing.T) {
	rawCfg := map[string]any{
		"url":         "http://localhost:19530",
		"collection":  "documents",
		"text_fields": []string{"title", "body"},
		"batch_size":  250,
	}

	cfg, err := ParseConfig(filament.NewConfig(rawCfg))
	if err != nil {
		t.Fatalf("unexpected error parsing config: %v", err)
	}

	if cfg.URL != "http://localhost:19530" {
		t.Errorf("expected URL to be 'http://localhost:19530', got %q", cfg.URL)
	}

	if cfg.Collection != "documents" {
		t.Errorf("expected Collection to be 'documents', got %q", cfg.Collection)
	}

	if len(cfg.TextFields) != 2 || cfg.TextFields[0] != "title" || cfg.TextFields[1] != "body" {
		t.Errorf("expected TextFields to be ['title', 'body'], got %v", cfg.TextFields)
	}

	if cfg.BatchSize != 250 {
		t.Errorf("expected BatchSize to be 250, got %d", cfg.BatchSize)
	}
}

// TestMilvusSink_HTTPSSecurityCheck verifies that plain HTTP URLs are rejected when api_key is set.
func TestMilvusSink_HTTPSSecurityCheck(t *testing.T) {
	rawCfg := map[string]any{
		"url":     "http://remote-milvus.example.com",
		"api_key": "secret-token",
	}

	_, err := ParseConfig(filament.NewConfig(rawCfg))
	if err == nil {
		t.Errorf("expected ParseConfig to error when api_key is configured over plain http, got nil")
	}
}

// TestMilvusSink_Lifecycle verifies Open, Apply, and Commit lifecycle execution for the Milvus sink.
func TestMilvusSink_Lifecycle(t *testing.T) {
	ctx := context.Background()
	s := New()

	runSpec := filament.RunSpec{
		Run: filament.RunID("test-run-1"),
		Sink: filament.Ref{
			Config: map[string]any{
				"url":        "mock://localhost:19530",
				"collection": "test_collection",
			},
		},
	}

	if err := s.Open(ctx, runSpec); err != nil {
		t.Fatalf("failed to open milvus sink: %v", err)
	}

	batch := arrowbatch.NewMarker()
	batch.Resource = "users"

	receipt, err := s.Apply(ctx, batch, filament.ApplyOptions{})
	if err != nil {
		t.Fatalf("failed to apply batch: %v", err)
	}

	if receipt.Rows != 0 {
		t.Errorf("expected 0 rows written for marker batch, got %d", receipt.Rows)
	}

	if err := s.Commit(ctx); err != nil {
		t.Fatalf("failed to commit run: %v", err)
	}
}

// TestMilvusSink_AbortRejection verifies that Apply rejects writes after Abort is called.
func TestMilvusSink_AbortRejection(t *testing.T) {
	ctx := context.Background()
	s := New()

	runSpec := filament.RunSpec{
		Run: filament.RunID("test-run-abort"),
		Sink: filament.Ref{
			Config: map[string]any{
				"url": "mock://localhost:19530",
			},
		},
	}

	if err := s.Open(ctx, runSpec); err != nil {
		t.Fatalf("failed to open milvus sink: %v", err)
	}

	if err := s.Abort(ctx); err != nil {
		t.Fatalf("failed to abort milvus sink: %v", err)
	}

	batch := arrowbatch.NewMarker()
	batch.Resource = "users"

	_, err := s.Apply(ctx, batch, filament.ApplyOptions{})
	if err == nil {
		t.Fatalf("expected Apply to return an error after Abort, got nil")
	}
}
