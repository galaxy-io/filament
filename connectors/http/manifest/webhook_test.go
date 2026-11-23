package manifest

import (
	"reflect"
	"strings"
	"testing"
)

const webhookBase = `version: 1
name: hooks
display_name: Hooks
description: Webhook fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection: {base_url: https://example.com}
defaults:
  response:
    records: $.results
    pagination: {next_url: next}
%s
resources:
  - name: events
    path: /events
    primary_key: [id]
    fields: {id: string}
%s
`

const validWebhookBlock = `webhook:
  verifier: {profile: standard_webhooks}`

func TestParseWebhookResource(t *testing.T) {
	m, err := Parse([]byte(strings.Replace(strings.Replace(webhookBase, "%s", validWebhookBlock, 1), "%s", `
  - name: events_webhook
    mode: webhook
    primary_key: [id]
    fields: {id: {path: event.uuid, type: string}}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	live := m.Resources[1]
	if !live.IsWebhook() || live.Path != "" {
		t.Fatalf("webhook resource = %#v", live)
	}
	// Request defaults never reach a webhook resource. With no records path
	// the whole delivery body is one record.
	if live.Response.RecordsPath != "" || live.Response.Cardinality != "one" || live.Pagination.Type != "" || live.Method != "" {
		t.Fatalf("webhook response %#v pagination %#v method %q", live.Response, live.Pagination, live.Method)
	}
}

func TestParseRejectsInvalidWebhooks(t *testing.T) {
	for name, tc := range map[string]struct{ block, resource string }{
		"no webhook block": {"", `
  - name: live
    mode: webhook`},
		"no webhook resource": {validWebhookBlock, ""},
		"unknown verifier": {`webhook:
  verifier: {profile: nope}`, `
  - name: live
    mode: webhook`},
		"stray verifier params": {`webhook:
  verifier: {profile: standard_webhooks, params: {header: x}}`, `
  - name: live
    mode: webhook`},
		"request on webhook resource": {validWebhookBlock, `
  - name: live
    mode: webhook
    path: /live`},
		"unsafe name": {validWebhookBlock, `
  - name: live.events
    mode: webhook`},
		"headers on webhook resource": {validWebhookBlock, `
  - name: live
    mode: webhook
    headers: {X-Api-Version: "2"}`},
		"response match on webhook resource": {validWebhookBlock, `
  - name: live
    mode: webhook
    response: {match: {ok: "true"}}`},
		"events without route": {validWebhookBlock, `
  - name: live
    mode: webhook
    events: [push]`},
		"events on API resource": {routedWebhookBlock, `
  - name: live
    mode: webhook
    events: [push]
  - name: more
    path: /more
    events: [pull]`},
		"route without events": {routedWebhookBlock, `
  - name: live
    mode: webhook`},
		"event routed twice": {routedWebhookBlock, `
  - name: live
    mode: webhook
    events: [push]
  - name: other
    mode: webhook
    events: [push]`},
		"webhook resource in discovery.include": {validWebhookBlock + `
discovery:
  mode: static
  include: [events, live]`, `
  - name: live
    mode: webhook`},
		"webhook resource in default_resources": {validWebhookBlock + `
discovery:
  mode: static
  default_resources: [live]`, `
  - name: live
    mode: webhook`},
		"challenge is not supported": {validWebhookBlock + `
  challenge: {method: GET, echo: {query: hub.challenge}}`, `
  - name: live
    mode: webhook`},
		"setup single URL without route": {validWebhookBlock + `
  setup: Send to {{delivery_url}}.`, `
  - name: live
    mode: webhook`},
		"setup names unknown resource": {validWebhookBlock + `
  setup: Send to {{delivery_url:events}}.`, `
  - name: live
    mode: webhook`},
		"unknown secret issuer": {validWebhookBlock + `
  secret: sender`, `
  - name: live
    mode: webhook`},
		"reserved config field": {validWebhookBlock + `
config:
  ingestion: {type: string}`, `
  - name: live
    mode: webhook`},
		"conditional config field": {validWebhookBlock + `
config:
  region: {type: enum, enum: [us, eu]}
  token: {type: secret, visible_when: {field: region, values: [us]}}`, `
  - name: live
    mode: webhook`},
	} {
		data := strings.Replace(strings.Replace(webhookBase, "%s", tc.block, 1), "%s", tc.resource, 1)
		if _, err := Parse([]byte(data)); err == nil {
			t.Errorf("%s: manifest parsed", name)
		}
	}
}

const routedWebhookBlock = `webhook:
  verifier: {profile: standard_webhooks}
  route: {event: {header: X-Event-Type}}`

func TestParseRoutedWebhook(t *testing.T) {
	block := routedWebhookBlock + `
  setup: |
    Send every event to {{delivery_url}}, or orders alone to {{ delivery_url:orders_webhook }}.`
	m, err := Parse([]byte(strings.Replace(strings.Replace(webhookBase, "%s", block, 1), "%s", `
  - name: orders_webhook
    mode: webhook
    events: [order.created, order.updated]
    primary_key: [id]
    fields: {id: string}
  - name: everything_else
    mode: webhook
    events: ["*"]
    primary_key: [id]
    fields: {id: string}`, 1)))
	if err != nil {
		t.Fatal(err)
	}
	ingress := m.Ingress()
	if ingress.Route == nil || ingress.Route.Event.Header != "X-Event-Type" {
		t.Fatalf("route = %+v", ingress.Route)
	}
	want := map[string]string{"order.created": "orders_webhook", "order.updated": "orders_webhook", "*": "everything_else"}
	if !reflect.DeepEqual(ingress.Route.Events, want) {
		t.Fatalf("route events = %v, want %v", ingress.Route.Events, want)
	}
	if err := ingress.Validate(); err != nil {
		t.Fatal(err)
	}
}
