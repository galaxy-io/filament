package sink

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// TestQdrantSink_Spec verifies that the Qdrant sink returns a non-empty name and display title.
func TestQdrantSink_Spec(t *testing.T) {
	s := New()
	spec := s.Spec()

	if spec.Name != "qdrant" {
		t.Errorf("expected spec.Name to be 'qdrant', got %q", spec.Name)
	}

	if spec.DisplayName != "Qdrant" {
		t.Errorf("expected DisplayName to be 'Qdrant', got %q", spec.DisplayName)
	}
}

// TestQdrantSink_ParseConfig verifies parsing of raw configuration options into a Config struct.
func TestQdrantSink_ParseConfig(t *testing.T) {
	rawCfg := map[string]any{
		"url":         "http://localhost:6333",
		"collection":  "documents",
		"text_fields": []string{"title", "body"},
		"batch_size":  250,
	}

	cfg, err := ParseConfig(filament.NewConfig(rawCfg))
	if err != nil {
		t.Fatalf("unexpected error parsing config: %v", err)
	}

	if cfg.URL != "http://localhost:6333" {
		t.Errorf("expected URL to be 'http://localhost:6333', got %q", cfg.URL)
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

// TestQdrantSink_HTTPSSecurityCheck verifies that plain HTTP URLs are rejected when api_key is set.
func TestQdrantSink_HTTPSSecurityCheck(t *testing.T) {
	rawCfg := map[string]any{
		"url":     "http://remote-qdrant.example.com",
		"api_key": "secret-token",
	}

	_, err := ParseConfig(filament.NewConfig(rawCfg))
	if err == nil {
		t.Errorf("expected ParseConfig to error when api_key is configured over plain http, got nil")
	}
}

// TestQdrantSink_Lifecycle verifies Open, Apply, and Commit lifecycle execution for the Qdrant sink.
func TestQdrantSink_Lifecycle(t *testing.T) {
	ctx := context.Background()
	s := New()

	runSpec := filament.RunSpec{
		Run: filament.RunID("test-run-1"),
		Sink: filament.Ref{
			Config: map[string]any{
				"url":        "mock://localhost:6333",
				"collection": "test_collection",
			},
		},
	}

	if err := s.Open(ctx, runSpec); err != nil {
		t.Fatalf("failed to open qdrant sink: %v", err)
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

// TestQdrantSink_AbortRejection verifies that Apply rejects writes after Abort is called.
func TestQdrantSink_AbortRejection(t *testing.T) {
	ctx := context.Background()
	s := New()

	runSpec := filament.RunSpec{
		Run: filament.RunID("test-run-abort"),
		Sink: filament.Ref{
			Config: map[string]any{
				"url": "mock://localhost:6333",
			},
		},
	}

	if err := s.Open(ctx, runSpec); err != nil {
		t.Fatalf("failed to open qdrant sink: %v", err)
	}

	if err := s.Abort(ctx); err != nil {
		t.Fatalf("failed to abort qdrant sink: %v", err)
	}

	batch := arrowbatch.NewMarker()
	batch.Resource = "users"

	_, err := s.Apply(ctx, batch, filament.ApplyOptions{})
	if err == nil {
		t.Fatalf("expected Apply to return an error after Abort, got nil")
	}
}
