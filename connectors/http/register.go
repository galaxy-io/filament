package httpapi

import "github.com/galaxy-io/filament/registry"

// Importing this package registers every version listed in the embedded catalog.
func init() {
	entries, err := loadCatalog(catalogFS)
	if err != nil {
		panic(err)
	}
	for _, entry := range entries {
		registry.RegisterSource(entry.key(), entry.maturity, entry.source)
	}
	for _, entry := range entries {
		if entry.isDefault {
			registry.DefaultSources.RegisterAlias(entry.name, entry.key())
		}
	}
}
