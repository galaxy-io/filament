package snowflake

import (
	"github.com/galaxy-io/filament"
	snowflakeconnection "github.com/galaxy-io/filament/connectors/snowflake/internal/connection"
)

// Spec describes the sink's connection configuration and write capabilities.
func (*Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Snowflake",
		Description:  "Cloud-native data platform for scalable analytics, elastic compute, and secure data sharing.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-snowflake-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-snowflake-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(snowflakeconnection.Fields(), filament.ConfigField{
			Name: "schema", Type: filament.FieldString, Default: snowflakeconnection.DefaultSchema,
			Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name.",
		})},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			EncodedIntegrity:   true,
			PreferredBatchRows: 100_000,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalAppend,
				filament.IngestionIncrementalUpsert,
				filament.IngestionIncrementalDelete,
				filament.IngestionCDCMerge,
				filament.IngestionCDCAppend,
			),
		},
	}
}
