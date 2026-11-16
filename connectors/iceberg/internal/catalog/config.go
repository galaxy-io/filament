package catalog

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

// Table update modes, named after the Iceberg write.merge.mode and
// write.delete.mode values they set on new tables.
const (
	UpdateModeMergeOnRead = "merge-on-read"
	UpdateModeCopyOnWrite = "copy-on-write"
)

// Setup contains resolved Iceberg catalog properties and table creation settings.
type Setup struct {
	Properties        iceberg.Properties
	TableLocationRoot string
	TableUpdateMode   string
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

// Fields returns the connection-scoped catalog and table configuration.
func Fields() []filament.ConfigField {
	return []filament.ConfigField{catalogConfigField(), tableConfigField()}
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
		}, {
			Name:    "update_mode",
			Type:    filament.FieldEnum,
			Default: UpdateModeMergeOnRead,
			Help:    "How new tables apply updates and deletes. Use copy-on-write when a query engine cannot read equality deletes.",
			Enum: []filament.EnumOption{
				{Value: UpdateModeMergeOnRead, Label: "Merge-on-read"},
				{Value: UpdateModeCopyOnWrite, Label: "Copy-on-write"},
			},
		}},
	}
}

// Resolve translates connector configuration into catalog setup.
func Resolve(cfg filament.Config) (Setup, error) {
	catalogCfg := cfg.Sub("catalog")
	providerName := catalogCfg.String("provider")
	index := slices.IndexFunc(catalogProviders, func(provider catalogProvider) bool {
		return provider.option.Value == providerName
	})
	if index < 0 {
		return Setup{}, fmt.Errorf("unsupported catalog provider %q", providerName)
	}

	properties, err := catalogProviders[index].build(catalogCfg)
	if err != nil {
		return Setup{}, fmt.Errorf("%s catalog: %w", providerName, err)
	}

	tableCfg := cfg.Sub("table")
	updateMode := tableCfg.String("update_mode")
	switch updateMode {
	case "":
		updateMode = UpdateModeMergeOnRead
	case UpdateModeMergeOnRead, UpdateModeCopyOnWrite:
	default:
		return Setup{}, fmt.Errorf("unsupported table update mode %q", updateMode)
	}

	return Setup{
		Properties:        properties,
		TableLocationRoot: tableCfg.String("location_root"),
		TableUpdateMode:   updateMode,
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
