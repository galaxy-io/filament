package httpapi

import (
	_ "embed"

	ingestion "github.com/galaxy-io/filament"
)

//go:embed manifests/notion.yaml
var notionManifest []byte

//go:embed manifests/linear.yaml
var linearManifest []byte

// NewNotion returns a Source backed by the embedded Notion manifest.
func NewNotion() *Source {
	return NewManifest("notion", "Notion", notionManifest, ingestion.ConfigSchema{Fields: []ingestion.ConfigField{
		{Name: "api_key", Type: ingestion.FieldSecret, Required: true, Help: "Notion integration secret"},
	}})
}

// NewLinear returns a Source backed by the embedded Linear manifest.
func NewLinear() *Source {
	return NewManifest("linear", "Linear", linearManifest, ingestion.ConfigSchema{Fields: []ingestion.ConfigField{
		{Name: "api_key", Type: ingestion.FieldSecret, Required: true, Help: "Linear API key"},
	}})
}
