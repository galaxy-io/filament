package httpapi

import (
	"encoding/json"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/registry"
)

func catalogSource(t *testing.T, key string) *Source {
	t.Helper()
	source, err := registry.DefaultSources.Resolve(key)
	if err != nil {
		t.Fatal(err)
	}
	return source.(*Source)
}

func catalogManifest(t *testing.T, name, version string) []byte {
	t.Helper()
	data, err := catalogFS.ReadFile("manifests/" + name + "/" + version + "/manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestEmbeddedCatalogRegistration(t *testing.T) {
	entries, err := loadCatalog(catalogFS)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		t.Run(entry.key(), func(t *testing.T) {
			source := catalogSource(t, entry.key())
			spec, err := registry.DefaultSources.Spec(entry.key())
			if err != nil {
				t.Fatal(err)
			}
			if spec.Name != entry.key() || spec.Version != entry.version || spec.Maturity != entry.maturity {
				t.Fatalf("incorrect catalog identity: %#v", spec)
			}
			second := catalogSource(t, entry.key())
			if source == second || source.embeddedManifest == second.embeddedManifest {
				t.Fatal("source instances share mutable state")
			}
			if entry.isDefault && catalogSource(t, entry.name).Spec().Name != entry.key() {
				t.Fatal("default did not resolve to concrete version")
			}
		})
	}
	specs := registry.DefaultSources.Specs()
	for _, entry := range entries {
		count, aliases := 0, 0
		for _, spec := range specs {
			if spec.Name == entry.name {
				aliases++
				if entry.isDefault && (spec.Version != entry.version || spec.Maturity != entry.maturity) {
					t.Fatalf("alias metadata = %#v", spec)
				}
			}
			if spec.Name == entry.key() {
				count++
			}
		}
		if entry.isDefault && aliases != 1 {
			t.Fatalf("alias %s listed %d times", entry.name, aliases)
		}
		if count != 1 {
			t.Fatalf("%s listed %d times", entry.key(), count)
		}
	}
}

func testCatalog(t *testing.T) fstest.MapFS {
	t.Helper()
	registration := catalogRegistry{SchemaVersion: 1, Name: "example", DefaultVersion: "v2", Versions: map[string]catalogVersion{
		"v1": {Maturity: filament.MaturityBeta}, "v2": {Maturity: filament.MaturityAlpha},
	}}
	data, err := json.Marshal(registration)
	if err != nil {
		t.Fatal(err)
	}
	return fstest.MapFS{
		"manifests/example/registry.json":    {Data: data},
		"manifests/example/v1/manifest.yaml": {Data: exampleAPImanifest(t, "v1")},
		"manifests/example/v2/manifest.yaml": {Data: exampleAPImanifest(t, "v2")},
	}
}

func TestCatalogMultipleVersions(t *testing.T) {
	entries, err := loadCatalog(testCatalog(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || entries[0].key() != "example@v1" || entries[1].key() != "example@v2" || entries[0].isDefault || !entries[1].isDefault {
		t.Fatalf("entries: %#v", entries)
	}
	sources := registry.NewSources()
	for _, entry := range entries {
		sources.RegisterWithMaturity(entry.key(), entry.maturity, entry.source)
	}
	sources.RegisterAlias("example", "example@v2")
	for _, key := range []string{"example@v1", "example@v2", "example"} {
		spec, err := sources.Spec(key)
		if err != nil {
			t.Fatal(err)
		}
		wantVersion := "v2"
		if key == "example@v1" {
			wantVersion = "v1"
		}
		if spec.Version != wantVersion || spec.APIVersion != wantVersion || spec.Name != "example@"+wantVersion {
			t.Fatalf("%s: %#v", key, spec)
		}
	}
	if _, err := sources.Resolve("example@v3"); err == nil {
		t.Fatal("unknown version resolved")
	}
}

func TestCatalogRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(fstest.MapFS)
		want   string
	}{
		{"missing registry", func(f fstest.MapFS) { delete(f, "manifests/example/registry.json") }, "registry.json"},
		{"missing manifest", func(f fstest.MapFS) { delete(f, "manifests/example/v2/manifest.yaml") }, "v2/manifest.yaml"},
		{"invalid manifest", func(f fstest.MapFS) { f["manifests/example/v2/manifest.yaml"].Data = []byte("version: 99") }, "v2/manifest.yaml"},
		{"mismatched manifest name", func(f fstest.MapFS) {
			f["manifests/example/v2/manifest.yaml"].Data = []byte(strings.Replace(string(f["manifests/example/v2/manifest.yaml"].Data), "name: example", "name: other", 1))
		}, "name must match registry"},
		{"unlisted version", func(f fstest.MapFS) {
			f["manifests/example/v3/manifest.yaml"] = f["manifests/example/v1/manifest.yaml"]
		}, "unlisted version"},
		{"flat manifest", func(f fstest.MapFS) { f["manifests/old.yaml"] = &fstest.MapFile{Data: []byte("version: 1")} }, "expected connector directory"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			files := testCatalog(t)
			test.mutate(files)
			entries, err := loadCatalog(files)
			if err == nil || !strings.Contains(err.Error(), test.want) || entries != nil {
				t.Fatalf("entries=%v err=%v, want %q", entries, err, test.want)
			}
		})
	}
	for _, test := range []struct{ name, document, want string }{
		{"unknown field", `{"schema_version":1,"extra":true}`, "unknown field"},
		{"unsupported schema", `{"schema_version":2}`, "unsupported schema_version"},
		{"name mismatch", `{"schema_version":1,"name":"other"}`, "name must match directory"},
		{"missing default", `{"schema_version":1,"name":"example","versions":{"v1":{"maturity":"beta"}}}`, "default_version"},
		{"invalid version", `{"schema_version":1,"name":"example","default_version":"../v1","versions":{"../v1":{"maturity":"beta"}}}`, "invalid API version"},
		{"invalid maturity", `{"schema_version":1,"name":"example","default_version":"v1","versions":{"v1":{"maturity":"experimental"}}}`, "invalid maturity"},
		{"trailing JSON", `{} {}`, "one JSON document"},
	} {
		t.Run(test.name, func(t *testing.T) {
			files := testCatalog(t)
			files["manifests/example/registry.json"].Data = []byte(test.document)
			if _, err := loadCatalog(files); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error=%v, want %q", err, test.want)
			}
		})
	}
	t.Run("empty catalog", func(t *testing.T) {
		if _, err := loadCatalog(fstest.MapFS{"manifests": {Mode: fs.ModeDir}}); err == nil {
			t.Fatal("empty catalog accepted")
		}
	})
}

