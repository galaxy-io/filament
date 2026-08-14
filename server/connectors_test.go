package server

import (
	"context"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/registry"
)

type columnSourceCounts struct {
	configured atomic.Int32
	tornDown   atomic.Int32
}

type columnSource struct{ counts *columnSourceCounts }

func (s *columnSource) Spec() filament.ConnectorSpec   { return filament.ConnectorSpec{Name: "columns"} }
func (s *columnSource) Validate(filament.Config) error { return nil }
func (s *columnSource) Configure(context.Context, filament.Config) error {
	s.counts.configured.Add(1)
	return nil
}

func (s *columnSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}

func (s *columnSource) Teardown(context.Context) error {
	s.counts.tornDown.Add(1)
	return nil
}

func (s *columnSource) CursorColumns(_ context.Context, resource string) ([]filament.CursorColumn, error) {
	return []filament.CursorColumn{{
		SchemaField: filament.SchemaField{Name: resource + "_updated_at"},
		Eligible:    true, Configurable: true, SupportsLookback: true,
	}}, nil
}

type liveProbeSink struct {
	probes *atomic.Int32
	err    error
}

func (s *liveProbeSink) Spec() filament.SinkSpec                      { return filament.SinkSpec{Name: "live-sink"} }
func (s *liveProbeSink) Open(context.Context, filament.RunSpec) error { return nil }
func (s *liveProbeSink) Apply(context.Context, filament.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, nil
}
func (s *liveProbeSink) Commit(context.Context) error { return nil }
func (s *liveProbeSink) Abort(context.Context) error  { return nil }
func (s *liveProbeSink) Name() string                 { return "live-sink" }
func (s *liveProbeSink) TestConnection(context.Context, filament.Config) error {
	s.probes.Add(1)
	return s.err
}

func TestValidateConfigRunsLiveSinkProbe(t *testing.T) {
	probes := &atomic.Int32{}
	sinks := registry.NewSinks()
	sinks.Register("live-sink", func() filament.Sink {
		return &liveProbeSink{probes: probes, err: context.DeadlineExceeded}
	})
	api := New(registry.NewSources(), sinks, memory.New(), nil, nil)

	response, err := api.ValidateConfig(context.Background(), connect.NewRequest(&ingestionv1.ValidateConfigRequest{
		Connector: "live-sink",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
		Live:      true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetValid() {
		t.Fatalf("response = %#v, want validation failure", response.Msg)
	}
	if got := probes.Load(); got != 1 {
		t.Fatalf("live probes = %d, want 1", got)
	}
}

func TestGetConnector(t *testing.T) {
	sources := registry.NewSources()
	sources.RegisterWithMaturity("columns", filament.MaturityBeta, func() filament.Source {
		return &columnSource{counts: &columnSourceCounts{}}
	})
	api := New(sources, registry.NewSinks(), memory.New(), nil, nil)

	response, err := api.GetConnector(context.Background(), connect.NewRequest(&ingestionv1.GetConnectorRequest{
		Connector: "columns",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := response.Msg.GetConnector().GetName(); got != "columns" {
		t.Fatalf("connector name = %q, want %q", got, "columns")
	}
	if got := response.Msg.GetConnector().GetMaturity(); got != ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA {
		t.Fatalf("connector maturity = %v, want beta", got)
	}

	_, err = api.GetConnector(context.Background(), connect.NewRequest(&ingestionv1.GetConnectorRequest{
		Connector: "missing",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
	}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("unknown connector error = %v, want not found", err)
	}

	_, err = api.GetConnector(context.Background(), connect.NewRequest(&ingestionv1.GetConnectorRequest{
		Connector: "columns",
	}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("unspecified kind error = %v, want invalid argument", err)
	}
}

func TestListConnectorsIncludesConnectorMaturity(t *testing.T) {
	sources := registry.NewSources()
	sources.RegisterWithMaturity("columns", filament.MaturityStable, func() filament.Source {
		return &columnSource{counts: &columnSourceCounts{}}
	})
	api := New(sources, registry.NewSinks(), memory.New(), nil, nil)

	response, err := api.ListConnectors(context.Background(), connect.NewRequest(&ingestionv1.ListConnectorsRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(response.Msg.GetConnectors()); got != 1 {
		t.Fatalf("connectors = %d, want 1", got)
	}
	if got := response.Msg.GetConnectors()[0].GetMaturity(); got != ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_STABLE {
		t.Fatalf("connector maturity = %v, want stable", got)
	}
}

func TestSinkConnectorResponsesIncludeMaturity(t *testing.T) {
	sinks := registry.NewSinks()
	sinks.RegisterWithMaturity("live-sink", filament.MaturityBeta, func() filament.Sink {
		return &liveProbeSink{probes: &atomic.Int32{}}
	})
	api := New(registry.NewSources(), sinks, memory.New(), nil, nil)

	getResponse, err := api.GetConnector(context.Background(), connect.NewRequest(&ingestionv1.GetConnectorRequest{
		Connector: "live-sink",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := getResponse.Msg.GetConnector().GetMaturity(); got != ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA {
		t.Fatalf("get connector maturity = %v, want beta", got)
	}

	listResponse, err := api.ListConnectors(context.Background(), connect.NewRequest(&ingestionv1.ListConnectorsRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(listResponse.Msg.GetConnectors()); got != 1 {
		t.Fatalf("connectors = %d, want 1", got)
	}
	if got := listResponse.Msg.GetConnectors()[0].GetMaturity(); got != ingestionv1.ConnectorMaturity_CONNECTOR_MATURITY_BETA {
		t.Fatalf("list connector maturity = %v, want beta", got)
	}
}

func TestGetResourceColumnsBatchesOneConfiguredSource(t *testing.T) {
	counts := &columnSourceCounts{}
	sources := registry.NewSources()
	sources.Register("columns", func() filament.Source { return &columnSource{counts: counts} })
	api := New(sources, registry.NewSinks(), memory.New(), nil, nil)

	response, err := api.GetResourceColumns(context.Background(), connect.NewRequest(&ingestionv1.GetResourceColumnsRequest{
		Connector: "columns",
		Resources: []string{"orders", "users"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := counts.configured.Load(); got != 1 {
		t.Fatalf("configured = %d, want 1", got)
	}
	if got := counts.tornDown.Load(); got != 1 {
		t.Fatalf("torn down = %d, want 1", got)
	}
	if got := len(response.Msg.GetResources()); got != 2 {
		t.Fatalf("resource groups = %d, want 2", got)
	}
	if response.Msg.GetResources()[0].GetColumns()[0].GetName() != "orders_updated_at" {
		t.Fatalf("first response = %#v", response.Msg.GetResources()[0])
	}
	column := response.Msg.GetResources()[0].GetColumns()[0]
	if !column.GetConfigurable() || !column.GetSupportsLookback() {
		t.Fatalf("cursor capabilities did not round trip: %#v", column)
	}
}
