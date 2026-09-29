package connectors

import (
	"fmt"
	"os"

	httpapi "github.com/galaxy-io/filament/connectors/http"
	"github.com/galaxy-io/filament/registry"
)

// SourcesFromEnv adds HTTP_MANIFESTS_DIR to the bundled catalog. Private versions
// may be added and default aliases may change, but concrete bundled registrations
// cannot be replaced. The process-wide registry is unchanged.
func SourcesFromEnv() (*registry.Sources, error) {
	directory := os.Getenv("HTTP_MANIFESTS_DIR")
	if directory == "" {
		return registry.DefaultSources, nil
	}
	private, err := httpapi.LoadDirectory(directory)
	if err != nil {
		return nil, fmt.Errorf("HTTP_MANIFESTS_DIR: %w", err)
	}
	sources, err := registry.DefaultSources.WithOverrides(private)
	if err != nil {
		return nil, fmt.Errorf("HTTP_MANIFESTS_DIR: %w", err)
	}
	return sources, nil
}
