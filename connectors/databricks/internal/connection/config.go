// Package connection resolves and validates the Databricks connection
// configuration shared by every Databricks sink mechanism (Zerobus today).
package connection

import "github.com/galaxy-io/filament"

// Compression codecs accepted by the ipc_compression field.
const (
	CompressionNone = "none"
	CompressionLZ4  = "lz4"
	CompressionZstd = "zstd"
)

// DefaultSchema leaves the destination schema empty so the server can default
// it from the normalized source connection name.
const DefaultSchema = ""

// Fields returns the configuration schema for the Databricks connection plus
// the pipeline-scoped destination fields.
func Fields() []filament.ConfigField {
	return []filament.ConfigField{
		{Name: "workspace_url", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Databricks workspace URL, e.g. https://dbc-xxxxxxxx.cloud.databricks.com"},
		{Name: "zerobus_endpoint", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Zerobus ingest gRPC endpoint, e.g. <workspace-id>.zerobus.<region>.cloud.databricks.com"},
		{Name: "client_id", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "OAuth service-principal application ID"},
		{Name: "client_secret", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "OAuth service-principal secret"},
		{Name: "warehouse_id", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "SQL warehouse ID used to create tables when create_table is enabled"},
		{Name: "catalog", Type: filament.FieldString, Required: true, Scope: filament.ScopePipeline, Help: "Unity Catalog catalog holding the destination tables"},
		{Name: "schema", Type: filament.FieldString, Default: DefaultSchema, Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name."},
		{Name: "create_table", Type: filament.FieldBool, Default: false, Scope: filament.ScopePipeline, Help: "Create the destination Delta table if it does not exist (requires warehouse_id)."},
		{Name: "ipc_compression", Type: filament.FieldEnum, Default: CompressionNone, Enum: []filament.EnumOption{{Value: CompressionNone, Label: "None"}, {Value: CompressionLZ4, Label: "LZ4"}, {Value: CompressionZstd, Label: "Zstd"}}, Scope: filament.ScopePipeline, Help: "Arrow IPC compression codec used on the Zerobus ingest stream."},
	}
}
