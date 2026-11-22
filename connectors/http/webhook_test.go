package httpapi

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/webhooks/inbox"
	"github.com/galaxy-io/filament/webhooks/stream"
)

const webhookManifest = `version: 1
name: hooks
display_name: Hooks
description: Webhook fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection: {base_url: https://example.com}
webhook:
  verifier: {profile: standard_webhooks}
resources:
  - name: events
    path: /events
    primary_key: [uuid]
    fields: {uuid: string, event: string}
  - name: events_webhook
    mode: webhook
    primary_key: [uuid]
    fields:
      uuid: {path: event.uuid, type: string}
      event: {path: event.event, type: string}
  - name: batch_webhook
    mode: webhook
    records: $.batch
    primary_key: [uuid]
    fields: {uuid: string, event: string, at: {type: timestamptz, nullable: true}}
`

func configuredWebhookSource(t *testing.T, ingestion string) *Source {
	t.Helper()
	src := NewManifest([]byte(webhookManifest))
	if src.manifestErr != nil {
		t.Fatal(src.manifestErr)
	}
	if err := src.Configure(context.Background(), filament.NewConfig(map[string]any{"ingestion": ingestion})); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src
}

func TestWebhookResourcesDiscoveredOnlyWhenEnabled(t *testing.T) {
	ctx := context.Background()
	names := func(src *Source) map[string]map[string]string {
		res, err := src.Discover(ctx, filament.DiscoverOpts{})
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]map[string]string{}
		for _, r := range res.Resources {
			out[r.Name] = r.Metadata
		}
		return out
	}
	off := names(configuredWebhookSource(t, filament.IngestionAPI))
	if _, ok := off["events_webhook"]; ok || len(off) != 1 {
		t.Fatalf("webhooks off discovered %v", off)
	}
	on := names(configuredWebhookSource(t, filament.IngestionBoth))
	live := on["events_webhook"]
	if live[filament.ResourceMetaIngress] != filament.ResourceIngressWebhook {
		t.Fatalf("events_webhook metadata = %v", live)
	}
	if on["events"][filament.ResourceMetaIngress] != "" || len(on) != 3 {
		t.Fatalf("webhooks on discovered %v", on)
	}
	only := names(configuredWebhookSource(t, filament.IngestionWebhooks))
	if _, ok := only["events"]; ok || len(only) != 2 {
		t.Fatalf("webhooks only discovered %v", only)
	}
}

func TestWebhookResourcesStayOutOfBoundedRuns(t *testing.T) {
	src := configuredWebhookSource(t, filament.IngestionBoth)
	err := src.Extract(context.Background(), &collectSink{}, filament.ExtractOpts{Resources: []string{"events_webhook"}})
	if err == nil || !strings.Contains(err.Error(), "continuous pipeline") {
		t.Fatalf("bounded extract of a webhook resource: %v", err)
	}
}

func TestWebhookPlanReplicationStream(t *testing.T) {
	src := NewManifest([]byte(webhookManifest))
	request := func(ingestion string, resources ...string) filament.ReplicationStreamPlanningRequest {
		return filament.ReplicationStreamPlanningRequest{
			Execution: filament.ExecutionContinuous, ReplicationStreamID: "stream-1", SourceConnectionID: "source-1",
			Config: filament.NewConfig(map[string]any{"ingestion": ingestion}), Resources: resources,
		}
	}
	if _, err := src.PlanReplicationStream(request(filament.IngestionAPI, "events_webhook")); err == nil {
		t.Fatal("planned with webhooks off")
	}
	if _, err := src.PlanReplicationStream(request(filament.IngestionBoth, "events")); err == nil {
		t.Fatal("planned an API resource as a stream")
	}
	plan, err := src.PlanReplicationStream(request(filament.IngestionWebhooks, "events_webhook"))
	if err != nil || plan.ConsumerName != "webhook-stream-1" {
		t.Fatalf("plan %#v, err %v", plan, err)
	}
	if got := src.Spec().Webhook.Resources; len(got) != 2 || got[0] != "events_webhook" || got[1] != "batch_webhook" {
		t.Fatalf("webhook spec resources = %v", got)
	}
	if New().Spec().Stream != nil || New().Spec().Webhook != nil {
		t.Fatal("the generic manifest-path source advertises webhooks")
	}
}

