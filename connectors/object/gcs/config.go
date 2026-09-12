package gcs

import (
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

const (
	authMethodADC           = "application_default_credentials"
	defaultPartition        = "dt={{.Date}}"
	defaultChunkSizeMiB     = 16
	minChunkSizeMiB         = 1
	maxChunkSizeMiB         = 1024
	defaultUploadWorkers    = 4
	maxUploadWorkers        = 32
	runsDir                 = "_runs"
	successMarker           = "_SUCCESS.json"
	manifestContentType     = "application/json"
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
	partitionText := defaultPartition
	if cfg.Has("partition") {
		partitionText = strings.Trim(cfg.String("partition"), "/")
	}
	if out.partition, err = parsePartition(partitionText); err != nil {
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

type partitionData struct {
	Resource  string
	Run       string
	Date      string
	StartedAt time.Time
}

func newPartitionData(resource string, run filament.RunID, startedAt time.Time) partitionData {
	startedAt = startedAt.UTC()
	return partitionData{Resource: resource, Run: string(run), Date: startedAt.Format(time.DateOnly), StartedAt: startedAt}
}

func parsePartition(text string) (*template.Template, error) {
	tmpl, err := template.New("partition").Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	if _, err := renderPartition(tmpl, newPartitionData("resource", "run", time.Unix(0, 0))); err != nil {
		return nil, err
	}
	return tmpl, nil
}

func renderPartition(tmpl *template.Template, data partitionData) (string, error) {
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return "", err
	}
	rendered := out.String()
	if rendered == "" {
		return "", nil
	}
	for _, segment := range strings.Split(rendered, "/") {
		switch segment {
		case "", ".", "..":
			return "", fmt.Errorf("partition renders invalid path %q", rendered)
		}
	}
	return rendered, nil
}

type keyLayout struct {
	prefix    string
	run       filament.RunID
	startedAt time.Time
	partition *template.Template
	extension string
}

func newKeyLayout(cfg sinkConfig, run filament.RunSpec) keyLayout {
	startedAt := run.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now()
	}
	options := encoder.Options{FileFormat: cfg.fileFormat, Compression: cfg.compression}
	return keyLayout{prefix: cfg.prefix, run: run.Run, startedAt: startedAt, partition: cfg.partition, extension: options.Extension()}
}

func (l keyLayout) object(resource string) (string, error) {
	partition, err := renderPartition(l.partition, newPartitionData(resource, l.run, l.startedAt))
	if err != nil {
		return "", fmt.Errorf("gcs sink: %w", err)
	}
	if partition == "" {
		return l.join(resource, string(l.run)+l.extension), nil
	}
	return l.join(resource, partition, string(l.run)+l.extension), nil
}

func (l keyLayout) success() string { return l.join(runsDir, string(l.run), successMarker) }

func (l keyLayout) join(parts ...string) string {
	if l.prefix != "" {
		parts = append([]string{l.prefix}, parts...)
	}
	return strings.Join(parts, "/")
}
