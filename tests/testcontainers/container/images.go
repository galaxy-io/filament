// Package container provides legacy helpers for long-lived backing services.
// These helpers are not used by the current Filament CLI.
package container

import (
	"fmt"
	"sync"

	"github.com/galaxy-io/filament/tests/testcontainers/internal/envfile"
)

var loadImages = sync.OnceValues(envfile.Images)

// Image returns a shared image pin with repository and environment overrides.
func Image(key string) (string, error) {
	images, err := loadImages()
	if err != nil {
		return "", err
	}
	if image := images[key]; image != "" {
		return image, nil
	}
	return "", fmt.Errorf("no image for %q — pass --image explicitly", key)
}
