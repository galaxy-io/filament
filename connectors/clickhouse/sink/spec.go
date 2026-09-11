package clickhouse

import (
	"github.com/galaxy-io/filament"
	clickhouseconnection "github.com/galaxy-io/filament/connectors/clickhouse/internal/connection"
)

// Spec describes the sink's configuration and supported write policies.
func (s *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "clickhouse",
		DisplayName:  "ClickHouse",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-clickhouse-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-clickhouse-light.svg",
		Description:  "Column-oriented analytics database with typed batch loading and primary-key upserts.",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: append(clickhouseconnection.Fields(), filament.ConfigField{
			Name: "database", Type: filament.FieldString, Default: clickhouseconnection.DefaultDatabase, Scope: filament.ScopePipeline,
			Help: "Destination database. Empty defaults to the normalized source connection name.",
		})},
		SchemaField: "database",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			Upsertable:          true,
			PreferredBatchRows:  10_000,
			PreferredBatchBytes: 16 << 20,
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
