package compile

import (
	"errors"
	"testing"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestGroupEdgesCollectsColumnSelections(t *testing.T) {
	nodes := map[string]*ingestionv1.PipelineNode{"src": {Id: "src"}, "snk": {Id: "snk"}}
	selection := func(columns ...string) []*ingestionv1.ResourceColumnSelection {
		return []*ingestionv1.ResourceColumnSelection{{Resource: "orders", Columns: columns}}
	}
	groups, err := groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Resource: "orders", Columns: selection("id", "email")},
		{FromNode: "src", ToNode: "snk", Resource: "items"},
	}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || len(groups[0].columns["orders"]) != 2 || groups[0].columns["items"] != nil {
		t.Fatalf("columns = %v", groups[0].columns)
	}
	if routeColumns(groups[0])["orders"][1] != "email" {
		t.Fatalf("route columns = %v", routeColumns(groups[0]))
	}

	plain, err := groupEdges([]*ingestionv1.PipelineEdge{{FromNode: "src", ToNode: "snk", Resource: "orders"}}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if routeColumns(plain[0]) != nil {
		t.Fatalf("a route without selections must carry none: %v", routeColumns(plain[0]))
	}

	_, err = groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Columns: selection("id", "email")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", Columns: selection("id")},
	}, nodes)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("conflicting selections: %v", err)
	}
}
