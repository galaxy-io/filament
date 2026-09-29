package connectors

import (
	"fmt"
	"os"

	httpapi "github.com/galaxy-io/filament/connectors/http"
	"github.com/galaxy-io/filament/registry"
)

// SourcesFromEnv adds HTTP_MANIFESTS_DIR to the bundled catalog. Private versions
// and default aliases take precedence without changing the process-wide registry.
func SourcesFromEnv() (*registry.Sources, error) {
	directory := os.Getenv("HTTP_MANIFESTS_DIR")
	if directory == "" {
		return registry.DefaultSources, nil
	}
	private, err := httpapi.LoadDirectory(directory)
	if err != nil {
		return nil, fmt.Errorf("HTTP_MANIFESTS_DIR: %w", err)
	}
	return registry.DefaultSources.WithOverrides(private)
}
