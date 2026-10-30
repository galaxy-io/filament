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
		Description:  "Open source object-relational database that extends SQL to manage complex data workloads, with an emphasis on extensibility and data integrity.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-postgres-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-postgres-light.svg",
		Version:      "2",
		Config: filament.ConfigSchema{Fields: append(pgconnection.Fields(), []filament.ConfigField{
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name."},
			{Name: "schema_evolution", Type: filament.FieldEnum, Default: "disabled", Scope: filament.ScopePipeline, Enum: []filament.EnumOption{{Value: "disabled", Label: "Disabled"}, {Value: "strict", Label: "Block schema changes"}, {Value: "nullable_add", Label: "Allow backward compatible additions"}}, Help: "Compare complete source schemas with destination tables before writing. Other changes block the run."},
			{Name: "mode", Type: filament.FieldEnum, Default: "typed", Enum: []filament.EnumOption{{Value: "typed", Label: "Typed"}}, Scope: filament.ScopePipeline, Help: "Destination table mode"},
		}...)},
		SchemaField: "schema",
		Capabilities: filament.SinkCapabilities{
			Schematized:         true,
			ReusableSession:     true,
			EncodedIntegrity:    true,
			PreferredBatchBytes: 16 << 20,
			WritePolicies:       postgresWriteCapabilities(),
		},
	}
}

func postgresWriteCapabilities() []filament.WritePolicyCapability {
	policies := filament.WriteCapabilities(filament.IngestionFullReplace, filament.IngestionFullAppend, filament.IngestionFullUpsert, filament.IngestionIncrementalUpsert, filament.IngestionCDCMerge, filament.IngestionCDCAppend)
	// Batch-durable policies stay first for bounded planning. Continuous planning
	// prefers these transaction-durable alternatives and also accepts saved policies.
	for _, p := range policies {
		if p.Mode == filament.WriteReplace {
			continue
		}
		p.Durability = filament.DurabilityAfterCommit
		policies = append(policies, p)
	}
	return policies
}
