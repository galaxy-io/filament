package catalog

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var lakekeeperCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderLakekeeper, Label: "Lakekeeper"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderLakekeeper, "uri", "Lakekeeper Iceberg REST endpoint, typically ending in /catalog.", true),
		catalogStringField(catalogProviderLakekeeper, "warehouse", "Name of the warehouse registered in Lakekeeper.", true),
		restAuthField(catalogProviderLakekeeper),
		catalogObjectField(catalogProviderLakekeeper, "Advanced REST, credential-vending, and storage properties passed directly to iceberg-go."),
	},
	build: buildLakekeeperCatalog,
}

func buildLakekeeperCatalog(cfg filament.Config) (iceberg.Properties, error) {
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
