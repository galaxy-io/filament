package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

const composedRoot = `version: 1
name: composed
display_name: Composed
description: Fragment loader fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection: {base_url: https://example.com}
includes: [parts/resources.yaml, fields.yaml]
`

func fragmentFixture() fstest.MapFS {
	return fstest.MapFS{
		"v1/manifest.yaml":        {Data: []byte(composedRoot)},
		"v1/parts/resources.yaml": {Data: []byte("resources:\n  - name: items\n    path: /items\n    primary_key: [id]\n    use_fields: [identity]\n")},
		"v1/fields.yaml":          {Data: []byte("field_sets:\n  identity:\n    id: string\n    label: string?\n")},
	}
}

func TestComposedManifest(t *testing.T) {
	files := fragmentFixture()
	m, err := LoadFS(files, "v1/manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Resources) != 1 || len(m.Resources[0].Fields) != 2 || m.Resources[0].Fields[0].Name != "id" || m.Resources[0].Fields[1].Name != "label" {
		t.Fatalf("field ordering lost: %+v", m.Resources)
	}
	// Nested includes resolve relative to their own file, within the same root.
	files["v1/parts/resources.yaml"].Data = append(files["v1/parts/resources.yaml"].Data, []byte("includes: [nested.yaml]\n")...)
	files["v1/parts/nested.yaml"] = &fstest.MapFile{Data: []byte("resources: [{name: extra, path: /extra}]\n")}
	if m, err = LoadFS(files, "v1/manifest.yaml"); err != nil || len(m.Resources) != 2 {
		t.Fatalf("nested includes: %v", err)
	}
}

func TestComposedManifestRejectsInvalidIncludes(t *testing.T) {
	for _, tc := range []struct{ name, entry, data, want string }{
		{"missing", "v1/parts/resources.yaml", "includes: [absent.yaml]", "absent.yaml"},
		{"escape", "v1/parts/resources.yaml", "includes: [../../secret.yaml]", "escapes"},
		{"absolute", "v1/parts/resources.yaml", "includes: [/secret.yaml]", "escapes"},
		{"remote", "v1/parts/resources.yaml", "includes: ['https://example.com/x.yaml']", "local path"},
		{"cycle", "v1/parts/resources.yaml", "includes: [../manifest.yaml]", "cycle"},
		{"repeat", "v1/parts/resources.yaml", "includes: [../fields.yaml]", "more than once"},
		{"connection override", "v1/parts/resources.yaml", "connection: {base_url: https://other.example.com}", "fragments may only"},
		{"duplicate fieldset", "v1/parts/resources.yaml", "field_sets: {identity: {other: string}}", "duplicate field set"},
		{"duplicate resource", "v1/fields.yaml", "resources: [{name: items, path: /other}]", "duplicate resource"},
		{"invalid fields", "v1/fields.yaml", "field_sets: {identity: {id: unsupported}}", "grammar"},
		{"duplicate YAML key", "v1/fields.yaml", "field_sets: {}\nfield_sets: {}", "already defined"},
		{"invalid include", "v1/fields.yaml", "includes: [42]", "nonempty paths"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := fragmentFixture()
			files[tc.entry].Data = []byte(tc.data)
			if _, err := LoadFS(files, "v1/manifest.yaml"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %s", err, tc.want)
			}
		})
	}
}

func TestLocalManifestConfinesSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "manifest.yaml"), []byte(strings.Replace(composedRoot, "[parts/resources.yaml, fields.yaml]", "[outside.yaml]", 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "outside.yaml"), []byte("resources: []"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "outside.yaml"), filepath.Join(root, "outside.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(filepath.Join(root, "manifest.yaml")); err == nil {
		t.Fatal("followed a symlink outside the manifest directory")
	}
}

func TestFieldOverrides(t *testing.T) {
	files := fragmentFixture()
	files["v1/parts/resources.yaml"].Data = []byte(`resources:
  - name: items
    path: /items
    primary_key: [id]
    use_fields: [identity]
    field_overrides:
      label: {path: DisplayName, type: string}
    fields:
      extra: string?
  - name: other
    path: /other
    primary_key: [id]
    use_fields: [identity]
`)
	m, err := LoadFS(files, "v1/manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	fields := m.Resources[0].Fields
	if len(fields) != 3 || fields[0].Name != "id" || fields[1].Name != "label" || fields[2].Name != "extra" || fields[1].Nullable || fields[1].Path != "DisplayName" {
		t.Fatalf("override lost order or properties: %+v", fields)
	}
	if !m.Resources[1].Fields[1].Nullable || m.Resources[1].Fields[1].Path != "label" || !m.FieldSets["identity"][1].Nullable {
		t.Fatal("override mutated shared fields")
	}
	files["v1/parts/resources.yaml"].Data = []byte(strings.Replace(string(files["v1/parts/resources.yaml"].Data), "label: {path:", "missing: {path:", 1))
	if _, err := LoadFS(files, "v1/manifest.yaml"); err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unknown override accepted: %v", err)
	}
}

func TestParseRejectsUnresolvedIncludes(t *testing.T) {
	if _, err := Parse([]byte(composedRoot)); err == nil || !strings.Contains(err.Error(), "includes require") {
		t.Fatalf("unresolved includes: %v", err)
	}
}

func TestManifestIncludeLimits(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		files := fstest.MapFS{}
		for i := 0; i < 18; i++ {
			files[fmt.Sprintf("%d.yaml", i)] = &fstest.MapFile{Data: []byte(fmt.Sprintf("includes: [%d.yaml]\n", i+1))}
		}
		if _, err := ReadFS(files, "0.yaml"); err == nil || !strings.Contains(err.Error(), "limit exceeded") {
			t.Fatalf("depth limit: %v", err)
		}
	})
	t.Run("files", func(t *testing.T) {
		files := fstest.MapFS{}
		var root strings.Builder
		root.WriteString("includes:\n")
		for i := 0; i < 128; i++ {
			name := fmt.Sprintf("%d.yaml", i)
			fmt.Fprintf(&root, "  - %s\n", name)
			files[name] = &fstest.MapFile{Data: []byte("resources: []\n")}
		}
		files["root.yaml"] = &fstest.MapFile{Data: []byte(root.String())}
		if _, err := ReadFS(files, "root.yaml"); err == nil || !strings.Contains(err.Error(), "limit exceeded") {
			t.Fatalf("file limit: %v", err)
		}
	})
	t.Run("bytes", func(t *testing.T) {
		files := fstest.MapFS{"root.yaml": {Data: []byte(strings.Repeat(" ", 32<<20+1))}}
		if _, err := ReadFS(files, "root.yaml"); err == nil || !strings.Contains(err.Error(), "32 MiB") {
			t.Fatalf("byte limit: %v", err)
		}
	})
}
