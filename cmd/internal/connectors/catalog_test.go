package connectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/registry"
)

func TestSourcesFromEnvProtectsBundledRegistrations(t *testing.T) {
	for _, version := range []string{"2022-11-28", "2099-01-01"} {
		t.Run(version, func(t *testing.T) {
			directory := t.TempDir()
			provider := filepath.Join(directory, "github")
			if err := os.MkdirAll(filepath.Join(provider, version), 0700); err != nil {
				t.Fatal(err)
			}
			registration := strings.ReplaceAll(`{"schema_version":1,"name":"github","default_version":"2022-11-28","versions":{"2022-11-28":{"maturity":"alpha"}}}`, "2022-11-28", version)
			if err := os.WriteFile(filepath.Join(provider, "registry.json"), []byte(registration), 0600); err != nil {
				t.Fatal(err)
			}
			manifest := strings.ReplaceAll(`version: 1
name: github
api_version: "2022-11-28"
display_name: Private GitHub
description: Private catalog override
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields:
      id: string
`, "2022-11-28", version)
			if err := os.WriteFile(filepath.Join(provider, version, "manifest.yaml"), []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}

			t.Setenv("HTTP_MANIFESTS_DIR", directory)
			sources, err := SourcesFromEnv()
			if version == "2022-11-28" {
				if err == nil || sources != nil || !strings.Contains(err.Error(), "HTTP_MANIFESTS_DIR") || !strings.Contains(err.Error(), "github@2022-11-28") {
					t.Fatalf("bundled version replacement accepted: %v, %v", sources, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"github", "github@" + version} {
					spec, err := sources.Spec(name)
					if err != nil || spec.DisplayName != "Private GitHub" {
						t.Fatalf("%s: %+v, %v", name, spec, err)
					}
				}
				pinned, err := sources.Spec("github@2022-11-28")
				if err != nil || pinned.DisplayName != "GitHub" {
					t.Fatalf("bundled pin replaced: %+v, %v", pinned, err)
				}
				if _, err := sources.Resolve("postgres"); err != nil {
					t.Fatal(err)
				}
			}
			original, err := registry.DefaultSources.Spec("github")
			if err != nil || original.DisplayName != "GitHub" {
				t.Fatalf("global catalog mutated: %+v, %v", original, err)
			}
		})
	}
}

func TestSourcesFromEnvDefaultAndInvalidDirectory(t *testing.T) {
	t.Setenv("HTTP_MANIFESTS_DIR", "")
	sources, err := SourcesFromEnv()
	if err != nil || sources != registry.DefaultSources {
		t.Fatalf("default sources=%v err=%v", sources, err)
	}
	t.Setenv("HTTP_MANIFESTS_DIR", filepath.Join(t.TempDir(), "missing"))
	if sources, err := SourcesFromEnv(); err == nil || sources != nil || !strings.Contains(err.Error(), "HTTP_MANIFESTS_DIR") {
		t.Fatalf("missing directory: %v, %v", sources, err)
	}
}
