package iceberg

import (
	"fmt"
	"slices"

	iceberg "github.com/apache/iceberg-go"

	"github.com/galaxy-io/filament"
)

const (
	catalogProviderGeneric    = "generic"
	catalogProviderREST       = "rest"
	catalogProviderPolaris    = "polaris"
	catalogProviderLakekeeper = "lakekeeper"
)

type catalogSetup struct {
	Properties        iceberg.Properties
	TableLocationRoot string
}

type catalogProvider struct {
	option filament.EnumOption
	fields []filament.ConfigField
	build  func(filament.Config) (iceberg.Properties, error)
}

// catalogProviders is intentionally explicit: it defines both supported
// providers and their stable order in the UI.
var catalogProviders = []catalogProvider{
	genericCatalogProvider,
	restCatalogProvider,
	polarisCatalogProvider,
	lakekeeperCatalogProvider,
}

func catalogConfigField() filament.ConfigField {
	options := make([]filament.EnumOption, 0, len(catalogProviders))
	fields := []filament.ConfigField{{
		Name:     "provider",
		Type:     filament.FieldEnum,
		Required: true,
		Enum:     options,
		Help:     "Catalog implementation used to manage Iceberg namespaces and tables.",
	}}

	for _, provider := range catalogProviders {
		options = append(options, provider.option)
		fields = append(fields, provider.fields...)
	}
	fields[0].Enum = options

	return filament.ConfigField{
		Name:     "catalog",
		Type:     filament.FieldObject,
		Required: true,
		Scope:    filament.ScopeConnection,
		Help:     "Iceberg catalog connection.",
		Fields:   fields,
	}
}

func tableConfigField() filament.ConfigField {
	return filament.ConfigField{
		Name:  "table",
		Type:  filament.FieldObject,
		Scope: filament.ScopeConnection,
		Help:  "Table creation settings.",
		Fields: []filament.ConfigField{{
			Name: "location_root",
			Type: filament.FieldString,
			Help: "Optional object-storage root for new tables, such as s3://bucket/warehouse. Leave empty when the catalog assigns table locations.",
		}},
	}
}

func buildCatalogSetup(cfg filament.Config) (catalogSetup, error) {
	catalogCfg := cfg.Sub("catalog")
	providerName := catalogCfg.String("provider")
	index := slices.IndexFunc(catalogProviders, func(provider catalogProvider) bool {
		return provider.option.Value == providerName
	})
	if index < 0 {
		return catalogSetup{}, fmt.Errorf("unsupported catalog provider %q", providerName)
	}

	properties, err := catalogProviders[index].build(catalogCfg)
	if err != nil {
		return catalogSetup{}, fmt.Errorf("%s catalog: %w", providerName, err)
	}

	return catalogSetup{
		Properties:        properties,
		TableLocationRoot: cfg.Sub("table").String("location_root"),
	}, nil
}

func providerCondition(provider string) *filament.FieldCondition {
	return &filament.FieldCondition{Field: "provider", Values: []string{provider}}
}

func catalogStringField(provider, name, help string, required bool) filament.ConfigField {
	return filament.ConfigField{
		Name:        name,
		Type:        filament.FieldString,
		Required:    required,
		Help:        help,
		VisibleWhen: providerCondition(provider),
	}
}

func catalogObjectField(provider, help string) filament.ConfigField {
	return filament.ConfigField{
		Name:        "properties",
		Type:        filament.FieldObject,
		Help:        help,
		VisibleWhen: providerCondition(provider),
	}
}

func copyProperties(dst iceberg.Properties, values map[string]any) {
	for key, value := range values {
		dst[key] = fmt.Sprint(value)
	}
}
