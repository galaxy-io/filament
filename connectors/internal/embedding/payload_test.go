package embedding_test

import (
	"reflect"
	"testing"

	"github.com/galaxy-io/filament/connectors/internal/embedding"
)

func TestPayload(t *testing.T) {
	got := embedding.Payload(embedding.Document{
		ID:       "42",
		Text:     "hello",
		Vector:   []float32{1, 2},
		Metadata: map[string]any{"tag": "a", "views": int64(3)},
	})
	want := map[string]any{
		embedding.FieldSourceID: "42",
		embedding.FieldDocument: "hello",
		"tag":                   "a",
		"views":                 int64(3),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Payload = %v, want %v", got, want)
	}
}

func TestPayloadOmitsEmptyText(t *testing.T) {
	got := embedding.Payload(embedding.Document{ID: "1"})
	if _, ok := got[embedding.FieldDocument]; ok {
		t.Fatalf("Payload = %v, want no %q field for empty text", got, embedding.FieldDocument)
	}
}

func TestPayloadNamespacesReservedKeys(t *testing.T) {
	got := embedding.Payload(embedding.Document{
		ID:   "row-1",
		Text: "real text",
		Metadata: map[string]any{
			"id":         "spoofed",
			"vector":     "spoofed",
			"document":   "spoofed",
			"_source_id": "spoofed",
		},
	})
	want := map[string]any{
		embedding.FieldSourceID: "row-1",
		embedding.FieldDocument: "real text",
		"_meta_id":              "spoofed",
		"_meta_vector":          "spoofed",
		"_meta_document":        "spoofed",
		"_meta__source_id":      "spoofed",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Payload = %v, want %v", got, want)
	}
}

func TestPayloadKeepsBothCollidingKeys(t *testing.T) {
	// A source with both "id" and "_meta_id" columns must keep both values.
	for range 20 { // map iteration order varies; the result must not
		got := embedding.Payload(embedding.Document{
			ID:       "row-1",
			Metadata: map[string]any{"id": "from id", "_meta_id": "from _meta_id"},
		})
		want := map[string]any{
			embedding.FieldSourceID: "row-1",
			"_meta_id":              "from _meta_id",
			"_meta__meta_id":        "from id",
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Payload = %v, want %v", got, want)
		}
	}
}
