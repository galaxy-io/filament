package httpapi

import (
	_ "embed"

	"github.com/galaxy-io/filament"
)

//go:embed manifests/notion.yaml
var notionManifest []byte

//go:embed manifests/linear.yaml
var linearManifest []byte

func NewNotion() *Source {
	return NewManifest("notion", "Notion", notionManifest, ingestion.ConfigSchema{Fields: []ingestion.ConfigField{
		{Name: "api_key", Type: ingestion.FieldSecret, Required: true, Scope: ingestion.ScopeConnection, Help: "Notion integration secret"},
	}})
}

func NewLinear() *Source {
	return NewManifest("linear", "Linear", linearManifest, ingestion.ConfigSchema{Fields: []ingestion.ConfigField{
		{Name: "api_key", Type: ingestion.FieldSecret, Required: true, Scope: ingestion.ScopeConnection, Help: "Linear API key"},
	}})
}
