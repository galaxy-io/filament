package httpapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// TestManifests runs every shipped manifest through the full parse pipeline:
// grammar schema, strict decode, normalization, and semantic validation. A
// manifest added without a dedicated source test is still validated here.
func TestManifests(t *testing.T) {
	paths, err := filepath.Glob("manifests/*.yaml")
	if err != nil {
		t.Fatalf("glob manifests: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no manifests found under manifests/")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read manifest: %v", err)
			}
			if _, err := manifest.Parse(data); err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
		})
	}
}