// Use a synthetic provider for version-selection fixtures, not invented vendor releases.
func exampleAPImanifest(t *testing.T, version string) []byte {
	t.Helper()
	declaration := ""
	if version != unversionedAPI {
		declaration = `api_version: "` + version + `"`
	}
	return []byte(`version: 1
name: example
display_name: Example
description: Example API
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
` + declaration + `
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields:
      id: string
`)
}

func TestCatalogAPIVersionIdentifiers(t *testing.T) {
	for _, version := range []string{"v2", "v8", "3.0", "2022-11-28", "2026-07", "2026-07-29.dahlia", "unversioned"} {
		t.Run(version, func(t *testing.T) {
			registration := catalogRegistry{SchemaVersion: 1, Name: "example", DefaultVersion: version, Versions: map[string]catalogVersion{version: {Maturity: filament.MaturityAlpha}}}
			data, err := json.Marshal(registration)
			if err != nil {
				t.Fatal(err)
			}
			files := fstest.MapFS{
				"manifests/example/registry.json":                 {Data: data},
				"manifests/example/" + version + "/manifest.yaml": {Data: exampleAPImanifest(t, version)},
			}
			entries, err := loadCatalog(files)
			if err != nil {
				t.Fatal(err)
			}
			spec := entries[0].source().Spec()
			wantAPI := version
			if version == unversionedAPI {
				wantAPI = ""
			}
			if spec.Name != "example@"+version || spec.Version != version || spec.APIVersion != wantAPI {
				t.Fatalf("spec=%+v", spec)
			}

			// A directory cannot claim a different version than its API contract.
			files["manifests/example/"+version+"/manifest.yaml"].Data = exampleAPImanifest(t, "v999")
			if _, err := loadCatalog(files); err == nil || !strings.Contains(err.Error(), "api_version") {
				t.Fatalf("mismatched API version: %v", err)
			}
			if version != unversionedAPI {
				files["manifests/example/"+version+"/manifest.yaml"].Data = exampleAPImanifest(t, unversionedAPI)
				if _, err := loadCatalog(files); err == nil || !strings.Contains(err.Error(), "api_version") {
					t.Fatalf("missing API version: %v", err)
				}
			}
		})
	}
}

func TestCatalogRejectsUnsafeAPIVersionPaths(t *testing.T) {
	for _, version := range []string{"", ".", "..", "../v2", "v2/nested", "v2\\nested", "/v2", "v2@other", "v2?x", "v2 ", ".hidden"} {
		t.Run(version, func(t *testing.T) {
			registration := catalogRegistry{SchemaVersion: 1, Name: "example", DefaultVersion: version, Versions: map[string]catalogVersion{version: {Maturity: filament.MaturityAlpha}}}
			data, err := json.Marshal(registration)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := loadCatalog(fstest.MapFS{"manifests/example/registry.json": {Data: data}}); err == nil || !strings.Contains(err.Error(), "invalid API version") {
				t.Fatalf("version %q: %v", version, err)
			}
		})
	}
}
