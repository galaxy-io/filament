package manifest

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/webhooks"
)

// ModeWebhook marks a resource read from webhook deliveries rather than an
// API request. Its rows come from the bodies the provider sends, decoded with
// the resource's records path and projected with its fields, so a webhook
// resource and the API resource it mirrors share one schema.
const ModeWebhook = "webhook"

// IngestionField is the connection config field that chooses whether a
// connection reads the API, webhook deliveries or both. The source adds it
// when the manifest declares a webhook block, and shows the manifest's own
// config fields only when the connection reads the API.
const IngestionField = "ingestion"

// WebhookSpec declares that the provider can push records to Filament. A
// connection receives them when its IngestionField includes webhooks.
type WebhookSpec struct {
	// Verifier selects a webhooks verifier profile and its parameters. The
	// server authenticates deliveries with it, without the manifest.
	Verifier WebhookVerifier `yaml:"verifier"`
	// Route, when set, accepts every delivery at the endpoint's single URL
	// and routes it to the webhook resource whose events name its type.
	Route *WebhookRoute `yaml:"route,omitempty"`
	// Setup is Markdown telling the user how to configure the sender, shown
	// alongside the delivery URL and signing secret. It may name delivery
	// URLs with SetupDeliveryURL placeholders, which the UI fills in.
	Setup string `yaml:"setup,omitempty"`
	// Secret says who creates the signing secret: "filament", the default,
	// or "provider" when the provider generates it and the user copies it
	// into the connection.
	Secret string `yaml:"secret,omitempty"`
}

// WebhookVerifier names a verifier profile and its parameters.
type WebhookVerifier struct {
	Profile string         `yaml:"profile"`
	Params  map[string]any `yaml:"params,omitempty"`
}

// WebhookValue names where a value is in a request. Exactly one is set.
type WebhookValue struct {
	Header string `yaml:"header,omitempty"`
	Body   string `yaml:"body,omitempty"`
}

func (v WebhookValue) source() webhooks.ValueSource {
	return webhooks.ValueSource{Header: v.Header, Body: v.Body}
}

// WebhookRoute names where a delivery's event type is. Each webhook
// resource lists the event types it reads in its events.
type WebhookRoute struct {
	Event WebhookValue `yaml:"event"`
}

// ParamsJSON returns the verifier parameters as a JSON object, nil when none.
func (v WebhookVerifier) ParamsJSON() (json.RawMessage, error) {
	if len(v.Params) == 0 {
		return nil, nil
	}
	return json.Marshal(v.Params)
}

// Ingress builds the receiver's routing configuration. The route maps every event type a webhook resource lists to that resource.
func (m *Manifest) Ingress() webhooks.Ingress {
	var out webhooks.Ingress
	if m.Webhook == nil {
		return out
	}
	if route := m.Webhook.Route; route != nil {
		events := map[string]string{}
		for _, r := range m.Resources {
			if r.IsWebhook() {
				for _, event := range r.Events {
					events[event] = r.Name
				}
			}
		}
		out.Route = &webhooks.Route{Event: route.Event.source(), Events: events}
	}
	return out
}

// IngressJSON is Ingress encoded for the connector spec, nil when empty.
func (m *Manifest) IngressJSON() json.RawMessage {
	ingress := m.Ingress()
	if ingress.IsZero() {
		return nil
	}
	raw, err := json.Marshal(ingress)
	if err != nil {
		return nil
	}
	return raw
}

// IsWebhook reports whether the resource is read from webhook deliveries.
func (r Resource) IsWebhook() bool { return r.Mode == ModeWebhook }

// normalizeWebhookResource decodes a delivery body the way records decodes a
// response. No records path means the whole body is one record. Request
// defaults never apply: a webhook resource makes no request.
func normalizeWebhookResource(r *Resource) {
	switch r.Records {
	case "":
		r.Response.Root = DefaultResponseRoot
		r.Response.RecordsPath = ""
		if r.Response.Cardinality == "" {
			r.Response.Cardinality = "one"
		}
	case "$":
		r.Response.Root = "array"
		r.Response.RecordsPath = ""
	default:
		r.Response.Root = DefaultResponseRoot
		r.Response.RecordsPath = strings.TrimPrefix(r.Records, "$.")
	}
}

