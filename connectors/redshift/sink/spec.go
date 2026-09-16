package redshift

import (
	"github.com/galaxy-io/filament"
	redshiftconnection "github.com/galaxy-io/filament/connectors/redshift/internal/connection"
)

// Spec describes the sink's configuration and write capabilities.
func (*Sink) Spec() filament.SinkSpec {
	capabilities := filament.WriteCapabilities(
		filament.IngestionFullReplace,
		filament.IngestionFullAppend,
		filament.IngestionFullUpsert,
		filament.IngestionIncrementalAppend,
		filament.IngestionIncrementalUpsert,
		filament.IngestionIncrementalDelete,
		filament.IngestionCDCMerge,
		filament.IngestionCDCAppend,
	)
	for i := range capabilities {
		if capabilities[i].Mode == filament.WriteReplace {
			capabilities[i].Durability = filament.DurabilityAfterCommit
			capabilities[i].Atomicity = filament.AtomicityResource
		}
	}
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Amazon Redshift",
		Description:  "Fully managed, petabyte-scale AWS cloud data warehouse with provisioned clusters and automatically scaling Serverless workgroups.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-redshift-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-redshift-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(redshiftconnection.Fields(),
			filament.ConfigField{Name: "schema", Type: filament.FieldString, Default: redshiftconnection.DefaultSchema, Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the connection schema."},
			filament.ConfigField{Name: "staging_prefix", Type: filament.FieldString, Default: redshiftconnection.DefaultStagingPrefix, Scope: filament.ScopePipeline, Help: "Base key prefix for transient Parquet files and COPY manifests; the pipeline ID is appended automatically."},
		)},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			Upsertable:          true,
			EncodedIntegrity:    true,
			PreferredBatchRows:  100_000,
			PreferredBatchBytes: 256 << 20,
			WritePolicies:       capabilities,
		},
	}
}
