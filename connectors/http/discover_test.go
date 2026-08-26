package httpapi

import (
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func TestResolveNameUsesOnlyDeclaredPaths(t *testing.T) {
	record := map[string]any{
		"display_name": "Declared name",
		"properties": map[string]any{
			"Arbitrary": map[string]any{
				"type":  "title",
				"title": []any{map[string]any{"plain_text": "Implicit provider name"}},
			},
		},
	}

	got := resolveName(record, manifest.ResourceMap{
		NamePaths: []string{"missing", "display_name"},
	})
	if got != "Declared name" {
		t.Fatalf("resolved name = %q, want declared path value", got)
	}

	got = resolveName(record, manifest.ResourceMap{NamePath: "missing"})
	if got != "" {
		t.Fatalf("resolved name = %q, want no provider-specific fallback", got)
	}
}
