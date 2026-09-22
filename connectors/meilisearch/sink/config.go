package sink

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/galaxy-io/filament"
)

const (
	defaultPort         = 7700
	defaultBatchSize    = 10_000
	defaultTaskTimeout  = 60 * time.Second
	defaultWaitForTasks = true
	defaultGzip         = true
)

// Config describes the parsed configuration for a Meilisearch sink.
type Config struct {
	URL          string
	APIKey       string
	IndexPrefix  string
	Index        string
	PrimaryKey   string
	BatchSize    int
	Gzip         bool
	WaitForTasks bool
	TaskTimeout  time.Duration
}

// ConfigSchema returns the configuration fields accepted by the Meilisearch sink.
func ConfigSchema() filament.ConfigSchema {
	return filament.ConfigSchema{
		Fields: []filament.ConfigField{
			{
				Name:     "url",
				Type:     filament.FieldString,
				Required: true,
				Scope:    filament.ScopeConnection,
				Help:     "Meilisearch server URL (e.g. http://localhost:7700 or https://ms.example.com)",
			},
			{
				Name:     "api_key",
				Type:     filament.FieldSecret,
				Required: false,
				Secret:   true,
				Scope:    filament.ScopeConnection,
				Help:     "Meilisearch master key or API key with document and index permissions",
			},
			{
				Name:     "index_prefix",
				Type:     filament.FieldString,
				Required: false,
				Default:  "",
				Scope:    filament.ScopePipeline,
				Help:     "Prefix prepended to resource names to form the Meilisearch index UID",
			},
			{
				Name:     "index",
				Type:     filament.FieldString,
				Required: false,
				Default:  "",
				Scope:    filament.ScopePipeline,
				Help:     "Explicit destination index UID override. When empty, defaults to the sanitized resource name",
			},
			{
				Name:     "primary_key",
				Type:     filament.FieldString,
				Required: false,
				Default:  "",
				Scope:    filament.ScopePipeline,
				Help:     "Primary key attribute name for Meilisearch documents. If omitted, derived from the source schema",
			},
			{
				Name:     "batch_size",
				Type:     filament.FieldInt,
				Required: false,
				Default:  defaultBatchSize,
				Scope:    filament.ScopePipeline,
				Help:     "Preferred maximum document count per HTTP batch sent to Meilisearch",
			},
			{
				Name:     "gzip",
				Type:     filament.FieldBool,
				Required: false,
				Default:  defaultGzip,
				Scope:    filament.ScopePipeline,
				Help:     "Compress NDJSON payloads with gzip to reduce network transfer size",
			},
			{
				Name:     "wait_for_tasks",
				Type:     filament.FieldBool,
				Required: false,
				Default:  defaultWaitForTasks,
				Scope:    filament.ScopePipeline,
				Help:     "Wait for Meilisearch indexing tasks to finish on Commit to ensure durable writes",
			},
			{
				Name:     "task_timeout",
				Type:     filament.FieldDuration,
				Required: false,
				Default:  defaultTaskTimeout,
				Scope:    filament.ScopePipeline,
				Help:     "Maximum time to wait for enqueued indexing tasks to complete on Commit",
			},
		},
	}
}

// ParseConfig parses and validates a filament.Config into a structured Config.
func ParseConfig(cfg filament.Config) (Config, error) {
	rawURL := strings.TrimSpace(cfg.String("url"))
	if rawURL == "" {
		rawURL = strings.TrimSpace(cfg.String("host"))
	}
	if rawURL == "" {
		return Config{}, fmt.Errorf("meilisearch sink: url is required")
	}

	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		rawURL = "http://" + rawURL
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return Config{}, fmt.Errorf("meilisearch sink: invalid url %q: %w", rawURL, err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return Config{}, fmt.Errorf("meilisearch sink: url scheme must be http or https, got %q", parsedURL.Scheme)
	}

	batchSize := cfg.Int("batch_size")
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	taskTimeout := cfg.Duration("task_timeout")
	if taskTimeout <= 0 {
		taskTimeout = defaultTaskTimeout
	}

	gzip := defaultGzip
	if cfg.Has("gzip") {
		gzip = cfg.Bool("gzip")
	}

	waitForTasks := defaultWaitForTasks
	if cfg.Has("wait_for_tasks") {
		waitForTasks = cfg.Bool("wait_for_tasks")
	}

	return Config{
		URL:          strings.TrimRight(parsedURL.String(), "/"),
		APIKey:       strings.TrimSpace(cfg.Secret("api_key")),
		IndexPrefix:  strings.TrimSpace(cfg.String("index_prefix")),
		Index:        strings.TrimSpace(cfg.String("index")),
		PrimaryKey:   strings.TrimSpace(cfg.String("primary_key")),
		BatchSize:    batchSize,
		Gzip:         gzip,
		WaitForTasks: waitForTasks,
		TaskTimeout:  taskTimeout,
	}, nil
}
