package mysql

import (
	"github.com/galaxy-io/filament"
	mysqlconnection "github.com/galaxy-io/filament/connectors/mysql/internal/connection"
)

// Spec describes the source's config fields, modes, and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:         "mysql",
		DisplayName:  "MySQL",
		Description:  "Widely-used open-source relational database known for speed, reliability, and ease of use.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-mysql-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-mysql-light.svg",
		Version:      "2",
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
		Config: filament.ConfigSchema{Fields: append(mysqlconnection.Fields(), []filament.ConfigField{
			{Name: "replication", Type: filament.FieldEnum, Default: string(filament.ReplicationStandard), Enum: []filament.EnumOption{
				{Value: string(filament.ReplicationStandard), Label: "Standard"},
				{Value: string(filament.ReplicationCDC), Label: "Change Data Capture (CDC)"},
			}, Scope: filament.ScopeConnection, Help: "Standard reads tables with queries; CDC streams changes from the binary log"},
			{Name: "database", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Database to read tables from (defaults to the DSN's database)"},
			{Name: "page_size", Type: filament.FieldInt, Default: defaultPageSize, Scope: filament.ScopePipeline, Help: "Rows to target per read page"},
			{Name: "shard_pages", Type: filament.FieldInt, Default: defaultShardPages, Scope: filament.ScopePipeline, Help: "InnoDB pages per shard; 0 disables sharding"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "server_id", Type: filament.FieldInt, Default: defaultServerID, Scope: filament.ScopePipeline, Help: "Replication client server_id for CDC (must be unique in the replica topology)"},
			{Name: "snapshot_mode", Type: filament.FieldEnum, Default: string(filament.SnapshotInitial), Enum: []filament.EnumOption{
				{Value: string(filament.SnapshotInitial), Label: "Initial"},
				{Value: string(filament.SnapshotNone), Label: "None"},
			}, Scope: filament.ScopePipeline, VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{string(filament.ReplicationCDC)}}, Help: "Initial reads a table in full before streaming its changes; None streams from the current position only"},
		}...)},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}
