package iceberg

import (
	"fmt"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

var restCatalogProvider = catalogProvider{
	option: filament.EnumOption{Value: catalogProviderREST, Label: "REST"},
	fields: []filament.ConfigField{
		catalogStringField(catalogProviderREST, "uri", "Base URL of the Iceberg REST catalog, such as https://catalog.example.com/api/catalog.", true),
		catalogStringField(catalogProviderREST, "warehouse", "Warehouse identifier sent to the REST catalog. This may be a logical catalog name, as with Apache Polaris, rather than a storage path.", false),
		restAuthField(catalogProviderREST),
		catalogObjectField(catalogProviderREST, "Advanced REST catalog and storage properties passed directly to iceberg-go."),
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
