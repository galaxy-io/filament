package postgres

import (
	"github.com/galaxy-io/filament"
	pgconnection "github.com/galaxy-io/filament/connectors/postgres/internal/connection"
)

// Spec describes the source's config fields, modes, and write policies.
func (s *Source) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{
		Name:         "postgres",
		DisplayName:  "PostgreSQL",
		Description:  "Popular open-source relational database management system known for reliability and advanced features.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-postgres-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-postgres-light.svg",
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
		Config: filament.ConfigSchema{Fields: append(pgconnection.Fields(), []filament.ConfigField{
			{Name: "replication", Type: filament.FieldEnum, Default: replicationStandard, Enum: []filament.EnumOption{
				{Value: replicationStandard, Label: "Standard"},
				{Value: replicationCDC, Label: "Change Data Capture (CDC)"},
			}, Scope: filament.ScopeConnection, Help: "Standard reads tables with queries; CDC streams changes from the write-ahead log"},
			{Name: "schema", Type: filament.FieldString, Default: defaultSchema, Scope: filament.ScopePipeline, Help: "Schema to read tables from"},
			{Name: "page_size", Type: filament.FieldInt, Default: defaultPageSize, Scope: filament.ScopePipeline, Help: "Rows to target per read page"},
			{Name: "shard_pages", Type: filament.FieldInt, Default: defaultShardPages, Scope: filament.ScopePipeline, Help: "Heap blocks per shard; 0 disables sharding"},
			{Name: "max_conns", Type: filament.FieldInt, Scope: filament.ScopePipeline, Help: "Maximum source database connections"},
			{Name: "scan_strategy", Type: filament.FieldEnum, Default: "keyset", Enum: []filament.EnumOption{
				{Value: "auto", Label: "Auto"},
				{Value: "keyset", Label: "Keyset"},
				{Value: "bitmap", Label: "Bitmap"},
				{Value: "ctid", Label: "CTID + xmin"},
			}, Scope: filament.ScopePipeline, Help: "Read strategy"},
			{
				Name: "publication", Type: filament.FieldString, Default: defaultPublication, Scope: filament.ScopeConnection,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Logical replication publication used by CDC",
			},
			{
				Name: "manage_publication", Type: filament.FieldBool, Default: true, Scope: filament.ScopeConnection,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Create the CDC publication and add selected tables when needed",
			},
			{
				Name: "snapshot_mode", Type: filament.FieldEnum, Default: string(filament.SnapshotInitial), Enum: []filament.EnumOption{
					{Value: string(filament.SnapshotInitial), Label: "Initial"},
					{Value: string(filament.SnapshotNone), Label: "None"},
				}, Scope: filament.ScopePipeline,
				VisibleWhen: &filament.FieldCondition{Field: "replication", Values: []string{replicationCDC}},
				Help:        "Initial reads a table in full before streaming its changes; None streams from the current position only",
			},
		}...)},
		Resources: filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}
