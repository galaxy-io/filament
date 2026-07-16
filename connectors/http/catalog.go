package httpapi

import (
	_ "embed"

	"github.com/galaxy-io/filament"
)

//go:embed manifests/notion.yaml
var notionManifest []byte

//go:embed manifests/linear.yaml
var linearManifest []byte

// NewNotion returns a Source backed by the embedded Notion manifest.
func NewNotion() *Source {
	return NewManifest("notion", "Notion", notionManifest, filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "api_key", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "Notion integration secret"},
	}})
}

// NewLinear returns a Source backed by the embedded Linear manifest.
func NewLinear() *Source {
	return NewManifest("linear", "Linear", linearManifest, filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "api_key", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection, Help: "Linear API key"},
	}})
}
