package iceberg

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var polarisCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderPolaris, Label: "Apache Polaris"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderPolaris, "uri", "Polaris catalog endpoint.", true),
		catalogStringField(catalogProviderPolaris, "warehouse", "Polaris catalog name.", true),
		restAuthField(catalogProviderPolaris),
		catalogObjectField(catalogProviderPolaris, "properties", "Additional iceberg-go REST catalog properties."),
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