// webhookResourceShape keeps only what a webhook resource may declare. A
// webhook resource that differs from its shape declares a request setting.
// Listing what is allowed, rather than what is not, keeps a request field
// added to Resource later from passing silently.
func webhookResourceShape(r Resource) Resource {
	return Resource{
		Name: r.Name, Mode: r.Mode, Records: r.Records, Events: r.Events,
		PrimaryKey: r.PrimaryKey, Fields: r.Fields, FieldOverrides: r.FieldOverrides,
		UseFields: r.UseFields, ExcludeFields: r.ExcludeFields,
		Response: ResponseSpec{Root: r.Response.Root, RecordsPath: r.Response.RecordsPath, Cardinality: r.Response.Cardinality},
	}
}

// SetupDeliveryURL matches a delivery URL placeholder in setup text:
// {{delivery_url}} is the endpoint's single URL, which needs a route, and
// {{delivery_url:<resource>}} is one webhook resource's URL.
var SetupDeliveryURL = regexp.MustCompile(`\{\{\s*delivery_url(?::([^}\s]+))?\s*\}\}`)

// validateWebhook checks the webhook block and every webhook resource.
func validateWebhook(agg *errs.ManifestErrors, m *Manifest) {
	var webhookResources []string
	events := map[string]string{}
	for i := range m.Resources {
		r := &m.Resources[i]
		at := fmt.Sprintf("resources[%q]", r.Name)
		if !r.IsWebhook() {
			if len(r.Events) > 0 {
				_ = agg.Addf(at+".events", "requires mode: webhook")
			}
			continue
		}
		webhookResources = append(webhookResources, r.Name)
		if m.Webhook == nil {
			_ = agg.Addf(at+".mode", "webhook resources require a top-level webhook block")
		}
		if !webhooks.ValidResourceName(r.Name) {
			_ = agg.Addf(at+".name", "webhook resource names may use only letters, digits, _ and -, up to 64 characters")
		}
		if !reflect.DeepEqual(webhookResourceShape(*r), *r) {
			_ = agg.Addf(at, "a webhook resource makes no request: declare only name, records, events, primary_key and fields")
		}
		if len(r.Events) > 0 && (m.Webhook == nil || m.Webhook.Route == nil) {
			_ = agg.Addf(at+".events", "requires webhook.route")
		}
		for _, event := range r.Events {
			if event == "" {
				_ = agg.Addf(at+".events", "may not list an empty event type")
				continue
			}
			if owner, ok := events[event]; ok {
				_ = agg.Addf(at+".events", "event %q is already routed to %q", event, owner)
				continue
			}
			events[event] = r.Name
		}
	}
	if m.Webhook == nil {
		return
	}
	if len(webhookResources) == 0 {
		_ = agg.Addf("webhook", "declares no resources with mode: webhook")
	}
	validateWebhookConfig(agg, m)
	params, err := m.Webhook.Verifier.ParamsJSON()
	if err == nil {
		_, err = webhooks.BuildVerifier(m.Webhook.Verifier.Profile, params)
	}
	if err != nil {
		_ = agg.Addf("webhook.verifier", "%v", err)
	}
	if m.Webhook.Route != nil && len(events) == 0 {
		_ = agg.Addf("webhook.route", "no webhook resource lists events")
	}
	ingress := m.Ingress()
	// An empty route is reported above. Validate what remains.
	if ingress.Route != nil && len(ingress.Route.Events) == 0 {
		ingress.Route = nil
	}
	if err := ingress.Validate(); err != nil {
		_ = agg.Addf("webhook", "%v", err)
	}
	for _, match := range SetupDeliveryURL.FindAllStringSubmatch(m.Webhook.Setup, -1) {
		switch resource := match[1]; {
		case resource == "" && m.Webhook.Route == nil:
			_ = agg.Addf("webhook.setup", "%s needs webhook.route; name a resource with {{delivery_url:<resource>}}", match[0])
		case resource != "" && !slices.Contains(webhookResources, resource):
			_ = agg.Addf("webhook.setup", "%s names %q, which is not a webhook resource", match[0], resource)
		}
	}
}

// validateWebhookConfig checks the config of a manifest with a webhook block.
// The source adds the IngestionField and shows each connection field only to
// connections that read the API, which is the one condition a field can carry.
func validateWebhookConfig(agg *errs.ManifestErrors, m *Manifest) {
	if _, ok := m.Config[IngestionField]; ok {
		_ = agg.Addf("config."+IngestionField, "is reserved for the webhook block")
	}
	for name, spec := range m.Config {
		if spec.VisibleWhen != nil && spec.Scope != "pipeline" {
			_ = agg.Addf("config."+name+".visible_when", "is not supported with a webhook block")
		}
	}
}
