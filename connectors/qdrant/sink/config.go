// Package sink implements the filament.Sink interface for Qdrant vector store destinations.
package sink

import (
	"fmt"
	"strings"

	"github.com/galaxy-io/filament"
)

// defaultBatchSize specifies the default row limit per Qdrant point batch upsert.
const defaultBatchSize = 500

// Config defines the connection and execution settings for the Qdrant sink.
type Config struct {
	// URL specifies the Qdrant cluster REST endpoint URI (e.g. https://xyz.qdrant.tech).
	URL string `json:"url"`

	// APIKey specifies the authentication API key or token for Qdrant Cloud.
	APIKey string `json:"api_key,omitempty"`

	// Collection specifies the target Qdrant collection name.
	Collection string `json:"collection,omitempty"`

	// PrimaryKey specifies the record column name used as unique document ID.
	PrimaryKey string `json:"primary_key,omitempty"`

	// TextFields lists column names used to construct document text content.
	TextFields []string `json:"text_fields,omitempty"`

	// EmbeddingField specifies an optional column containing pre-computed float vectors.
	EmbeddingField string `json:"embedding_field,omitempty"`

	// BatchSize specifies the maximum points written per batch.
	BatchSize int `json:"batch_size,omitempty"`
}

// ConfigSchema describes the configuration schema for UI catalog display and validation.
func ConfigSchema() filament.ConfigSchema {
	return filament.ConfigSchema{
		Fields: []filament.ConfigField{
			{
				Name:     "url",
				Type:     filament.FieldString,
				Required: true,
				Scope:    filament.ScopeConnection,
				Help:     "Qdrant REST API endpoint URL (e.g., https://xyz.qdrant.tech or http://localhost:6333)",
			},
			{
				Name:     "api_key",
				Type:     filament.FieldSecret,
				Required: false,
				Secret:   true,
				Scope:    filament.ScopeConnection,
				Help:     "Qdrant API Key or cluster authorization token",
			},
			{
				Name:     "collection",
				Type:     filament.FieldString,
				Required: false,
				Scope:    filament.ScopePipeline,
				Help:     "Target Qdrant collection name",
			},
			{
				Name:     "primary_key",
				Type:     filament.FieldString,
				Required: false,
				Scope:    filament.ScopePipeline,
				Help:     "Column name in source records mapped as point ID",
			},
			{
				Name:     "text_fields",
				Type:     filament.FieldList,
				Required: false,
				Scope:    filament.ScopePipeline,
				Help:     "Column names to concatenate for document text",
			},
			{
				Name:     "embedding_field",
				Type:     filament.FieldString,
				Required: false,
				Scope:    filament.ScopePipeline,
				Help:     "Column name containing pre-computed float32 array vectors",
			},
			{
				Name:     "batch_size",
				Type:     filament.FieldInt,
				Required: false,
				Default:  defaultBatchSize,
				Scope:    filament.ScopePipeline,
				Help:     "Maximum points per batch write",
			},
		},
	}
}

// ParseConfig decodes and validates a filament.Config object into a Config struct.
func ParseConfig(cfg filament.Config) (Config, error) {
	c := Config{
		BatchSize: defaultBatchSize,
	}

	c.URL = cfg.String("url")
	if strings.TrimSpace(c.URL) == "" {
		return c, fmt.Errorf("qdrant sink: 'url' is required")
	}

	if val := cfg.String("api_key"); val != "" {
		c.APIKey = val
	}

	// Security check: require HTTPS when api_key is configured on remote endpoints
	if c.APIKey != "" && !strings.HasPrefix(c.URL, "https://") && !strings.HasPrefix(c.URL, "mock://") {
		return c, fmt.Errorf("qdrant sink: HTTPS scheme (https://) is required when 'api_key' is configured")
	}

	if val := cfg.String("collection"); val != "" {
		c.Collection = val
	}

	if val := cfg.String("primary_key"); val != "" {
		c.PrimaryKey = val
	}

	if raw, ok := cfg.Raw()["text_fields"]; ok {
		switch v := raw.(type) {
		case []string:
			c.TextFields = v
		case []any:
			var fields []string
			for _, item := range v {
				if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
					fields = append(fields, strings.TrimSpace(s))
				}
			}
			c.TextFields = fields
		case string:
			parts := strings.Split(v, ",")
			var fields []string
			for _, p := range parts {
				if s := strings.TrimSpace(p); s != "" {
					fields = append(fields, s)
				}
			}
			c.TextFields = fields
		}
	}

	if val := cfg.String("embedding_field"); val != "" {
		c.EmbeddingField = val
	}

	if size := cfg.Int("batch_size"); size > 0 {
		c.BatchSize = size
	}

	return c, nil
}
