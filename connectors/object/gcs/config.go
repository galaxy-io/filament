package gcs

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	object "github.com/galaxy-io/filament/connectors/object/internal"
)

const (
	authMethodADC           = "application_default_credentials"
	defaultChunkSizeMiB     = 16
	minChunkSizeMiB         = 1
	maxChunkSizeMiB         = 1024
	defaultUploadWorkers    = 4
	maxUploadWorkers        = 32
	maxPooledEncodedBuffer  = 4 << 20
	initialEncodedBufferCap = 64 << 10
)

type sinkConfig struct {
	bucket        string
	authMethod    string
	prefix        string
	partition     *template.Template
	chunkSize     int
	uploadWorkers int
	fileFormat    encoder.FileFormat
	compression   encoder.Compression
}

func parseConfig(cfg filament.Config) (sinkConfig, error) {
	fileFormat, err := encoder.ParseFileFormat(cfg.String("file_format"))
	if err != nil {
		return sinkConfig{}, fmt.Errorf("gcs sink: file_format: %w", err)
	}
	compressionName := cfg.String("compression")
	if compressionName == "" {
		compressionName = string(fileFormat.DefaultCompression())
	}
	compression, err := encoder.ParseCompression(compressionName)
	if err != nil {
		return sinkConfig{}, fmt.Errorf("gcs sink: compression: %w", err)
	}
	if err := (encoder.Options{FileFormat: fileFormat, Compression: compression}).Validate(); err != nil {
		return sinkConfig{}, fmt.Errorf("gcs sink: %w", err)
	}
	out := sinkConfig{
		bucket:        strings.TrimSpace(cfg.String("bucket")),
		authMethod:    strings.TrimSpace(cfg.String("auth_method")),
		prefix:        strings.Trim(cfg.String("prefix"), "/"),
		chunkSize:     defaultChunkSizeMiB << 20,
		uploadWorkers: defaultUploadWorkers,
		fileFormat:    fileFormat,
		compression:   compression,
	}
	if out.bucket == "" {
		return sinkConfig{}, fmt.Errorf("gcs sink: bucket is required")
	}
	if out.authMethod == "" {
		out.authMethod = authMethodADC
	}
	if out.authMethod != authMethodADC {
		return sinkConfig{}, fmt.Errorf("gcs sink: unsupported auth_method %q", out.authMethod)
	}
	partitionText := object.DefaultPartition
	if cfg.Has("partition") {
		partitionText = strings.Trim(cfg.String("partition"), "/")
	}
	if out.partition, err = object.ParsePartition(partitionText); err != nil {
		return sinkConfig{}, fmt.Errorf("gcs sink: partition: %w", err)
	}
	if cfg.Has("chunk_size_mib") {
		value := cfg.Int("chunk_size_mib")
		if value < minChunkSizeMiB || value > maxChunkSizeMiB {
			return sinkConfig{}, fmt.Errorf("gcs sink: chunk_size_mib must be between %d and %d", minChunkSizeMiB, maxChunkSizeMiB)
		}
		out.chunkSize = value << 20
	}
	if cfg.Has("upload_concurrency") {
		value := cfg.Int("upload_concurrency")
		if value < 1 || value > maxUploadWorkers {
			return sinkConfig{}, fmt.Errorf("gcs sink: upload_concurrency must be between 1 and %d", maxUploadWorkers)
		}
		out.uploadWorkers = value
	}
	return out, nil
}