func TestWebhookDeliveriesProjectLikeResponses(t *testing.T) {
	src := configuredWebhookSource(t, filament.IngestionBoth)
	sink := &collectSink{}
	rows := newRowSink(context.Background(), src, sink)
	err := src.writeDeliveries(context.Background(), rows, []stream.Delivery{
		{Delivery: inbox.Delivery{ID: "d1", Resource: "events_webhook", Body: []byte(`{"event":{"uuid":"u1","event":"$pageview"},"person":{}}`)}},
		{Delivery: inbox.Delivery{ID: "d2", Resource: "batch_webhook", Body: []byte(`{"batch":[{"uuid":"u2","event":"a"},{"uuid":"u3","event":"b"}]}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, rec := range sink.records {
		var row map[string]any
		if err := json.Unmarshal(rec.Data, &row); err != nil {
			t.Fatal(err)
		}
		got[rec.Resource+"/"+rec.ID] = row["event"].(string)
	}
	want := map[string]string{"events_webhook/u1": "$pageview", "batch_webhook/u2": "a", "batch_webhook/u3": "b"}
	if len(got) != len(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
	for key, event := range want {
		if got[key] != event {
			t.Fatalf("rows = %v, want %v", got, want)
		}
	}
}

type countingMetrics struct{ counts map[string]float64 }

func (m *countingMetrics) Counter(name string, labels ...filament.Label) filament.Counter {
	key := name
	for _, l := range labels {
		key += "," + l.Key + "=" + l.Value
	}
	return countingCounter{m, key}
}
func (m *countingMetrics) Gauge(string, ...filament.Label) filament.Gauge         { return nil }
func (m *countingMetrics) Histogram(string, ...filament.Label) filament.Histogram { return nil }

type countingCounter struct {
	m   *countingMetrics
	key string
}

func (c countingCounter) Inc()          { c.m.counts[c.key]++ }
func (c countingCounter) Add(v float64) { c.m.counts[c.key] += v }

// A delivery that does not decode as its resource is rejected and counted,
// whole, and the deliveries after it are still written.
func TestWebhookUndecodableDeliveriesAreSkipped(t *testing.T) {
	src := configuredWebhookSource(t, filament.IngestionBoth)
	metrics := &countingMetrics{counts: map[string]float64{}}
	src.SetWebhookInbox(nil, metrics)
	sink := &collectSink{}
	rows := newRowSink(context.Background(), src, sink)
	err := src.writeDeliveries(context.Background(), rows, []stream.Delivery{
		{Delivery: inbox.Delivery{ID: "d1", Resource: "events", Body: []byte(`{}`)}},
		{Delivery: inbox.Delivery{ID: "d2", Resource: "events_webhook", Body: []byte(`not json`)}},
		// The first record is valid and the second has a bad timestamp, so
		// neither is written.
		{Delivery: inbox.Delivery{ID: "d3", Resource: "batch_webhook", Body: []byte(`{"batch":[{"uuid":"u1","event":"a"},{"uuid":"u2","event":"b","at":"yesterday"}]}`)}},
		{Delivery: inbox.Delivery{ID: "d4", Resource: "batch_webhook", Body: []byte(`{"batch":[{"uuid":"u3","event":"c","at":"2026-11-16T00:00:00Z"}]}`)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.records) != 1 || sink.records[0].ID != "u3" {
		t.Fatalf("rows = %+v, want only u3", sink.records)
	}
	want := map[string]float64{
		"filament_webhook_rejected_deliveries_total,resource=events":         1,
		"filament_webhook_rejected_deliveries_total,resource=events_webhook": 1,
		"filament_webhook_rejected_deliveries_total,resource=batch_webhook":  1,
	}
	if len(metrics.counts) != len(want) {
		t.Fatalf("counts = %v, want %v", metrics.counts, want)
	}
	for key, n := range want {
		if metrics.counts[key] != n {
			t.Fatalf("counts = %v, want %v", metrics.counts, want)
		}
	}
}

// A manifest that only receives webhooks makes no requests, so testing its
// connection succeeds without a probe. The base URL is unreachable on purpose.
func TestWebhookOnlyManifestPassesConnectionTest(t *testing.T) {
	src := NewManifest([]byte(`version: 1
name: hooks_only
display_name: Hooks only
description: Webhook-only fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection: {base_url: "http://127.0.0.1:1"}
webhook:
  verifier: {profile: standard_webhooks}
resources:
  - name: events_webhook
    mode: webhook
    primary_key: [id]
    fields: {id: string}
`))
	if src.manifestErr != nil {
		t.Fatal(src.manifestErr)
	}
	if err := src.TestConnection(context.Background(), filament.NewConfig(map[string]any{"ingestion": filament.IngestionBoth})); err != nil {
		t.Fatalf("webhook-only connection test: %v", err)
	}
}

// A connection that reads only webhooks needs none of the manifest's API
// config, makes no probe request, and cannot run API resources.
func TestWebhookOnlyConnectionNeedsNoAPIConfig(t *testing.T) {
	ctx := context.Background()
	src := catalogSource(t, "posthog@unversioned")
	if err := src.Validate(filament.NewConfig(map[string]any{})); err == nil {
		t.Fatal("API connection validated without api_key")
	}
	webhooksOnly := filament.NewConfig(map[string]any{"ingestion": filament.IngestionWebhooks})
	if err := src.Validate(webhooksOnly); err != nil {
		t.Fatalf("webhook-only validate: %v", err)
	}
	if err := src.TestConnection(ctx, webhooksOnly); err != nil {
		t.Fatalf("webhook-only connection test: %v", err)
	}
	if err := src.Configure(ctx, webhooksOnly); err != nil {
		t.Fatalf("webhook-only configure: %v", err)
	}
	res, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil || len(res.Resources) != 1 || res.Resources[0].Name != "events_webhook" {
		t.Fatalf("webhook-only discover = %+v, %v", res.Resources, err)
	}
	err = src.Extract(ctx, &collectSink{}, filament.ExtractOpts{Resources: []string{"events"}})
	if err == nil || !strings.Contains(err.Error(), "reads only webhooks") {
		t.Fatalf("webhook-only extract: %v", err)
	}
	for _, field := range src.Spec().Config.Fields {
		if field.Scope == filament.ScopePipeline && field.VisibleWhen != nil {
			t.Fatalf("pipeline field %s depends on the connection's ingestion field", field.Name)
		}
	}
	if !src.Spec().Webhook.Enabled(webhooksOnly) || src.Spec().Webhook.Enabled(filament.NewConfig(map[string]any{})) {
		t.Fatal("spec ingestion does not follow the connection config")
	}
}

// Metronome creates each webhook's signing secret, so its spec says so and
// Filament does not mint one. PostHog accepts the key Filament generates.
func TestCatalogWebhookSecretIssuer(t *testing.T) {
	if got := catalogSource(t, "metronome@v2").Spec().Webhook.SecretIssuer; got != filament.WebhookSecretProvider {
		t.Fatalf("metronome secret issuer = %q", got)
	}
	if got := catalogSource(t, "posthog@unversioned").Spec().Webhook; got.ProviderIssuesSecret() {
		t.Fatalf("posthog secret issuer = %q", got.SecretIssuer)
	}
}
