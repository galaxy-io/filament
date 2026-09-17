package hubspot

import "github.com/galaxy-io/filament"

// Spec describes the source's configuration and ingestion policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name: "hubspot", DisplayName: "HubSpot", Version: "1",
		Description:  "Read HubSpot CRM records, activities, owners, and custom objects.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-hubspot-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-hubspot-light.svg",
		Modes:        []filament.ReadMode{filament.ModeFull, filament.ModeIncremental},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionFullReplace, filament.IngestionFullUpsert, filament.IngestionFullAppend,
			filament.IngestionIncrementalUpsert, filament.IngestionIncrementalAppend,
		),
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{
				Name: "api_key", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection,
				Help: "HubSpot service key or static access token with read access to the selected objects and their properties",
			},
		}},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}
