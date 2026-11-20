package server

import (
	"context"
	"errors"
	"slices"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/webhooks/stream"
	"github.com/galaxy-io/filament/worker"
)

// splitSource reads its API resources bounded and its "live" webhook
// resource continuously.
type splitSource struct{ leverSource }

func (splitSource) Spec() filament.ConnectorSpec {
	spec := leverSource{}.Spec()
	spec.Name = "splitsource"
	spec.Webhook = &filament.WebhookSpec{Verifier: "standard_webhooks", Resources: []string{"live"}}
	spec.Stream = &filament.StreamCapabilities{
		Input:    filament.InputRows,
		Ordering: []filament.Ordering{filament.OrderingNone},
		Delivery: filament.DeliveryReplayableAtLeastOnce,
	}
	return spec
}

// Validation only inspects the stream capabilities, so a stream is never
// opened and no position is resolved.
func (splitSource) Lookup(string, int) (rowmodel.PositionCodec, error) {
	return nil, errors.New("splitsource: no stream positions")
}

func (splitSource) OpenStream(context.Context, filament.StreamOpenOpts) (filament.StreamSession, error) {
	return nil, errors.New("splitsource: streams are not opened in validation")
}

func (splitSource) PlanReplicationStream(req filament.ReplicationStreamPlanningRequest) (filament.ReplicationStreamPlan, error) {
	return stream.Plan(req)
}

func (splitSource) BindReplicationStream(config map[string]any, admitted filament.ReplicationStream) (map[string]any, error) {
	return stream.Bind(config, admitted)
}

// continuousMemory is the in-memory store with continuous execution turned
// on. SQLite has no stream runtime, and validation only asks whether one
// exists, so the stream methods are never called.
type continuousMemory struct {
	*sqlite.Store
	noStreams
}

type noStreams struct{ filament.ContinuousRunStore }

// One pipeline holds both kinds of resource. Each edge validates in the
// execution mode of its resource. A mixed pipeline saves as bounded.
func TestValidatePipelineResolvesExecutionPerResource(t *testing.T) {
	sources := registry.NewSources()
	sources.Register("splitsource", func() filament.Source { return splitSource{} })
	sinks := registry.NewSinks()
	sinks.Register("leversink", func() filament.Sink { return leverSink{} })
	api := New(worker.Local(sources, sinks), continuousMemory{Store: sqlite.NewMemory()}, nil, nil)
	create := func(kind ingestionv1.ConnectorKind, connector string) string {
		resp, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{Kind: kind, Name: connector, Connector: connector}))
		if err != nil {
			t.Fatal(err)
		}
		return resp.Msg.GetConnection().GetId()
	}
	src, snk := create(ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, "splitsource"), create(ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, "leversink")

	for _, mode := range []ingestionv1.ExecutionMode{ingestionv1.ExecutionMode_EXECUTION_MODE_UNSPECIFIED, ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED} {
		resp, err := api.ValidatePipeline(testCtx(), connect.NewRequest(&ingestionv1.ValidatePipelineRequest{
			ExecutionMode: mode,
			Graph: &ingestionv1.PipelineGraph{
				Nodes: []*ingestionv1.PipelineNode{
					{Id: "src", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, ConnectionId: src},
					{Id: "snk", Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK, ConnectionId: snk},
				},
				Edges: []*ingestionv1.PipelineEdge{
					{FromNode: "src", ToNode: "snk", Resource: "orders"},
					{FromNode: "src", ToNode: "snk", Resource: "live"},
				},
			},
		}))
		if err != nil {
			t.Fatal(err)
		}
		if !resp.Msg.GetValid() {
			t.Fatalf("mode %v: mixed graph is invalid: %s", mode, pipelineValidationMessage(resp.Msg))
		}
		edges := resp.Msg.GetEdges()
		if got := edges[0].GetEffectiveExecutionMode(); got != ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED {
			t.Errorf("pipeline %v: orders runs %v", mode, got)
		}
		if got := edges[0].GetEffectiveReadMode(); got != ingestionv1.ReadMode_READ_MODE_FULL {
			t.Errorf("pipeline %v: orders read mode %v", mode, got)
		}
		if got := edges[1].GetEffectiveExecutionMode(); got != ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS {
			t.Errorf("pipeline %v: live runs %v", mode, got)
		}
	}
}

// ingestingSource is a split source whose connections choose how they read
// it through the "ingestion" field.
type ingestingSource struct{ splitSource }

func (ingestingSource) Spec() filament.ConnectorSpec {
	spec := splitSource{}.Spec()
	spec.Name = "ingestingsource"
	spec.Webhook.IngestionField = "ingestion"
	spec.Config = filament.ConfigSchema{Fields: []filament.ConfigField{{
		Name: "ingestion", Type: filament.FieldEnum, Default: filament.IngestionAPI, Scope: filament.ScopeConnection,
		Enum: []filament.EnumOption{{Value: filament.IngestionAPI}, {Value: filament.IngestionWebhooks}, {Value: filament.IngestionBoth}},
	}}}
	return spec
}

// A connection offers only the execution modes its ingestion choice can run.
func TestConnectionExecutionModesFollowIngestion(t *testing.T) {
	sources := registry.NewSources()
	sources.Register("ingestingsource", func() filament.Source { return ingestingSource{} })
	api := New(worker.Local(sources, registry.NewSinks()), continuousMemory{Store: sqlite.NewMemory()}, nil, nil)
	bounded, continuous := ingestionv1.ExecutionMode_EXECUTION_MODE_BOUNDED, ingestionv1.ExecutionMode_EXECUTION_MODE_CONTINUOUS
	for ingestion, want := range map[string][]ingestionv1.ExecutionMode{
		"":                         {bounded},
		filament.IngestionAPI:      {bounded},
		filament.IngestionWebhooks: {continuous},
		filament.IngestionBoth:     {bounded, continuous},
	} {
		config := map[string]any{}
		if ingestion != "" {
			config["ingestion"] = ingestion
		}
		cfg, err := structpb.NewStruct(config)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
			Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "src " + ingestion, Connector: "ingestingsource", Config: cfg,
		}))
		if err != nil {
			t.Fatalf("ingestion %q: %v", ingestion, err)
		}
		if got := resp.Msg.GetConnection().GetExecutionModes(); !slices.Equal(got, want) {
			t.Errorf("ingestion %q: execution modes %v, want %v", ingestion, got, want)
		}
	}
}
