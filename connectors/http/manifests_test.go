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
	paths, err := filepath.Glob("manifests/*/*/manifest.yaml")
	if err != nil {
		t.Fatalf("glob manifests: %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no manifests found under manifests/")
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			m, err := manifest.Load(path)
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
	data, err := os.ReadFile("manifests/linear/unversioned/manifest.yaml")
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

// Twilio returns messages and calls newest first, so a paused incremental
// run must not commit the newest cursor seen so far. Both resources commit
// the checkpoint only when the scan completes, and both keep the top-level
// paginated row cursor the validator requires for that mode.
func TestTwilioIncrementalCommitsOnComplete(t *testing.T) {
	m, err := manifest.Load("manifests/twilio/2010-04-01/manifest.yaml")
	if err != nil {
		t.Fatalf("load Twilio manifest: %v", err)
	}
	for _, name := range []string{"messages", "calls"} {
		t.Run(name, func(t *testing.T) {
			var res *manifest.Resource
			for i := range m.Resources {
				if m.Resources[i].Name == name {
					res = &m.Resources[i]
				}
			}
			if res == nil {
				t.Fatalf("resource %s missing", name)
			}
			if res.Incremental == nil || !res.Incremental.CommitOnComplete {
				t.Fatalf("resource %s must set commit_on_complete", name)
			}
			if res.Parent != nil || res.Incremental.ResponseCursor != "" {
				t.Fatalf("resource %s must keep a top-level paginated row cursor for commit_on_complete", name)
			}
		})
	}
}
