package motherduck

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/motherduck/internal/connection"
)

// Spec describes the sink's configuration and supported write policies.
func (s *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "MotherDuck",
		Description:  "Serverless DuckDB with typed Arrow batch loading.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-motherduck-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-motherduck-light.svg",
		Version:      "1",
		Maturity:     filament.MaturityAlpha,
		Config: filament.ConfigSchema{Fields: append(connection.Fields(), filament.ConfigField{
			Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline,
			Help: "Destination schema.",
		})},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			PreferredBatchRows:  100_000,
			PreferredBatchBytes: 64 << 20,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDCAppend,
			),
		},
	}
}
