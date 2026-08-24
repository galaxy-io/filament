package httpapi

import (
	_ "embed"
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
	return newCatalogSource(notionManifest)
}

// NewLinear returns a Source backed by the embedded Linear manifest.
func NewLinear() *Source {
	return newCatalogSource(linearManifest)
}

// NewAttio returns a Source backed by the embedded Attio REST API manifest.
func NewAttio() *Source {
	return newCatalogSource(attioManifest)
}

// NewGitHub returns a Source backed by the embedded GitHub REST API manifest.
func NewGitHub() *Source {
	return newCatalogSource(githubManifest)
}

// NewSlack returns a Source backed by the embedded Slack Web API manifest.
func NewSlack() *Source {
	return newCatalogSource(slackManifest)
}

// NewResend returns a Source backed by the embedded Resend REST API manifest.
func NewResend() *Source {
	return newCatalogSource(resendManifest)
}

// NewStripe returns a Source backed by the embedded Stripe REST API manifest.
func NewStripe() *Source {
	return newCatalogSource(stripeManifest)
}

// NewPostHog returns a Source backed by the embedded PostHog REST API manifest.
func NewPostHog() *Source {
	return newCatalogSource(posthogManifest)
}

func newCatalogSource(data []byte) *Source {
	return NewManifest(data)
}
