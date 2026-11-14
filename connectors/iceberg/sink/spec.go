package iceberg

import (
	"time"

	"github.com/galaxy-io/filament"
	icebergcatalog "github.com/galaxy-io/filament/connectors/iceberg/internal/catalog"
)

// Spec reports the sink's capabilities and configuration surface.
func (s *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "iceberg",
		DisplayName:  "Apache Iceberg",
		Description:  "Open table format for large analytic datasets that lets multiple processing engines share tables, with schema evolution and reproducible snapshot queries.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-iceberg-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-iceberg-light.svg",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: append(icebergcatalog.Fields(), []filament.ConfigField{
			{Name: "namespace", Type: filament.FieldString, Default: defaultNamespace, Scope: filament.ScopePipeline, Help: "Destination namespace (database) for this pipeline's tables. Empty defaults to the normalized source connection name."},
			{Name: "stage_buffer_limit_mb", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Staging buffer flush threshold in MiB."},
			{Name: "maintenance", Type: filament.FieldBool, Default: true, Scope: filament.ScopeConnection, Help: "Compact data files, merge manifests and expire snapshots after commits. Turn off when the catalog maintains tables itself."},
		}...)},
		SchemaField: "namespace",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			PreferredBatchBytes: defaultStageBufLimitBytes,
			// Every epoch is a table snapshot, so epochs run longer than the
			// fresh-session default and fit the stage buffer without spilling.
			PreferredEpochBoundary: &filament.Boundary{
				MaxRecords: 1_000_000,
				MaxBytes:   defaultStageBufLimitBytes,
				MaxWait:    time.Minute,
				MaxAge:     time.Minute,
			},
			WritePolicies: commitDurableCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionIncrementalDelete,
				filament.IngestionCDCMerge,
				filament.IngestionCDCAppend,
			),
		},
	}
}

func commitDurableCapabilities(types ...filament.IngestionType) []filament.WritePolicyCapability {
	capabilities := filament.WriteCapabilities(types...)
	for i := range capabilities {
		capabilities[i].Durability = filament.DurabilityAfterCommit
		capabilities[i].Atomicity = filament.AtomicityResource
	}
	return capabilities
}
