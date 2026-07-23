package iceberg

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var genericCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderGeneric, Label: "Generic"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderGeneric, "type", "iceberg-go catalog type.", false),
		catalogStringField(catalogProviderGeneric, "uri", "Catalog endpoint.", false),
		catalogStringField(catalogProviderGeneric, "warehouse", "Catalog warehouse identifier or location.", false),
		catalogObjectField(catalogProviderGeneric, "properties", "Additional iceberg-go catalog properties."),
	},
	build: buildGenericCatalog,
}

func buildGenericCatalog(cfg filament.Config) (iceberg.Properties, error) {
	properties := iceberg.Properties{}
	copyProperties(properties, cfg.Sub("properties").Raw())
	copyStringProperties(properties, cfg, map[string]string{
		"type":      "type",
		"uri":       "uri",
		"warehouse": "warehouse",
	})
	if properties["type"] == "" && properties["uri"] == "" {
		return nil, fmt.Errorf("type or uri is required")
	}
	return properties, nil
}
