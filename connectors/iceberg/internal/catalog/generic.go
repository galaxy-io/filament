package catalog

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var genericCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderGeneric, Label: "Generic"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderGeneric, "type", "Catalog type registered with iceberg-go, such as sql, hive, or glue. Either type or URI is required.", false),
		catalogStringField(catalogProviderGeneric, "uri", "Catalog connection URI. Its scheme may also identify the catalog type.", false),
		catalogStringField(catalogProviderGeneric, "warehouse", "Warehouse identifier passed to the catalog. Depending on the implementation, this may be a logical name or storage location.", false),
		catalogObjectField(catalogProviderGeneric, "Advanced catalog and storage properties passed directly to iceberg-go."),
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
