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
	if got := routeColumns(groups[0])["orders"]; got[0] != "email" || got[1] != "id" {
		t.Fatalf("route columns must be sorted so order never changes a fingerprint: %v", got)
	}

	reordered, err := groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Columns: selection("email", "id")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", Columns: selection("id", "email")},
	}, nodes)
	if err != nil || len(reordered[0].columns["orders"]) != 2 {
		t.Fatalf("same columns in another order must not conflict: %v", err)
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

func TestSelectEdgesKeepsTheNamedResources(t *testing.T) {
	nodes := map[string]*ingestionv1.PipelineNode{"src": {Id: "src"}, "snk": {Id: "snk"}, "other": {Id: "other"}}
	edges := []*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Resource: "orders"},
		{FromNode: "src", ToNode: "snk", Resource: "items"},
		{FromNode: "src", ToNode: "other", Resource: "customers"},
		{FromNode: "src", ToNode: "other"},
	}
	groups, err := groupEdges(selectEdges(edges, []string{"orders"}), nodes)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("a route with nothing selected must not run: %d groups", len(groups))
	}
	if resources, _ := routeResources(groups[0]); len(resources) != 1 || resources[0] != "orders" {
		t.Fatalf("resources = %v", resources)
	}
	if len(edges) != 4 {
		t.Fatal("selectEdges must not change its input")
	}
}
