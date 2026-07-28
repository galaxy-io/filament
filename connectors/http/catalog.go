package httpapi

import (
	_ "embed"

	"github.com/galaxy-io/filament"
)

//go:embed manifests/notion.yaml
var notionManifest []byte

//go:embed manifests/linear.yaml
var linearManifest []byte

//go:embed manifests/github.yaml
var githubManifest []byte

// NewNotion returns a Source backed by the embedded Notion manifest.
func NewNotion() *Source {
	return NewManifestWithMetadata("notion", "Notion", "All-in-one productivity app for notes, tasks, databases, and knowledge collaboration.", "https://cdn.getgalaxy.io/sources/source-icon-notion-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-notion-light.svg", notionManifest, manifestOwnedConfig())
}

// NewLinear returns a Source backed by the embedded Linear manifest.
func NewLinear() *Source {
	return NewManifestWithMetadata("linear", "Linear", "Modern issue tracking and project management tool built for high-performance teams.", "https://cdn.getgalaxy.io/sources/source-icon-linear-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-linear-light.svg", linearManifest, manifestOwnedConfig())
}

// NewGitHub returns a Source backed by the embedded GitHub REST API manifest.
func NewGitHub() *Source {
	return NewManifestWithMetadata("github", "GitHub", "Code hosting platform for version control, collaboration, and software development workflows.", "https://cdn.getgalaxy.io/sources/source-icon-github-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-github-light.svg", githubManifest, manifestOwnedConfig())
}

func manifestOwnedConfig() filament.ConfigSchema { return filament.ConfigSchema{} }
