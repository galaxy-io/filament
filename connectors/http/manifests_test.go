package httpapi

import (
	"os"
	"path/filepath"
	"strings"
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
			m, err := manifest.Parse(data)
			if err != nil {
				t.Fatalf("parse manifest: %v", err)
			}
			if m.DisplayName == "" || m.Description == "" || m.DarkLogoURL == "" || m.LightLogoURL == "" {
				t.Fatalf("catalog metadata must be manifest-owned: %#v", m)
			}
		})
	}
}

func TestLinearManifestUsesExclusiveIncrementalBoundary(t *testing.T) {
	data, err := os.ReadFile("manifests/linear.yaml")
	if err != nil {
		t.Fatalf("read Linear manifest: %v", err)
	}
	manifestText := string(data)
	if strings.Contains(manifestText, "updatedAt: { gte: $updatedAfter }") {
		t.Fatal("Linear incremental queries must not replay the saved watermark boundary")
	}
	if got := strings.Count(manifestText, "updatedAt: { gt: $updatedAfter }"); got != 4 {
		t.Fatalf("exclusive Linear incremental filters = %d, want 4", got)
	}
}
