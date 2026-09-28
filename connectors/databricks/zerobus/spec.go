package zerobus

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/databricks/internal/connection"
)

// Spec describes the Zerobus sink's configuration and append-only write
// capabilities. Zerobus ingests into a pre-existing Delta table and appends
// records; it does not replace, upsert, merge, or delete.
func (*Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Databricks Zerobus",
		Description:  "Stream records into Databricks Delta tables via the Zerobus Ingest API over Arrow Flight.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-databricks-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-databricks-light.svg",
		Version:      "1",
		Config:       filament.ConfigSchema{Fields: connection.Fields()},
		SchemaField:  "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			PreferredBatchRows:  50_000,
			PreferredBatchBytes: 32 << 20,
			WritePolicies:       appendWritePolicies(),
		},
	}
}

// appendWritePolicies returns the append capabilities the sink serves, marked
// durable only after Commit. Apply buffers via IngestBatch; a record is not
// durable until Commit's Flush. Advertising DurabilityAfterCommit makes the
// planner promote incremental append to CheckpointAfterCommit, so a checkpoint
// never advances past data a crash before Flush would lose.
func appendWritePolicies() []filament.WritePolicyCapability {
	caps := filament.WriteCapabilities(
		filament.IngestionFullAppend,
		filament.IngestionIncrementalAppend,
	)
	for i := range caps {
		caps[i].Durability = filament.DurabilityAfterCommit
	}
	return caps
}
