package appstoreconnect

import "github.com/galaxy-io/filament"

// Spec describes the source's configuration and ingestion policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:         "appstoreconnect",
		DisplayName:  "App Store Connect",
		Version:      "1",
		Description:  "Apple App Store Connect source for app catalog metadata, customer reviews, daily sales reports, and analytics report segments.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-apple-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-apple-light.svg",
		Modes:        []filament.ReadMode{filament.ModeFull, filament.ModeIncremental},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionFullReplace, filament.IngestionFullUpsert, filament.IngestionFullAppend,
			filament.IngestionIncrementalUpsert, filament.IngestionIncrementalAppend,
		),
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{
				Name: "issuer_id", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection,
				Help: "App Store Connect API Issuer ID (UUID from Users and Access > Integrations > App Store Connect API)",
			},
			{
				Name: "key_id", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection,
				Help: "App Store Connect 10-character API Key ID",
			},
			{
				Name: "private_key", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection,
				Help: "PEM-encoded PKCS#8 ECDSA P-256 private key (.p8 file contents)",
			},
			{
				Name: "vendor_number", Type: filament.FieldString, Required: false, Scope: filament.ScopeConnection,
				Help: "Optional Apple Vendor Number required when extracting daily sales_reports",
			},
			{
				Name: "app_ids", Type: filament.FieldString, Required: false, Scope: filament.ScopeConnection,
				Help: "Optional comma-separated App Store Connect app IDs to filter extraction",
			},
			{
				Name: "start_date", Type: filament.FieldString, Required: false, Scope: filament.ScopeConnection,
				Help: "Optional YYYY-MM-DD initial backfill start date for daily sales_reports (defaults to 30 days ago UTC)",
			},
			{
				Name: "requests_per_second", Type: filament.FieldInt, Required: false, Default: 2, Scope: filament.ScopeConnection,
				Help: "Maximum API requests per second (defaults to 2)",
			},
		}},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}
