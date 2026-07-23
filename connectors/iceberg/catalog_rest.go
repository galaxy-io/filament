package iceberg

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var restCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderREST, Label: "REST"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderREST, "uri", "REST catalog endpoint.", true),
		catalogStringField(catalogProviderREST, "warehouse", "Catalog warehouse identifier or location.", false),
		restAuthField(catalogProviderREST),
		catalogObjectField(catalogProviderREST, "properties", "Additional iceberg-go REST catalog properties."),
	},
	build: buildRESTCatalog,
}

func buildRESTCatalog(cfg filament.Config) (iceberg.Properties, error) {
	if cfg.String("uri") == "" {
		return nil, fmt.Errorf("uri is required")
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
