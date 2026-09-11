package mysql

import (
	"github.com/galaxy-io/filament"
	mysqlconnection "github.com/galaxy-io/filament/connectors/mysql/internal/connection"
)

// Spec describes the sink's config fields and write capabilities.
func (t *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "mysql",
		DisplayName:  "MySQL",
		Description:  "Widely-used open-source relational database known for speed, reliability, and ease of use.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-mysql-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-mysql-light.svg",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: append(mysqlconnection.Fields(), []filament.ConfigField{
			{Name: "database", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Destination database. Empty defaults to the normalized source connection name."},
			{Name: "mode", Type: filament.FieldEnum, Default: "typed", Enum: []filament.EnumOption{{Value: "typed", Label: "Typed"}}, Scope: filament.ScopePipeline, Help: "Destination table mode"},
		}...)},
		SchemaField: "database",
		Capabilities: filament.SinkCapabilities{
			Schematized:        true,
			Upsertable:         true,
			EncodedIntegrity:   true,
			PreferredBatchRows: 4096,
			WritePolicies: filament.WriteCapabilities(
				filament.IngestionFullReplace,
				filament.IngestionFullAppend,
				filament.IngestionFullUpsert,
				filament.IngestionIncrementalUpsert,
				filament.IngestionCDCMerge,
				filament.IngestionCDCAppend,
			),
		},
	}
}
