package s3

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

const (
	authMethodInstanceProfile = "instance_profile"
	authMethodIAMCredentials  = "iam_credentials"
	minPartSizeMiB            = 5
	maxPartSizeMiB            = 5 * 1024
	defaultPartSizeMiB        = 16
	defaultUploadWorkers      = 4
	maxUploadWorkers          = 32
)

type sinkConfig struct {
	bucket          string
	prefix          string
	partition       *template.Template
	region          string
	endpoint        string
	authMethod      string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	pathStyle       bool
	partSize        int64
	uploadWorkers   int
	fileFormat      encoder.FileFormat
	compression     encoder.Compression
}

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
			{Name: "region", Type: filament.FieldString, Scope: filament.ScopeConnection, Help: "AWS region; defaults to the SDK's resolved region."},
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

// Validate performs pure connector-specific validation and no network I/O.
func (s *Sink) Validate(cfg filament.Config) error {
	_, err := parseConfig(cfg)
	return err
}

func parseConfig(cfg filament.Config) (sinkConfig, error) {
	formatName := cfg.String("file_format")
	compressionName := cfg.String("compression")
	if formatName == "" {
		legacyName := cfg.String("encoding")
		if legacyName == "" {
			legacyName = cfg.String("file_type")
		}
		switch strings.ToLower(strings.TrimSpace(legacyName)) {
		case "json_gzip", "gzip_json", "ndjson_gzip", "jsonl_gzip", "json.gz", "ndjson.gz":
			formatName = string(encoder.FileFormatNDJSON)
			if !cfg.Has("compression") {
				compressionName = string(encoder.CompressionGZIP)
			}
		case "json", "jsonl":
			// The temporary combined encoding option treated these as NDJSON.
			// Preserve that behavior for stored legacy configurations.
			formatName = string(encoder.FileFormatNDJSON)
		default:
			formatName = legacyName
		}
	}
	fileFormat, err := encoder.ParseFileFormat(formatName)
	if err != nil {
		return sinkConfig{}, fmt.Errorf("s3 sink: file_format: %w", err)
	}
	if compressionName == "" {
		compressionName = string(fileFormat.DefaultCompression())
	}
	compression, err := encoder.ParseCompression(compressionName)
	if err != nil {
		return sinkConfig{}, fmt.Errorf("s3 sink: compression: %w", err)
	}
	if err := (encoder.Options{FileFormat: fileFormat, Compression: compression}).Validate(); err != nil {
		return sinkConfig{}, fmt.Errorf("s3 sink: %w", err)
	}
	out := sinkConfig{
		bucket:          strings.TrimSpace(cfg.String("bucket")),
		prefix:          strings.Trim(cfg.String("prefix"), "/"),
		region:          strings.TrimSpace(cfg.String("region")),
		endpoint:        strings.TrimSpace(cfg.String("endpoint")),
		authMethod:      strings.TrimSpace(cfg.String("auth_method")),
		accessKeyID:     cfg.Secret("access_key_id"),
		secretAccessKey: cfg.Secret("secret_access_key"),
		sessionToken:    cfg.Secret("session_token"),
		partSize:        defaultPartSizeMiB << 20,
		uploadWorkers:   defaultUploadWorkers,
		fileFormat:      fileFormat,
		compression:     compression,
	}
	if out.bucket == "" {
		return sinkConfig{}, fmt.Errorf("s3 sink: bucket is required")
	}
	partitionText := defaultPartition
	if cfg.Has("partition") {
		partitionText = strings.Trim(cfg.String("partition"), "/")
	}
	if out.partition, err = parsePartition(partitionText); err != nil {
		return sinkConfig{}, fmt.Errorf("s3 sink: partition: %w", err)
	}
	if out.authMethod == "" {
		out.authMethod = authMethodInstanceProfile
	}
	credentialsPresent := out.accessKeyID != "" || out.secretAccessKey != "" || out.sessionToken != ""
	switch out.authMethod {
	case authMethodInstanceProfile:
		if credentialsPresent {
			return sinkConfig{}, fmt.Errorf("s3 sink: IAM credential fields require auth_method %q", authMethodIAMCredentials)
		}
	case authMethodIAMCredentials:
		if out.accessKeyID == "" || out.secretAccessKey == "" {
			return sinkConfig{}, fmt.Errorf("s3 sink: access_key_id and secret_access_key are required for auth_method %q", authMethodIAMCredentials)
		}
	default:
		return sinkConfig{}, fmt.Errorf("s3 sink: unsupported auth_method %q", out.authMethod)
	}
	if cfg.Has("part_size_mib") {
		value := cfg.Int("part_size_mib")
		if value < minPartSizeMiB || value > maxPartSizeMiB {
			return sinkConfig{}, fmt.Errorf("s3 sink: part_size_mib must be between %d and %d", minPartSizeMiB, maxPartSizeMiB)
		}
		out.partSize = int64(value) << 20
	}
	if cfg.Has("upload_concurrency") {
		value := cfg.Int("upload_concurrency")
		if value < 1 || value > maxUploadWorkers {
			return sinkConfig{}, fmt.Errorf("s3 sink: upload_concurrency must be between 1 and %d", maxUploadWorkers)
		}
		out.uploadWorkers = value
	}
	out.pathStyle = out.endpoint != ""
	if cfg.Has("path_style") {
		out.pathStyle = cfg.Bool("path_style")
	}
	return out, nil
}
