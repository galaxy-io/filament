package filament_test

import (
	"testing"

	"github.com/galaxy-io/filament"
)

func TestResourceExecutionSplitsFixedWebhookResources(t *testing.T) {
	split := filament.ConnectorSpec{Webhook: &filament.WebhookSpec{Resources: []string{"events_webhook"}}}
	for resource, want := range map[string]filament.ExecutionMode{
		"events_webhook": filament.ExecutionContinuous,
		"persons":        filament.ExecutionBounded,
		"":               filament.ExecutionBounded,
	} {
		if got := filament.ResourceExecution(split, resource, filament.ExecutionContinuous); got != want {
			t.Errorf("%q: got %s, want %s", resource, got, want)
		}
	}
	// A source that does not split runs every resource in the pipeline's mode.
	for name, spec := range map[string]filament.ConnectorSpec{
		"no webhooks": {},
		"dynamic":     {Webhook: &filament.WebhookSpec{DynamicResources: true}},
	} {
		if got := filament.ResourceExecution(spec, "events_webhook", filament.ExecutionContinuous); got != filament.ExecutionContinuous {
			t.Errorf("%s: got %s", name, got)
		}
		if got := filament.ResourceExecution(spec, "events_webhook", filament.ExecutionBounded); got != filament.ExecutionBounded {
			t.Errorf("%s: got %s", name, got)
		}
	}
}
