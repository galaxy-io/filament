package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/internal/scalar"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func collect(t *testing.T, r Reader, body string) []map[string]any {
	t.Helper()
	var out []map[string]any
	err := r.Read(context.Background(), strings.NewReader(body), func(rec map[string]any) error {
		out = append(out, rec)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestStreamReadersPreserveLargeIntegers(t *testing.T) {
	const id = "9007199254740993"
	cases := []struct {
		name string
		spec *manifest.StreamSpec
		body string
	}{
		{"ndjson", &manifest.StreamSpec{Type: "ndjson"}, fmt.Sprintf("{\"id\": %s, \"v\": 1.5}\n", id)},
		{"sse", &manifest.StreamSpec{Type: "sse"}, fmt.Sprintf("data: {\"id\": %s}\n\n", id)},
		{"chunked", &manifest.StreamSpec{Type: "chunked_array"}, fmt.Sprintf("[{\"id\": %s}]", id)},
	}
	for _, tc := range cases {
		r, err := New(tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		recs := collect(t, r, tc.body)
		if len(recs) != 1 {
			t.Fatalf("%s: got %d records", tc.name, len(recs))
		}
		got, ok := scalar.String(recs[0]["id"])
		if !ok {
			t.Fatalf("%s: id not a scalar: %T", tc.name, recs[0]["id"])
		}
		if got != id {
			t.Errorf("%s: id corrupted: got %s (type %T), want %s", tc.name, got, recs[0]["id"], id)
		}
	}
}

func TestDecodeRecord(t *testing.T) {
	rec, err := decodeRecord([]byte(`{"id": 9007199254740993, "v": 1.5}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := rec["id"]; got != json.Number("9007199254740993") {
		t.Errorf("id = %v (%T), want exact json.Number", got, got)
	}
	if got := rec["v"]; got != json.Number("1.5") {
		t.Errorf("v = %v (%T), want json.Number 1.5", got, got)
	}
	if _, err := decodeRecord([]byte(`{"id": 1} {"id": 2}`)); err == nil {
		t.Error("trailing data after the object: want an error, got none")
	}
	if _, err := decodeRecord([]byte(`{"id": `)); err == nil {
		t.Error("truncated object: want an error, got none")
	}
}
