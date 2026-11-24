package compile

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// namedRoute is a route with one edge per resource. "" is an all-resources edge.
func namedRoute(resources ...string) *routeGroup {
	tables := map[string]*routeTable{}
	for _, resource := range resources {
		tables[resource] = &routeTable{named: true, selector: resource}
	}
	return &routeGroup{tables: tables}
}

func TestReplicationStreamContinuityFingerprintIgnoresResources(t *testing.T) {
	group := &routeGroup{
		source: &ingestionv1.PipelineNode{ConnectionId: "source"},
		sink:   &ingestionv1.PipelineNode{ConnectionId: "sink"},
		tables: map[string]*routeTable{"customers": {named: true, writeMode: filament.WriteMerge}},
	}
	connections := map[string]filament.Connection{
		"source": {ID: "source", Version: 3},
		"sink":   {ID: "sink", Version: 7},
	}
	sink := filament.Ref{Connector: "postgres", Config: map[string]any{"schema": "raw"}}
	sourceContinuity := map[string]any{"schema": "public", "publication": "filament"}

	initial, err := replicationStreamContinuityFingerprint(group, connections, "postgres", sourceContinuity, sink)
	if err != nil {
		t.Fatal(err)
	}
	group.tables["orders"] = &routeTable{named: true, writeMode: filament.WriteMerge}
	compatible, err := replicationStreamContinuityFingerprint(group, connections, "postgres", sourceContinuity, sink)
	if err != nil {
		t.Fatal(err)
	}
	if compatible != initial {
		t.Fatalf("compatible edit changed fingerprint: %q != %q", compatible, initial)
	}

	sourceContinuity["publication"] = "other_publication"
	incompatible, err := replicationStreamContinuityFingerprint(group, connections, "postgres", sourceContinuity, sink)
	if err != nil {
		t.Fatal(err)
	}
	if incompatible == initial {
		t.Fatal("publication change should fork replication continuity")
	}
}

func TestCompileIngestionTypesPerResourceWriteModes(t *testing.T) {
	group := &routeGroup{
		from: "src", to: "snk",
		tables: map[string]*routeTable{
			"orders": {named: true, readMode: filament.ModeIncremental, writeMode: filament.WriteUpsert},
			"audit":  {named: true, readMode: filament.ModeFull, writeMode: filament.WriteReplace},
		},
	}
	types, err := compileIngestionTypes(group, false)
	if err != nil {
		t.Fatal(err)
	}
	if types["orders"] != filament.IngestionIncrementalUpsert || types["audit"] != filament.IngestionFullReplace {
		t.Fatalf("ingestion types = %v", types)
	}
	if _, err := compileIngestionTypes(group, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("mixed CDC route: got %v, want ErrInvalid", err)
	}
}

func TestContinuityFingerprintLeavesMembershipOut(t *testing.T) {
	source := filament.Ref{Connector: "postgres", Config: map[string]any{"dsn": "x"}}
	sink := filament.Ref{Connector: "postgres", Config: map[string]any{"schema": "raw"}}
	plan := filament.ContinuousWritePlan{}
	continuity := map[string]any{"schema": "public"}
	initial, err := continuityFingerprint(source, sink, plan, continuity)
	if err != nil {
		t.Fatal(err)
	}
	continuity["schema"] = "other"
	if forked, err := continuityFingerprint(source, sink, plan, continuity); err != nil || forked == initial {
		t.Fatalf("source continuity must fork the stream: %q %v", forked, err)
	}
}

func TestResyncMustNameARouteResource(t *testing.T) {
	groups := []*routeGroup{namedRoute("orders", "customers")}
	if err := validateResync(groups, []string{"orders"}); err != nil {
		t.Fatal(err)
	}
	if err := validateResync(groups, []string{"invoices"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown resource: %v", err)
	}
	if got := routeResync(groups[0], []string{"customers", "orders"}); len(got) != 2 {
		t.Fatalf("route resync = %v", got)
	}
}

func TestResourcesMustNameARouteResource(t *testing.T) {
	groups := []*routeGroup{namedRoute("orders", "customers")}
	if err := validateResources(groups, []string{"orders"}, []string{"orders"}); err != nil {
		t.Fatal(err)
	}
	if err := validateResources(groups, []string{"invoices"}, nil); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown resource: %v", err)
	}
	if err := validateResources(groups, []string{"orders"}, []string{"customers"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("resync outside the subset: %v", err)
	}
}

func TestPartialRouteComparesSelectedResources(t *testing.T) {
	full := namedRoute("orders", "customers")
	if isPartialRoute(namedRoute("orders", "customers"), full) {
		t.Fatal("every resource selected is not partial")
	}
	if !isPartialRoute(namedRoute("orders"), full) {
		t.Fatal("one of two resources is partial")
	}
	if !isPartialRoute(namedRoute("orders"), namedRoute("")) {
		t.Fatal("an all-resources route run by name is partial")
	}
}
