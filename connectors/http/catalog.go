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

//go:embed manifests/resend.yaml
var resendManifest []byte

//go:embed manifests/posthog.yaml
var posthogManifest []byte

//go:embed manifests/stripe.yaml
var stripeManifest []byte

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

// NewResend returns a Source backed by the embedded Resend REST API manifest.
func NewResend() *Source {
	return NewManifestWithMetadata("resend", "Resend", "Email API for developers covering transactional sends, inbound mail, broadcasts, contacts, and audience management.", "https://cdn.getgalaxy.io/sources/source-icon-resend-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-resend-light.svg", resendManifest, manifestOwnedConfig())
}

// NewStripe returns a Source backed by the embedded Stripe REST API manifest.
func NewStripe() *Source {
	return NewManifestWithMetadata("stripe", "Stripe", "Payments platform covering charges, payment intents, invoicing, subscriptions, and the account balance ledger.", "https://cdn.getgalaxy.io/sources/source-icon-stripe-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-stripe-light.svg", stripeManifest, manifestOwnedConfig())
}

// NewPostHog returns a Source backed by the embedded PostHog REST API manifest.
func NewPostHog() *Source {
	return NewManifestWithMetadata("posthog", "PostHog", "Product analytics platform covering people, events, cohorts, feature flags, experiments, and session replay.", "https://cdn.getgalaxy.io/sources/source-icon-posthog-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-posthog-light.svg", posthogManifest, manifestOwnedConfig())
}

func manifestOwnedConfig() filament.ConfigSchema { return filament.ConfigSchema{} }
