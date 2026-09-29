package httpapi

import (
	"fmt"
	"os"

	"github.com/galaxy-io/filament/registry"
)

// Importing this package registers every version listed in the embedded catalog.
func init() {
	entries, err := loadCatalog(catalogFS)
	if err != nil {
		panic(err)
	}
	registerCatalog(registry.DefaultSources, entries)
}

// LoadDirectory loads a private catalog from directory/<connector>/registry.json
// and directory/<connector>/<api-version>/manifest.yaml. It validates the whole
// catalog before registration and captures file contents for this process's
// lifetime. It does not mutate the bundled registry or watch for file changes.
func LoadDirectory(directory string) (*registry.Sources, error) {
	entries, err := loadCatalogRoot(os.DirFS(directory), ".")
	if err != nil {
		return nil, fmt.Errorf("HTTP manifest directory %q: %w", directory, err)
	}
	sources := registry.NewSources()
	registerCatalog(sources, entries)
	return sources, nil
}

func registerCatalog(sources *registry.Sources, entries []catalogEntry) {
	for _, entry := range entries {
		sources.RegisterWithMaturity(entry.key(), entry.maturity, entry.source)
	}
	for _, entry := range entries {
		if entry.isDefault {
			sources.RegisterAlias(entry.name, entry.key())
		}
	}
}
