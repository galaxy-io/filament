package s3

import (
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

// Spec describes the sink's configuration and commit-durable write modes.
func (s *Sink) Spec() filament.SinkSpec {
	iamCredentials := &filament.FieldCondition{Field: "auth_method", Values: []string{authMethodIAMCredentials}}
	jsonFormat := &filament.FieldCondition{Field: "file_format", Values: []string{
		string(encoder.FileFormatNDJSON), string(encoder.FileFormatJSONL), string(encoder.FileFormatJSON),
	}}
	parquetFormat := &filament.FieldCondition{Field: "file_format", Values: []string{string(encoder.FileFormatParquet)}}
	return filament.SinkSpec{
		Name:         "s3",
		DisplayName:  "Amazon S3",
		Description:  "Run-versioned NDJSON, JSONL, JSON, or Parquet objects in Amazon S3 or an S3-compatible object store.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-s3-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-s3-light.svg",
		Version:      "3",
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "bucket", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Destination S3 bucket."},
			{Name: "prefix", Type: filament.FieldString, Scope: filament.ScopePipeline, Help: "Root folder for this pipeline. Each resource gets its own folder beneath it, and run manifests land in _runs. Empty defaults to the normalized source connection name."},
			{Name: "partition", Type: filament.FieldString, Default: defaultPartition, Scope: filament.ScopePipeline, Help: "Folders between each resource and its files. Use {{.Date}}, {{.StartedAt}}, {{.Resource}}, and {{.Run}}, for example {{.Resource}}/dt={{.Date}}. The default writes one folder per day. Leave empty to write files directly under the resource."},
			{Name: "file_format", Type: filament.FieldEnum, Default: string(encoder.DefaultFileFormat), Scope: filament.ScopePipeline, Help: "File format used for each resource object. NDJSON and JSONL contain one object per line; JSON contains one array.", Enum: []filament.EnumOption{
				{Value: string(encoder.FileFormatNDJSON), Label: "NDJSON"},
				{Value: string(encoder.FileFormatJSONL), Label: "JSONL"},
				{Value: string(encoder.FileFormatJSON), Label: "JSON"},
				{Value: string(encoder.FileFormatParquet), Label: "Parquet"},
			}},
			{Name: "compression", Type: filament.FieldEnum, Default: string(encoder.FileFormatNDJSON.DefaultCompression()), Scope: filament.ScopePipeline, VisibleWhen: jsonFormat, Help: "Compression used for JSON files.", Enum: compressionOptions(encoder.FileFormatNDJSON)},
			{Name: "compression", Type: filament.FieldEnum, Default: string(encoder.FileFormatParquet.DefaultCompression()), Scope: filament.ScopePipeline, VisibleWhen: parquetFormat, Help: "Compression used for Parquet files.", Enum: compressionOptions(encoder.FileFormatParquet)},
			{Name: "region", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "AWS region; defaults to the SDK's resolved region."},
			{Name: "endpoint", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "Custom S3 endpoint, such as MinIO; defaults to AWS."},
			{Name: "path_style", Type: filament.FieldBool, Scope: filament.ScopeConnection, Help: "Use path-style bucket addressing. Defaults to true for custom endpoints."},
			{Name: "auth_method", Type: filament.FieldEnum, Default: authMethodInstanceProfile, Scope: filament.ScopeConnection, Help: "How Filament authenticates to S3.", Enum: []filament.EnumOption{
				{Value: authMethodInstanceProfile, Label: "Instance Profile"},
				{Value: authMethodIAMCredentials, Label: "IAM Credentials"},
			}},
			{Name: "access_key_id", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: iamCredentials, Help: "AWS access key ID."},
			{Name: "secret_access_key", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, VisibleWhen: iamCredentials, Help: "AWS secret access key."},
			{Name: "session_token", Type: filament.FieldSecret, Scope: filament.ScopeConnection, VisibleWhen: iamCredentials, Help: "Optional AWS session token for temporary credentials."},
			{Name: "part_size_mib", Type: filament.FieldInt, Default: defaultPartSizeMiB, Scope: filament.ScopePipeline, Help: "Multipart request size in MiB; 5 through 5120. Also sets the 10,000-part object-size ceiling; buffers allocate lazily."},
			{Name: "upload_concurrency", Type: filament.FieldInt, Default: defaultUploadWorkers, Scope: filament.ScopePipeline, Help: "Concurrent S3 upload operations across resources; 1 through 32."},
		}},
		SchemaField: "prefix",
		Capabilities: filament.SinkCapabilities{
			EncodedIntegrity: true,
			Schematized:      true,
			WritePolicies: commitDurableCapabilities(
				filament.IngestionFullAppend,
				filament.IngestionCDCAppend,
				filament.IngestionFullReplace,
			),
		},
	}
}

func compressionOptions(format encoder.FileFormat) []filament.EnumOption {
	compressions := format.SupportedCompressions()
	options := make([]filament.EnumOption, len(compressions))
	for i, compression := range compressions {
		options[i] = filament.EnumOption{Value: string(compression), Label: compressionLabel(compression)}
	}
	return options
}

func compressionLabel(compression encoder.Compression) string {
	switch compression {
	case encoder.CompressionNone:
		return "None"
	case encoder.CompressionGZIP:
		return "Gzip"
	case encoder.CompressionSnappy:
		return "Snappy"
	default:
		return string(compression)
	}
}

func commitDurableCapabilities(types ...filament.IngestionType) []filament.WritePolicyCapability {
	capabilities := filament.WriteCapabilities(types...)
	for i := range capabilities {
		capabilities[i].Durability = filament.DurabilityAfterCommit
		capabilities[i].Atomicity = filament.AtomicityResource
	}
	return capabilities
}
