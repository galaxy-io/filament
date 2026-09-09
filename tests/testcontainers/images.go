//go:build integration || e2e

// Package testcontainers provisions backing services for integration and e2e
// tests. Exclusive containers use t.Cleanup; shared containers require Ryuk.
package testcontainers

import (
	"sync"
	"testing"

	"github.com/galaxy-io/filament/tests/testcontainers/internal/envfile"
)

var loadImages = sync.OnceValues(envfile.Images)

// Image returns a shared image pin with repository and environment overrides.
func Image(t testing.TB, key string) string {
	t.Helper()
	images, err := loadImages()
	if err != nil {
		t.Fatalf("load container images: %v", err)
	}
	if image := images[key]; image != "" {
		return image
	}
	t.Fatalf("no image for %q", key)
	return ""
}
