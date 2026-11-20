package iceberg

import (
	"fmt"
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
			{Name: "commit_interval", Type: filament.FieldDuration, Default: "1m", Scope: filament.ScopePipeline, Help: "How often a continuous run commits to the table"},
		}...)},
		SchemaField: "namespace",
		Capabilities: filament.SinkCapabilities{
			Schematized:            true,
			PreferredBatchRows:     100_000,
			PreferredBatchBytes:    defaultStageBufLimitBytes,
			PreferredEpochBoundary: new(commitBoundary(defaultCommitInterval, defaultStageBufLimitBytes)),
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

// EpochBoundary sets a continuous run's commit cadence from commit_interval
// and stage_buffer_limit_mb.
func (s *Sink) EpochBoundary(cfg filament.Config) (filament.Boundary, error) {
	bufLimit := int64(defaultStageBufLimitBytes)
	if mb := cfg.Int("stage_buffer_limit_mb"); mb > 0 {
		bufLimit = int64(mb) << 20
	}
	if v := cfg.Raw()["commit_interval"]; v == nil || v == "" {
		return commitBoundary(defaultCommitInterval, bufLimit), nil
	}
	interval := cfg.Duration("commit_interval")
	if interval <= 0 {
		return filament.Boundary{}, fmt.Errorf("iceberg sink: commit_interval must be a positive duration")
	}
	return commitBoundary(interval, bufLimit), nil
}

// commitBoundary ends an epoch, and so a table snapshot, at the interval or
// when the stage buffer fills.
func commitBoundary(interval time.Duration, bufLimit int64) filament.Boundary {
	return filament.Boundary{
		MaxRecords: 1_000_000,
		MaxBytes:   bufLimit,
		MaxWait:    interval,
		MaxAge:     interval,
	}
}
