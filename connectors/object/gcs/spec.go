package gcs

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	object "github.com/galaxy-io/filament/connectors/object/internal"
)

// Spec describes the sink's configuration and commit-durable write modes.
func (s *Sink) Spec() filament.SinkSpec {
	jsonFormat := &filament.FieldCondition{Field: "file_format", Values: []string{
		string(encoder.FileFormatNDJSON), string(encoder.FileFormatJSONL), string(encoder.FileFormatJSON),
	}}
	parquetFormat := &filament.FieldCondition{Field: "file_format", Values: []string{string(encoder.FileFormatParquet)}}
	return filament.SinkSpec{
		Name:         "gcs",
		DisplayName:  "Google Cloud Storage",
		Description:  "Run-versioned NDJSON, JSONL, JSON, or Parquet objects in Google Cloud Storage.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-gcs-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-gcs-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "bucket", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Destination GCS bucket."},
			{Name: "auth_method", Type: filament.FieldEnum, Default: authMethodADC, Scope: filament.ScopeConnection, Help: "How Filament authenticates to GCS. ADC uses an attached service account on Google Cloud, Workload Identity on GKE, or local gcloud application-default credentials.", Enum: []filament.EnumOption{
				{Value: authMethodADC, Label: "Application Default Credentials"},
			}},
			{Name: "prefix", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Root folder for this pipeline. Each resource gets its own folder beneath it, and run manifests land in _runs. Empty defaults to the normalized source connection name."},
			{Name: "partition", Type: filament.FieldString, Default: object.DefaultPartition, Scope: filament.ScopePipeline, Help: "Folders between each resource and its files. Use {{.Date}}, {{.StartedAt}}, {{.Resource}}, and {{.Run}}. The default writes one folder per day."},
			{Name: "file_format", Type: filament.FieldEnum, Default: string(encoder.DefaultFileFormat), Scope: filament.ScopePipeline, Help: "File format used for each resource object.", Enum: []filament.EnumOption{
				{Value: string(encoder.FileFormatNDJSON), Label: "NDJSON"},
				{Value: string(encoder.FileFormatJSONL), Label: "JSONL"},
				{Value: string(encoder.FileFormatJSON), Label: "JSON"},
				{Value: string(encoder.FileFormatParquet), Label: "Parquet"},
			}},
			{Name: "compression", Type: filament.FieldEnum, Default: string(encoder.FileFormatNDJSON.DefaultCompression()), Scope: filament.ScopePipeline, VisibleWhen: jsonFormat, Help: "Compression used for JSON files.", Enum: object.CompressionOptions(encoder.FileFormatNDJSON)},
			{Name: "compression", Type: filament.FieldEnum, Default: string(encoder.FileFormatParquet.DefaultCompression()), Scope: filament.ScopePipeline, VisibleWhen: parquetFormat, Help: "Compression used for Parquet files.", Enum: object.CompressionOptions(encoder.FileFormatParquet)},
			{Name: "chunk_size_mib", Type: filament.FieldInt, Default: defaultChunkSizeMiB, Scope: filament.ScopePipeline, Help: "GCS resumable-upload chunk size in MiB; 1 through 1024. Each active resource may buffer one chunk."},
			{Name: "upload_concurrency", Type: filament.FieldInt, Default: defaultUploadWorkers, Scope: filament.ScopePipeline, Help: "Concurrent GCS upload operations across resources; 1 through 32."},
		}},
		SchemaField: "prefix",
		Capabilities: filament.SinkCapabilities{
			EncodedIntegrity: true, Schematized: true, PreferredBatchBytes: defaultChunkSizeMiB << 20,
			WritePolicies: object.CommitDurableCapabilities(filament.IngestionFullAppend, filament.IngestionCDCAppend, filament.IngestionFullReplace),
		},
	}
}
