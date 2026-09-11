package httpapi

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

// TestGitHubLive is opt-in and read-only. Credentials stay in a local file;
// logs contain resource counts, never tokens or source row contents. Use Go's
// subtest selection to verify successive batches of resources.
func TestGitHubLive(t *testing.T) {
	path := os.Getenv("FILAMENT_GITHUB_TOKEN_FILE")
	if path == "" {
		t.Skip("set FILAMENT_GITHUB_TOKEN_FILE and FILAMENT_GITHUB_ORGANIZATION for live tests")
	}
	organization := os.Getenv("FILAMENT_GITHUB_ORGANIZATION")
	if organization == "" {
		t.Fatal("FILAMENT_GITHUB_ORGANIZATION is required")
	}
	token, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("cannot read GitHub token file: ", err)
	}
	src := NewGitHub()
	// Optional narrow pass; without it, run the shipped organization walk.
	if repo := os.Getenv("FILAMENT_GITHUB_REPOSITORY"); repo != "" {
		for i := range src.embeddedManifest.Resources {
			r := &src.embeddedManifest.Resources[i]
			if r.Name == "repositories" {
				r.Path = "/repos/" + organization + "/" + repo
				r.Query = nil
				r.Response.Cardinality = "one"
				r.Pagination = manifest.PaginationSpec{}
			}
		}
	}
	ctx := context.Background()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"organization": organization, "token": strings.TrimSpace(string(token)),
	})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(ctx)
	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	for _, resource := range discovered.Resources {
		t.Run(resource.Name, func(t *testing.T) {
			var sink collectSink
			if err := src.Extract(ctx, &sink, filament.ExtractOpts{Resources: []string{resource.Name}}); err != nil {
				t.Fatal(err)
			}
			keys := make(map[string]bool, len(sink.records))
			for _, rec := range sink.records {
				if rec.Resource != resource.Name {
					t.Fatal("unselected parent was emitted")
				}
				if len(resource.PrimaryKey) == 0 {
					continue // Heterogeneous timelines are snapshot-only.
				}
				var row map[string]json.RawMessage
				if err := json.Unmarshal(rec.Data, &row); err != nil {
					t.Fatal(err)
				}
				key := map[string]json.RawMessage{}
				for _, name := range resource.PrimaryKey {
					if len(row[name]) == 0 || string(row[name]) == "null" {
						t.Fatalf("null primary-key column %s", name)
					}
					key[name] = row[name]
				}
				encoded, _ := json.Marshal(key)
				if keys[string(encoded)] {
					t.Fatal("duplicate primary key across pages or parents")
				}
				keys[string(encoded)] = true
			}
			t.Logf("extracted and decoded %d rows; %d unique keys", len(sink.records), len(keys))
			if len(sink.records) == 0 {
				t.Log("empty resource: request verified; populated payload still requires fixture coverage")
			}
		})
	}
}
