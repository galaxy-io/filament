package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/galaxy-io/filament"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// catalogPath caches the local engine's connector catalog, so help that runs
// before an engine starts still lists connectors and their flags.
func (a *cliApp) catalogPath() string { return filepath.Join(a.stateDir(), "catalog.json") }

// loadCatalog reads the cached catalog, or returns an empty one.
func (a *cliApp) loadCatalog() climodel.Catalog {
	catalog := climodel.Catalog{
		Sources: map[string]filament.ConnectorSpec{},
		Sinks:   map[string]filament.SinkSpec{},
	}
	if data, err := os.ReadFile(a.catalogPath()); err == nil {
		_ = json.Unmarshal(data, &catalog)
	}
	return catalog
}

// saveCatalog refreshes the cache. It is best effort: a stale cache only
// affects help.
func (a *cliApp) saveCatalog(catalog climodel.Catalog) {
	if data, err := json.Marshal(catalog); err == nil {
		_ = os.WriteFile(a.catalogPath(), data, 0o600)
	}
}
