package httpapi

import (
	_ "embed"

	"github.com/galaxy-io/filament"
)

//go:embed manifests/notion.yaml
var notionManifest []byte

//go:embed manifests/linear.yaml
var linearManifest []byte

//go:embed manifests/attio.yaml
var attioManifest []byte

//go:embed manifests/github.yaml
var githubManifest []byte

//go:embed manifests/slack.yaml
var slackManifest []byte

//go:embed manifests/gitlab.yaml
var gitlabManifest []byte

//go:embed manifests/bitbucket.yaml
var bitbucketManifest []byte

// NewNotion returns a Source backed by the embedded Notion manifest.
func NewNotion() *Source {
	return NewManifestWithMetadata("notion", "Notion", "All-in-one productivity app for notes, tasks, databases, and knowledge collaboration.", "https://cdn.getgalaxy.io/sources/source-icon-notion-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-notion-light.svg", notionManifest, manifestOwnedConfig())
}

// NewLinear returns a Source backed by the embedded Linear manifest.
func NewLinear() *Source {
	return NewManifestWithMetadata("linear", "Linear", "Modern issue tracking and project management tool built for high-performance teams.", "https://cdn.getgalaxy.io/sources/source-icon-linear-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-linear-light.svg", linearManifest, manifestOwnedConfig())
}

// NewAttio returns a Source backed by the embedded Attio REST API manifest.
func NewAttio() *Source {
	return NewManifestWithMetadata("attio", "Attio", "CRM platform designed for modern teams to centralize customer data, pipelines, and workflows.", "https://cdn.getgalaxy.io/sources/source-icon-attio-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-attio-light.svg", attioManifest, manifestOwnedConfig())
}

// NewGitHub returns a Source backed by the embedded GitHub REST API manifest.
func NewGitHub() *Source {
	return NewManifestWithMetadata("github", "GitHub", "Code hosting platform for version control, collaboration, and software development workflows.", "https://cdn.getgalaxy.io/sources/source-icon-github-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-github-light.svg", githubManifest, manifestOwnedConfig())
}

// NewSlack returns a Source backed by the embedded Slack Web API manifest.
func NewSlack() *Source {
	return NewManifestWithMetadata("slack", "Slack", "Messaging and collaboration platform designed for teams to communicate and work together efficiently.", "https://cdn.getgalaxy.io/sources/source-icon-slack-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-slack-light.svg", slackManifest, manifestOwnedConfig())
}

// NewGitLab returns a Source backed by the embedded GitLab REST API manifest.
func NewGitLab() *Source {
	return NewManifestWithMetadata("gitlab", "GitLab", "DevSecOps platform for planning, coding, testing, and deploying software with integrated CI/CD.", "https://cdn.getgalaxy.io/sources/source-icon-gitlab-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-gitlab-light.svg", gitlabManifest, manifestOwnedConfig())
}

// NewBitbucket returns a Source backed by the embedded Bitbucket Cloud REST API manifest.
func NewBitbucket() *Source {
	return NewManifestWithMetadata("bitbucket", "Bitbucket", "Git-based code repository hosting with integrated CI/CD and collaboration tools.", "https://cdn.getgalaxy.io/sources/source-icon-bitbucket-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-bitbucket-light.svg", bitbucketManifest, manifestOwnedConfig())
}

func manifestOwnedConfig() filament.ConfigSchema { return filament.ConfigSchema{} }
