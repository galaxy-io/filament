package iceberg

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var polarisCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderPolaris, Label: "Apache Polaris"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderPolaris, "uri", "Polaris Iceberg REST endpoint, typically ending in /api/catalog.", true),
		catalogStringField(catalogProviderPolaris, "warehouse", "Name of the catalog registered in Polaris. This is not an object-storage path.", true),
		restAuthField(catalogProviderPolaris),
		catalogObjectField(catalogProviderPolaris, "Advanced REST, credential-vending, and storage properties passed directly to iceberg-go."),
	},
	build: buildPolarisCatalog,
}

func buildPolarisCatalog(cfg filament.Config) (iceberg.Properties, error) {
	if cfg.String("uri") == "" {
		return nil, fmt.Errorf("uri is required")
	}
	if cfg.String("warehouse") == "" {
		return nil, fmt.Errorf("warehouse is required")
	}

	properties := iceberg.Properties{"type": "rest"}
	copyProperties(properties, cfg.Sub("properties").Raw())
	copyStringProperties(properties, cfg, map[string]string{
		"uri":       "uri",
		"warehouse": "warehouse",
	})
	if err := applyRESTAuth(properties, cfg); err != nil {
		return nil, err
	}
	return properties, nil
}
