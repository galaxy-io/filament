package sqlserver

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/sqlserver/internal/connection"
)

// Spec describes the source's configuration, modes and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	name, display, description, logo := "sqlserver", "Microsoft SQL Server", "Microsoft's relational database, self-managed on premises or on a VM, read as full snapshots.", "mssql"
	if s.profile == connection.AzureSQL {
		name, display, description, logo = "azuresql", "Azure SQL", "Azure SQL Database and Azure SQL Managed Instance, read as full snapshots.", "azure"
	}
	return filament.ConnectorSpec{
		Name:         name,
		DisplayName:  display,
		Description:  description,
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-" + logo + "-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-" + logo + "-light.svg",
		Version:      "1",
		IsPreview:    true,
		Modes:        []filament.ReadMode{filament.ModeFull},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionFullReplace,
			filament.IngestionFullUpsert,
			filament.IngestionFullAppend,
		),
		Config: filament.ConfigSchema{Fields: append(s.profile.Fields(), []filament.ConfigField{
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Schema to read tables from"},
			{Name: "shard_rows", Type: filament.FieldInt, Default: defaultShardRows, Scope: filament.ScopePipeline, Help: "Rows per shard when splitting a table across connections; 0 reads each table on one connection"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "isolation", Type: filament.FieldEnum, Default: isolationAuto, Enum: []filament.EnumOption{
				{Value: isolationAuto, Label: "Snapshot when allowed"},
				{Value: isolationSnapshot, Label: "Snapshot"},
				{Value: isolationReadCommitted, Label: "Read committed"},
			}, Scope: filament.ScopePipeline, Help: "Transaction isolation for table reads. Snapshot needs ALLOW_SNAPSHOT_ISOLATION on the database"},
		}...)},
		Resources: filament.ResourceCapabilities{Discoverable: true},
	}
}
