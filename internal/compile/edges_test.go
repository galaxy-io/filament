package compile

import (
	"errors"
	"slices"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"

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
	if len(groups) != 1 || len(groups[0].columns()["orders"]) != 2 || groups[0].columns()["items"] != nil {
		t.Fatalf("columns = %v", groups[0].columns())
	}
	if got := routeSelection(groups[0].columns())["orders"]; got[0] != "email" || got[1] != "id" {
		t.Fatalf("route columns must be sorted so order never changes a fingerprint: %v", got)
	}

	reordered, err := groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Columns: selection("email", "id")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", Columns: selection("id", "email")},
	}, nodes)
	if err != nil || len(reordered[0].columns()["orders"]) != 2 {
		t.Fatalf("same columns in another order must not conflict: %v", err)
	}

	plain, err := groupEdges([]*ingestionv1.PipelineEdge{{FromNode: "src", ToNode: "snk", Resource: "orders"}}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if routeSelection(plain[0].columns()) != nil {
		t.Fatalf("a route without selections must carry none: %v", routeSelection(plain[0].columns()))
	}

	_, err = groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Columns: selection("id", "email")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", Columns: selection("id")},
	}, nodes)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("conflicting selections: %v", err)
	}
}

func TestGroupEdgesCollectsPartitionSelections(t *testing.T) {
	nodes := map[string]*ingestionv1.PipelineNode{"src": {Id: "src"}, "snk": {Id: "snk"}}
	selection := func(partitions ...string) []*ingestionv1.ResourcePartitionSelection {
		return []*ingestionv1.ResourcePartitionSelection{{Resource: "orders", Partitions: partitions}}
	}
	groups, err := groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Partitions: selection("orders_2026_11", "orders_2026_10")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", Partitions: selection("orders_2026_10", "orders_2026_11")},
	}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if got := routeSelection(groups[0].partitions())["orders"]; len(got) != 2 || got[0] != "orders_2026_10" {
		t.Fatalf("route partitions must be sorted: %v", got)
	}

	_, err = groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Partitions: selection("orders_2026_10")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", Partitions: selection("orders_2026_11")},
	}, nodes)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("conflicting partition selections: %v", err)
	}

	none, err := groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Resource: "orders", Partitions: selection()},
	}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := routeSelection(none[0].partitions())["orders"]; !ok || got == nil || len(got) != 0 {
		t.Fatalf("an empty selection must stay an empty selection, got %v (present %v)", got, ok)
	}
}

func TestGroupEdgesCollectsSinkConfigs(t *testing.T) {
	nodes := map[string]*ingestionv1.PipelineNode{"src": {Id: "src"}, "snk": {Id: "snk"}}
	sinkConfig := func(mode string) []*ingestionv1.ResourceSinkConfig {
		config, err := structpb.NewStruct(map[string]any{"update_mode": mode})
		if err != nil {
			t.Fatal(err)
		}
		return []*ingestionv1.ResourceSinkConfig{{Resource: "orders", Config: config}}
	}
	groups, err := groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", SinkConfigs: sinkConfig("merge-on-read")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", SinkConfigs: sinkConfig("merge-on-read")},
		{FromNode: "src", ToNode: "snk", Resource: "items"},
	}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	configs := routeSinkConfigs(groups[0].sinkConfigs())
	if len(configs) != 1 || configs["orders"]["update_mode"] != "merge-on-read" {
		t.Fatalf("sink configs = %v", configs)
	}

	plain, err := groupEdges([]*ingestionv1.PipelineEdge{{FromNode: "src", ToNode: "snk", Resource: "orders"}}, nodes)
	if err != nil {
		t.Fatal(err)
	}
	if routeSinkConfigs(plain[0].sinkConfigs()) != nil {
		t.Fatalf("a route without sink configs must carry none: %v", routeSinkConfigs(plain[0].sinkConfigs()))
	}

	_, err = groupEdges([]*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", SinkConfigs: sinkConfig("merge-on-read")},
		{FromNode: "src", ToNode: "snk", Resource: "orders", SinkConfigs: sinkConfig("copy-on-write")},
	}, nodes)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("conflicting sink configs: %v", err)
	}
}

