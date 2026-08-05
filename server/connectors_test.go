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
