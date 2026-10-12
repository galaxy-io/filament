package connectors

import (
	"fmt"
	"os"
	"strings"

	httpapi "github.com/galaxy-io/filament/connectors/http"
	"github.com/galaxy-io/filament/registry"
)

// SourcesFromEnv adds HTTP_MANIFESTS_DIR to the bundled catalog, then keeps
// only the sources CONNECTOR_SOURCES names when it is set. Private versions
// may be added and default aliases may change, but concrete bundled
// registrations cannot be replaced. The process-wide registry is unchanged.
func SourcesFromEnv() (*registry.Sources, error) {
	sources := registry.DefaultSources
	if directory := os.Getenv("HTTP_MANIFESTS_DIR"); directory != "" {
		private, err := httpapi.LoadDirectory(directory)
		if err != nil {
			return nil, fmt.Errorf("HTTP_MANIFESTS_DIR: %w", err)
		}
		if sources, err = sources.WithOverrides(private); err != nil {
			return nil, fmt.Errorf("HTTP_MANIFESTS_DIR: %w", err)
		}
	}
	names, err := connectorNames("CONNECTOR_SOURCES")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return sources, nil
	}
	only, err := sources.Only(names)
	if err != nil {
		return nil, fmt.Errorf("CONNECTOR_SOURCES: %w", err)
	}
	return only, nil
}

// SinksFromEnv keeps only the sinks CONNECTOR_SINKS names when it is set.
// The process-wide registry is unchanged.
func SinksFromEnv() (*registry.Sinks, error) {
	names, err := connectorNames("CONNECTOR_SINKS")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return registry.DefaultSinks, nil
	}
	only, err := registry.DefaultSinks.Only(names)
	if err != nil {
		return nil, fmt.Errorf("CONNECTOR_SINKS: %w", err)
	}
	return only, nil
}

// connectorNames splits a comma-separated list of connector names. A blank
// value is no list. A value that names nothing is an error, so a mistyped
// allowlist never falls back to every connector.
func connectorNames(key string) ([]string, error) {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var names []string
	for name := range strings.SplitSeq(value, ",") {
		if name = strings.TrimSpace(name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("%s: %q names no connectors", key, value)
	}
	return names, nil
}
