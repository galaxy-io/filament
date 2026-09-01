package main

import (
	"github.com/galaxy-io/filament"
	climodel "github.com/galaxy-io/filament/cmd/internal/cli/model"
	"github.com/galaxy-io/filament/registry"
)

func loadCatalog() climodel.Catalog {
	catalog := climodel.Catalog{
		Sources: map[string]filament.ConnectorSpec{},
		Sinks:   map[string]filament.SinkSpec{},
	}
	for _, spec := range registry.DefaultSources.Specs() {
		catalog.Sources[spec.Name] = spec
	}
	for _, spec := range registry.DefaultSinks.Specs() {
		catalog.Sinks[spec.Name] = spec
	}
	return catalog
}
