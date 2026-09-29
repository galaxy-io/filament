package httpapi

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDirectorySnapshotsPrivateCatalog(t *testing.T) {
	directory := t.TempDir()
	for name, file := range testCatalog(t) {
		destination := filepath.Join(directory, strings.TrimPrefix(name, "manifests/"))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, file.Data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	sources, err := LoadDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	source, err := sources.Resolve("example")
	if err != nil {
		t.Fatal(err)
	}
	if source.Spec().Name != "example@v2" {
		t.Fatalf("alias resolved to %q", source.Spec().Name)
	}
	if _, err := sources.Resolve("example@v1"); err != nil {
		t.Fatal(err)
	}
	// A process keeps the validated startup contents even if mounted files change.
	manifestPath := filepath.Join(directory, "example", "v2", "manifest.yaml")
	if err := os.WriteFile(manifestPath, []byte("invalid"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := sources.Resolve("example")
	if err != nil {
		t.Fatal(err)
	}
	if second == source || second.(*Source).manifestErr != nil {
		t.Fatal("factory did not use an independent startup snapshot")
	}
	if catalog, err := LoadDirectory(directory); err == nil || catalog != nil {
		t.Fatalf("invalid reload returned %v, %v", catalog, err)
	}
}

func TestLoadDirectoryErrors(t *testing.T) {
	directory := t.TempDir()
	for _, directory := range []string{directory, filepath.Join(directory, "missing")} {
		if catalog, err := LoadDirectory(directory); err == nil || catalog != nil || !strings.Contains(err.Error(), directory) {
			t.Fatalf("directory %q: catalog=%v err=%v", directory, catalog, err)
		}
	}
}

func TestLoadDirectoryFromProjectedVolume(t *testing.T) {
	directory := t.TempDir()
	for name, file := range testCatalog(t) {
		destination := filepath.Join(directory, "..release", strings.TrimPrefix(name, "manifests/"))
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, file.Data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("..release", filepath.Join(directory, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..data/example", filepath.Join(directory, "example")); err != nil {
		t.Fatal(err)
	}
	sources, err := LoadDirectory(directory)
	if err != nil {
		t.Fatal(err)
	}
	source, err := sources.Resolve("example")
	if err != nil || source.Spec().Name != "example@v2" {
		t.Fatalf("projected catalog: %v, %v", source, err)
	}
}
