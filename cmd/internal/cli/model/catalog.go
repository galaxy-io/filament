package model

import (
	"fmt"
	"sort"

	"github.com/galaxy-io/filament"
)

// Catalog contains connector metadata supplied by the selected target.
type Catalog struct {
	Sources map[string]filament.ConnectorSpec
	Sinks   map[string]filament.SinkSpec
}

// SourceNames returns registered source connector names in stable order.
func (c Catalog) SourceNames() []string { return sortedCatalogKeys(c.Sources) }

// SinkNames returns registered sink connector names in stable order.
func (c Catalog) SinkNames() []string { return sortedCatalogKeys(c.Sinks) }

// Description returns connector help text for a source or sink connector.
func (c Catalog) Description(kind, connector string) (string, bool) {
	if kind == "sink" {
		spec, ok := c.Sinks[connector]
		return spec.Description, ok
	}
	spec, ok := c.Sources[connector]
	return spec.Description, ok
}

// ConnectionSchema returns a connector's connection configuration schema.
func (c Catalog) ConnectionSchema(kind, connector string) (filament.ConfigSchema, error) {
	if kind == "source" {
		spec, ok := c.Sources[connector]
		if !ok {
			return filament.ConfigSchema{}, fmt.Errorf("unknown source connector %q", connector)
		}
		return spec.Config, nil
	}
	spec, ok := c.Sinks[connector]
	if !ok {
		return filament.ConfigSchema{}, fmt.Errorf("unknown sink connector %q", connector)
	}
	return spec.Config, nil
}

func sortedCatalogKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
