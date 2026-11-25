package sqlserver

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/sqlserver/internal/connection"
)

// Spec describes the source's configuration, modes and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	name, display, description, logo := "sqlserver", "Microsoft SQL Server", "Microsoft's relational database, self-managed on premises or on a VM, read as full snapshots, incremental watermarks or change data capture.", "mssql"
	if s.profile == connection.AzureSQL {
		name, display, description, logo = "azuresql", "Azure SQL", "Azure SQL Database and Azure SQL Managed Instance, read as full snapshots, incremental watermarks or change data capture.", "azure"
	}
	return filament.ConnectorSpec{
		Name:         name,
		DisplayName:  display,
		Description:  description,
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-" + logo + "-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-" + logo + "-light.svg",
		Version:      "1",
		IsPreview:    true,
		Modes:        []filament.ReadMode{filament.ModeFull, filament.ModeIncremental, filament.ModeCDC},
		SourcePolicies: filament.SourcePolicies(
			filament.IngestionFullReplace,
			filament.IngestionFullUpsert,
			filament.IngestionFullAppend,
			filament.IngestionIncrementalAppend,
			filament.IngestionIncrementalUpsert,
			filament.IngestionCDCAppend,
			filament.IngestionCDCMerge,
		),
		Config: filament.ConfigSchema{Fields: append(s.profile.Fields(), []filament.ConfigField{
			{Name: "replication", Type: filament.FieldEnum, Default: replicationStandard, Enum: []filament.EnumOption{
				{Value: replicationStandard, Label: "Standard"},
				{Value: replicationCDC, Label: "Change Data Capture (CDC)"},
			}, Scope: filament.ScopeConnection, Help: "Standard reads tables with queries; CDC reads changes from the database's change tables. CDC must be enabled on the database"},
			{
				Name: "manage_capture", Type: filament.FieldBool, Default: true, Scope: filament.ScopeConnection,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Enable capture on a selected table that has no capture instance. Needs db_owner",
			},
			{
				Name: "capture_role", Type: filament.FieldString, Scope: filament.ScopeConnection,
				VisibleWhen: &filament.FieldCondition{Field: "manage_capture", Values: []string{"true"}},
				Help:        "Gating role for capture instances the source enables; empty for none",
			},
			{
				Name: "snapshot_mode", Type: filament.FieldEnum, Default: string(filament.SnapshotInitial), Enum: []filament.EnumOption{
					{Value: string(filament.SnapshotInitial), Label: "Initial"},
					{Value: string(filament.SnapshotNone), Label: "None"},
				}, Scope: filament.ScopePipeline,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Initial reads a table in full before streaming its changes; None streams from the current position only",
			},
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Schema to read tables from"},
			{Name: "shard_rows", Type: filament.FieldInt, Default: defaultShardRows, Scope: filament.ScopePipeline, Help: "Rows per shard when splitting a table across connections; 0 reads each table on one connection"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections. With CDC, also how many shards the initial snapshot reads at once (4 when unset)"},
			{Name: "isolation", Type: filament.FieldEnum, Default: isolationAuto, Enum: []filament.EnumOption{
				{Value: isolationAuto, Label: "Snapshot when allowed"},
				{Value: isolationSnapshot, Label: "Snapshot"},
				{Value: isolationReadCommitted, Label: "Read committed"},
			}, Scope: filament.ScopePipeline, Help: "Transaction isolation for table reads. Snapshot needs ALLOW_SNAPSHOT_ISOLATION on the database"},
		}...)},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}
