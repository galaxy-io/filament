package server

import (
	"context"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/arrowbatch"
	sqlitestore "github.com/galaxy-io/filament/datastore/sqlite"
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

type catalogSource struct{ name, displayName, description string }

func (s *catalogSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: s.name, DisplayName: s.displayName, Description: s.description}
}
func (s *catalogSource) Validate(filament.Config) error                   { return nil }
func (s *catalogSource) Configure(context.Context, filament.Config) error { return nil }
func (s *catalogSource) Extract(context.Context, filament.RecordSink, filament.ExtractOpts) error {
	return nil
}
func (s *catalogSource) Teardown(context.Context) error { return nil }

type liveProbeSink struct {
	probes *atomic.Int32
	err    error
}

func (s *liveProbeSink) Spec() filament.SinkSpec                      { return filament.SinkSpec{Name: "live-sink"} }
func (s *liveProbeSink) Open(context.Context, filament.RunSpec) error { return nil }
func (s *liveProbeSink) Apply(context.Context, *arrowbatch.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, nil
}
func (s *liveProbeSink) Commit(context.Context) error { return nil }
func (s *liveProbeSink) Abort(context.Context) error  { return nil }
func (s *liveProbeSink) Name() string                 { return "live-sink" }
func (s *liveProbeSink) TestConnection(context.Context, filament.Config) error {
	s.probes.Add(1)
	return s.err
}

func TestValidateConfigDoesNotRunLiveSinkProbe(t *testing.T) {
	probes := &atomic.Int32{}
	sinks := registry.NewSinks()
	sinks.Register("live-sink", func() filament.Sink {
		return &liveProbeSink{probes: probes, err: context.DeadlineExceeded}
	})
	api := New(registry.NewSources(), sinks, sqlitestore.NewMemory(), nil, nil)

	response, err := api.ValidateConfig(testCtx(), connect.NewRequest(&ingestionv1.ValidateConfigRequest{
		Connector: "live-sink",
		Kind:      ingestionv1.ConnectorKind_CONNECTOR_KIND_SINK,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if !response.Msg.GetValid() {
		t.Fatalf("response = %#v, want valid structural config", response.Msg)
	}
	if got := probes.Load(); got != 0 {
		t.Fatalf("live probes = %d, want 0", got)
	}
}

func TestCanonicalizeDatabaseConnectionConfig(t *testing.T) {
	schema := filament.ConfigSchema{Fields: []filament.ConfigField{
		{Name: "connection_method", Type: filament.FieldEnum, Default: "fields", Scope: filament.ScopeConnection},
		{Name: "dsn", Type: filament.FieldSecret, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"url"}}},
		{Name: "host", Type: filament.FieldString, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"fields"}}},
		{Name: "password", Type: filament.FieldSecret, Scope: filament.ScopeConnection, VisibleWhen: &filament.FieldCondition{Field: "connection_method", Values: []string{"fields"}}},
	}}

	t.Run("legacy dsn infers url", func(t *testing.T) {
		cfg := map[string]any{"host": "stale"}
		refs := map[string]string{"dsn": "filament/tenant/connection/id/dsn/v1", "password": "filament/tenant/connection/id/password/v1"}
		canonicalizeConnectionConfig(schema, cfg, refs)
		if cfg["connection_method"] != "url" || cfg["host"] != nil || refs["password"] != "" {
			t.Fatalf("config = %#v, refs = %#v", cfg, refs)
		}
	})

	t.Run("new config defaults to fields", func(t *testing.T) {
		cfg := map[string]any{"host": "localhost", "dsn": "stale"}
		refs := map[string]string{}
		canonicalizeConnectionConfig(schema, cfg, refs)
		// A supplied DSN is a legacy URL configuration even without the selector.
		if cfg["connection_method"] != "url" {
			t.Fatalf("method = %v, want url", cfg["connection_method"])
		}
		cfg = map[string]any{"host": "localhost"}
		canonicalizeConnectionConfig(schema, cfg, refs)
		if cfg["connection_method"] != "fields" || cfg["host"] != "localhost" {
			t.Fatalf("config = %#v", cfg)
		}
	})

	t.Run("explicit fields removes dsn", func(t *testing.T) {
		cfg := map[string]any{"connection_method": "fields", "host": "localhost", "dsn": "stale"}
		refs := map[string]string{"dsn": "filament/tenant/connection/id/dsn/v1"}
		canonicalizeConnectionConfig(schema, cfg, refs)
		if cfg["dsn"] != nil || refs["dsn"] != "" {
			t.Fatalf("config = %#v, refs = %#v", cfg, refs)
		}
	})
}

func TestGetConnector(t *testing.T) {
	sources := registry.NewSources()
	sources.RegisterWithMaturity("columns", filament.MaturityBeta, func() filament.Source {
		return &columnSource{counts: &columnSourceCounts{}}
	})
	api := New(sources, registry.NewSinks(), sqlitestore.NewMemory(), nil, nil)

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
	api := New(sources, registry.NewSinks(), sqlitestore.NewMemory(), nil, nil)

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

func TestListConnectorsSearchSortAndPage(t *testing.T) {
	sources := registry.NewSources()
	for _, spec := range []catalogSource{
		{name: "alpha", displayName: "Alpha Warehouse", description: "loads orders"},
		{name: "zulu", displayName: "Zulu Warehouse", description: "loads customers"},
		{name: "other", displayName: "Other", description: "billing"},
	} {
		spec := spec
		sources.Register(spec.name, func() filament.Source { return &spec })
	}
	api := New(sources, registry.NewSinks(), sqlitestore.NewMemory(), nil, nil)

	response, err := api.ListConnectors(context.Background(), connect.NewRequest(&ingestionv1.ListConnectorsRequest{
		Kind:       ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE,
		Search:     "ware",
		Pagination: &ingestionv1.PaginationRequest{PageSize: 1},
		Sorting: &ingestionv1.SortingRequest{
			SortBy: ingestionv1.SortBy_SORT_BY_NAME, SortOrder: ingestionv1.SortOrder_SORT_ORDER_DESC,
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if response.Msg.GetPagination().GetTotal() != 2 || len(response.Msg.GetConnectors()) != 1 || response.Msg.GetConnectors()[0].GetName() != "zulu" {
		t.Fatalf("response = %+v", response.Msg)
	}
}

func TestSinkConnectorResponsesIncludeMaturity(t *testing.T) {
	sinks := registry.NewSinks()
	sinks.RegisterWithMaturity("live-sink", filament.MaturityBeta, func() filament.Sink {
		return &liveProbeSink{probes: &atomic.Int32{}}
	})
	api := New(registry.NewSources(), sinks, sqlitestore.NewMemory(), nil, nil)

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
	api := New(sources, registry.NewSinks(), sqlitestore.NewMemory(), nil, nil)

	response, err := api.GetResourceColumns(testCtx(), connect.NewRequest(&ingestionv1.GetResourceColumnsRequest{
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
	if !column.GetIsConfigurable() || !column.GetSupportsLookback() {
		t.Fatalf("cursor capabilities did not round trip: %#v", column)
	}
}
