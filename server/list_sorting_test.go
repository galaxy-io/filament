package server

import (
	"testing"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/worker"
)

func TestListConnectorsEmptySortingPreservesFirstPage(t *testing.T) {
	sources := registry.NewSources()
	for _, name := range []string{"alpha", "zulu"} {
		sources.Register(name, func() filament.Source { return &catalogSource{name: name} })
	}
	api := New(worker.Local(sources, registry.NewSinks()), nil, nil, nil)
	for _, sorting := range []*ingestionv1.SortingRequest{nil, {}, {SortBy: ingestionv1.SortBy_SORT_BY_NAME}} {
		response, err := api.ListConnectors(t.Context(), connect.NewRequest(&ingestionv1.ListConnectorsRequest{
			Sorting: sorting, Pagination: &ingestionv1.PaginationRequest{PageSize: 1},
		}))
		if err != nil {
			t.Fatal(err)
		}
		if rows := response.Msg.GetConnectors(); len(rows) != 1 || rows[0].GetName() != "alpha" {
			t.Fatalf("sorting %v: first page = %v, want alpha", sorting, rows)
		}
	}
}
