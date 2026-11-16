package filament

import (
	"encoding/json"
	"slices"
)

// WebhookSpec describes how a source receives provider webhooks through
// Filament's inbox. A source that sets ConnectorSpec.Webhook can own a
// webhook endpoint. The receiver builds the named verifier from its
// parameters, so verification needs no connector code in the server.
type WebhookSpec struct {
	// Verifier names a webhooks verifier profile and VerifierParams
	// configure it. Saving an endpoint copies both onto the endpoint.
	Verifier       string
	VerifierParams json.RawMessage
	// Ingress configures how the receiver routes deliveries sent to the
	// endpoint's single URL, as a webhooks.Ingress JSON object. Saving an
	// endpoint copies it too.
	Ingress json.RawMessage
	// DynamicResources means the endpoint configuration names the resources,
	// and discovery lists them from it. Otherwise the source discovers its
	// own webhook resources and marks them with ResourceIngressWebhook.
	DynamicResources bool
	// Resources names the source's webhook resources when they are fixed.
	// An endpoint accepts only these. Empty with DynamicResources.
	Resources []string
	// IngestionField names an enum connection config field that chooses how
	// a connection reads the source: IngestionAPI, IngestionWebhooks or
	// IngestionBoth. An unset value is IngestionAPI. Empty means every
	// connection of the source receives webhooks.
	IngestionField string
	// Setup is Markdown telling the user how to configure the sender.
	Setup string
	// SecretIssuer says who creates the signing secret:
	// WebhookSecretFilament (the default when empty) or
	// WebhookSecretProvider.
	SecretIssuer string
}

// Values of WebhookSpec.SecretIssuer.
const (
	// WebhookSecretFilament means Filament generates the signing secret and
	// the user copies it into the sender.
	WebhookSecretFilament = "filament"
	// WebhookSecretProvider means the provider generates the signing secret
	// and the user copies it into the connection. Filament never mints one.
	WebhookSecretProvider = "provider"
)

// ProviderIssuesSecret reports whether the provider creates the signing
// secret, so the connection must be given it.
func (w *WebhookSpec) ProviderIssuesSecret() bool {
	return w != nil && w.SecretIssuer == WebhookSecretProvider
}

// Values of a source's WebhookSpec.IngestionField.
const (
	// IngestionAPI reads only the provider's API. It is the default.
	IngestionAPI = "api"
	// IngestionWebhooks reads only webhook deliveries. The connection needs
	// no API credentials.
	IngestionWebhooks = "webhooks"
	// IngestionBoth reads the API and webhook deliveries.
	IngestionBoth = "both"
)

// Ingestion returns how a connection with cfg reads the source.
func (w *WebhookSpec) Ingestion(cfg Config) string {
	switch {
	case w == nil:
		return IngestionAPI
	case w.IngestionField == "":
		return IngestionWebhooks
	}
	return IngestionOf(cfg.String(w.IngestionField))
}

// IngestionOf normalizes a stored ingestion value. Unset or unknown values
// read the API only.
func IngestionOf(value string) string {
	switch value {
	case IngestionWebhooks, IngestionBoth:
		return value
	}
	return IngestionAPI
}

// Enabled reports whether a connection with cfg receives webhooks.
func (w *WebhookSpec) Enabled(cfg Config) bool {
	return w.Ingestion(cfg) != IngestionAPI
}

// Resource metadata that marks how a discovered resource is read. An API
// resource has no ingress key. A webhook resource reads the inbox in a
// continuous run.
const (
	ResourceMetaIngress    = "ingress"
	ResourceIngressWebhook = "webhook"
)

// Routed reports whether deliveries sent to the endpoint's single URL are
// routed to a resource by their event type.
func (w *WebhookSpec) Routed() bool {
	if w == nil || len(w.Ingress) == 0 {
		return false
	}
	var ingress struct {
		Route json.RawMessage `json:"route"`
	}
	return json.Unmarshal(w.Ingress, &ingress) == nil && len(ingress.Route) > 0 && string(ingress.Route) != "null"
}

// GetVerifier returns the verifier name, empty for a nil spec.
func (w *WebhookSpec) GetVerifier() string {
	if w == nil {
		return ""
	}
	return w.Verifier
}

// SplitsExecution reports whether the source has both bounded and continuous
// resources. Its fixed webhook resources run continuously and every other
// resource runs bounded, whatever execution mode the pipeline defaults to, so
// one pipeline can schedule API reads and stream webhooks from one source.
func (w *WebhookSpec) SplitsExecution() bool {
	return w != nil && !w.DynamicResources && len(w.Resources) > 0
}

// ResourceExecution returns the execution mode resource runs in. A source
// that splits execution decides it per resource. Any other source runs every
// resource in fallback, the pipeline's mode. An all-resources edge, named by
// the empty resource, reads the API resources of a splitting source:
// resource planning leaves webhook resources out of a bounded run.
func ResourceExecution(spec ConnectorSpec, resource string, fallback ExecutionMode) ExecutionMode {
	if !spec.Webhook.SplitsExecution() {
		return fallback
	}
	if resource != "" && slices.Contains(spec.Webhook.Resources, resource) {
		return ExecutionContinuous
	}
	return ExecutionBounded
}
