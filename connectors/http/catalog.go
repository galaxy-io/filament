//go:embed manifests/slack.yaml
var slackManifest []byte

//go:embed manifests/hubspot.yaml
var hubspotManifest []byte

// NewSlack returns a Source backed by the embedded Slack Web API manifest.
func NewSlack() *Source {
	return NewManifestWithMetadata("slack", "Slack", "Messaging and collaboration platform designed for teams to communicate and work together efficiently.", "https://cdn.getgalaxy.io/sources/source-icon-slack-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-slack-light.svg", slackManifest, manifestOwnedConfig())
}

// NewHubspot returns a Source backed by the embedded HubSpot CRM manifest.
func NewHubspot() *Source {
	return NewManifestWithMetadata("hubspot", "HubSpot", "CRM platform for marketing, sales,和服务, and customer relationship management.", "https://cdn.getgalaxy.io/sources/source-icon-hubspot-dark.svg", "https://cdn.getgalaxy.io/sources/source-icon-hubspot-light.svg", hubspotManifest, manifestOwnedConfig())
}

func manifestOwnedConfig() filament.ConfigSchema { return filament.ConfigSchema{} }
