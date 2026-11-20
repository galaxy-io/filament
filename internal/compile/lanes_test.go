package compile

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

type specWorker struct {
	filament.Worker
	specs map[string]filament.ConnectorSpec
}

func (w specWorker) SourceSpec(_ context.Context, name string) (filament.ConnectorSpec, error) {
	return w.specs[name], nil
}

func TestExecutionEdgesSplitsBySourceResource(t *testing.T) {
	c := &Compiler{Worker: specWorker{specs: map[string]filament.ConnectorSpec{
		"posthog": {Webhook: &filament.WebhookSpec{Resources: []string{"events_webhook"}}},
		"orders":  {},
	}}}
	nodes := map[string]*ingestionv1.PipelineNode{
		"ph": {Id: "ph", ConnectionId: "c-ph"}, "db": {Id: "db", ConnectionId: "c-db"},
	}
	connections := map[string]filament.Connection{"c-ph": {Connector: "posthog"}, "c-db": {Connector: "orders"}}
	edges := []*ingestionv1.PipelineEdge{
		{FromNode: "ph", ToNode: "snk", Resource: "persons"},
		{FromNode: "ph", ToNode: "snk", Resource: "events_webhook"},
		{FromNode: "db", ToNode: "snk", Resource: "orders"},
	}
	names := func(edges []*ingestionv1.PipelineEdge) (out []string) {
		for _, e := range edges {
			out = append(out, e.GetResource())
		}
		return out
	}

	bounded, err := c.executionEdges(context.Background(), edges, nodes, connections, filament.ExecutionBounded)
	if err != nil || len(bounded) != 2 || names(bounded)[0] != "persons" || names(bounded)[1] != "orders" {
		t.Fatalf("bounded: %v, %v", names(bounded), err)
	}
	continuous, err := c.executionEdges(context.Background(), edges, nodes, connections, filament.ExecutionContinuous)
	if err != nil || len(continuous) != 2 || names(continuous)[0] != "events_webhook" || names(continuous)[1] != "orders" {
		t.Fatalf("continuous: %v, %v", names(continuous), err)
	}
	if _, err := c.executionEdges(context.Background(), edges[1:2], nodes, connections, filament.ExecutionBounded); !errors.Is(err, ErrInvalid) {
		t.Fatalf("webhook-only bounded start: %v", err)
	}
}

func TestContinuousRouteKeySuffixesSplittingSources(t *testing.T) {
	split := filament.ConnectorSpec{Webhook: &filament.WebhookSpec{Resources: []string{"events_webhook"}}}
	if got := ContinuousRouteKey(split, "route/a/b"); got != "route/a/b/continuous" {
		t.Fatalf("split: %s", got)
	}
	dynamic := filament.ConnectorSpec{Webhook: &filament.WebhookSpec{DynamicResources: true}}
	if got := ContinuousRouteKey(dynamic, "route/a/b"); got != "route/a/b" {
		t.Fatalf("dynamic: %s", got)
	}
}
