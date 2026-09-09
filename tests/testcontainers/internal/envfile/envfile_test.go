package envfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImagesFromNestedModule(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"docker", "tests/integration/example"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for path, body := range map[string]string{
		"docker-compose.yaml": "services: {}",
		"tests/go.mod":        "module example.com/tests",
		"docker/.env":         "POSTGRES_IMAGE=example.com/postgres:file\nNATS_IMAGE=example.com/nats:file\n",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(filepath.Join(root, "tests/integration/example"))
	t.Setenv("POSTGRES_IMAGE", "example.com/postgres:environment")
	t.Setenv("NATS_IMAGE", "")
	images, err := Images()
	if err != nil {
		t.Fatal(err)
	}
	if images["POSTGRES_IMAGE"] != "example.com/postgres:environment" || images["NATS_IMAGE"] != "example.com/nats:file" {
		t.Fatalf("wrong override precedence: %v", images)
	}
}

func TestDefaultsOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	defaults, err := parse(strings.NewReader(imageDefaults))
	if err != nil {
		t.Fatal(err)
	}
	for key := range defaults {
		t.Setenv(key, "")
	}
	images, err := Images()
	if err != nil {
		t.Fatal(err)
	}
	for key, image := range defaults {
		registry, _, ok := strings.Cut(image, "/")
		if !ok || !strings.Contains(registry, ".") {
			t.Errorf("%s is not fully qualified: %s", key, image)
		}
		if images[key] != image {
			t.Errorf("%s: got %s, want embedded default %s", key, images[key], image)
		}
	}
}