func TestSubsetRunKeepsAllResourcesEdgeSettings(t *testing.T) {
	nodes := map[string]*ingestionv1.PipelineNode{"src": {Id: "src"}, "snk": {Id: "snk"}, "other": {Id: "other"}}
	config, err := structpb.NewStruct(map[string]any{"partition_by": []any{"day(created_at)"}})
	if err != nil {
		t.Fatal(err)
	}
	steps, err := structpb.NewStruct(map[string]any{"version": 1, "resources": map[string]any{
		"orders": map[string]any{"steps": []any{map[string]any{"drop": []any{"note"}}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	edges := []*ingestionv1.PipelineEdge{
		{
			FromNode: "src", ToNode: "snk", Transform: steps,
			Cursors:     []*ingestionv1.ResourceCursorConfig{{Resource: "orders", Field: "created_at"}},
			Columns:     []*ingestionv1.ResourceColumnSelection{{Resource: "orders", Columns: []string{"id", "created_at"}}},
			Partitions:  []*ingestionv1.ResourcePartitionSelection{{Resource: "orders", Partitions: []string{"orders_2026"}}},
			SinkConfigs: []*ingestionv1.ResourceSinkConfig{{Resource: "orders", Config: config}, {Resource: "items", Config: config}},
		},
		{FromNode: "src", ToNode: "snk", Resource: "orders"},
		{FromNode: "src", ToNode: "other", Resource: "orders"},
	}
	groups, err := groupEdges(edges, nodes)
	if err != nil {
		t.Fatal(err)
	}
	selected, _, err := subsetGroups(groups, []string{"orders"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	route, other := selected[0], selected[1]
	if route.cursorConfigs()["orders"].Field != "created_at" {
		t.Fatalf("cursors = %v", route.cursorConfigs())
	}
	if !slices.Equal(route.columns()["orders"], []string{"created_at", "id"}) {
		t.Fatalf("columns = %v", route.columns())
	}
	if !slices.Equal(route.partitions()["orders"], []string{"orders_2026"}) {
		t.Fatalf("partitions = %v", route.partitions())
	}
	if len(route.sinkConfigs()) != 1 || route.sinkConfigs()["orders"]["partition_by"] == nil {
		t.Fatalf("sink configs = %v", route.sinkConfigs())
	}
	if len(route.transforms()) != 1 || route.transforms()["orders"] == nil {
		t.Fatalf("transforms = %v", route.transforms())
	}
	if len(other.columns())+len(other.partitions())+len(other.sinkConfigs())+len(other.cursorConfigs())+len(other.transforms()) != 0 {
		t.Fatalf("a route declaring no settings must carry none: %+v", other.tables["orders"])
	}
}

func TestSubsetGroupsKeepTheNamedResources(t *testing.T) {
	nodes := map[string]*ingestionv1.PipelineNode{"src": {Id: "src"}, "snk": {Id: "snk"}, "other": {Id: "other"}}
	edges := []*ingestionv1.PipelineEdge{
		{FromNode: "src", ToNode: "snk", Resource: "orders"},
		{FromNode: "src", ToNode: "snk", Resource: "items"},
		{FromNode: "src", ToNode: "other", Resource: "customers"},
		{FromNode: "src", ToNode: "other"},
	}
	full, err := groupEdges(edges, nodes)
	if err != nil {
		t.Fatal(err)
	}
	groups, _, err := subsetGroups(full, []string{"orders"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatalf("a route with nothing selected must not run: %d groups", len(groups))
	}
	if resources, _ := routeResources(groups[0]); len(resources) != 1 || resources[0] != "orders" {
		t.Fatalf("resources = %v", resources)
	}
	if resources, _ := routeResources(full[0]); len(resources) != 2 {
		t.Fatalf("subsetGroups must not change the full route: %v", resources)
	}
}
