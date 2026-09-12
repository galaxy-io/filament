package bigquery

import (
	"github.com/galaxy-io/filament"
	bigqueryconnection "github.com/galaxy-io/filament/connectors/bigquery/internal/connection"
)

// Spec describes the sink's connection configuration and write capabilities.
func (*Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Google BigQuery",
		Description:  "Serverless Google Cloud data warehouse for large-scale analytics.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-bigquery-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-bigquery-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(bigqueryconnection.Fields(), filament.ConfigField{
			Name: "dataset", Type: filament.FieldString, Scope: filament.ScopePipeline,
			Help: "Destination dataset. Empty defaults to the normalized source connection name.",
		})},
		SchemaField: "dataset",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			Upsertable:          true,
			EncodedIntegrity:    true,
			PreferredBatchRows:  100_000,
			PreferredBatchBytes: 256 << 20,
			WritePolicies: bigQueryWriteCapabilities(
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

func bigQueryWriteCapabilities(types ...filament.IngestionType) []filament.WritePolicyCapability {
	capabilities := filament.WriteCapabilities(types...)
	for i := range capabilities {
		if capabilities[i].Mode == filament.WriteReplace {
			capabilities[i].Durability = filament.DurabilityAfterCommit
			capabilities[i].Atomicity = filament.AtomicityResource
		}
	}
	return capabilities
}
