package postgres

import (
	"github.com/galaxy-io/filament"
	pgconnection "github.com/galaxy-io/filament/connectors/postgres/internal/connection"
)

// Spec describes the sink's config fields and write capabilities.
func (t *Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         "postgres",
		DisplayName:  "PostgreSQL",
		Description:  "Popular open-source relational database management system known for reliability and advanced features.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-postgres-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-postgres-light.svg",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: append(pgconnection.Fields(), []filament.ConfigField{
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name."},
			{Name: "mode", Type: filament.FieldEnum, Default: "typed", Enum: []filament.EnumOption{{Value: "typed", Label: "Typed"}}, Scope: filament.ScopePipeline, Help: "Destination table mode"},
		}...)},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:      true,
			Upsertable:       true,
			EncodedIntegrity: true,
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
