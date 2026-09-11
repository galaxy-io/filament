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
