package compile

import (
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestReplicationStreamContinuityFingerprintIgnoresResources(t *testing.T) {
	group := &routeGroup{
		source:     &ingestionv1.PipelineNode{ConnectionId: "source"},
		sink:       &ingestionv1.PipelineNode{ConnectionId: "sink"},
		resources:  map[string]bool{"customers": true},
		writeModes: map[string]filament.WriteMode{"customers": filament.WriteMerge},
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
	group.resources["orders"] = true
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
		readModes:  map[string]filament.ReadMode{"orders": filament.ModeIncremental, "audit": filament.ModeFull},
		writeModes: map[string]filament.WriteMode{"orders": filament.WriteUpsert, "audit": filament.WriteReplace},
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
	groups := []*routeGroup{{resources: map[string]bool{"orders": true, "customers": true}}}
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
	groups := []*routeGroup{{resources: map[string]bool{"orders": true, "customers": true}}}
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
	full := &routeGroup{resources: map[string]bool{"orders": true, "customers": true}}
	if isPartialRoute(&routeGroup{resources: map[string]bool{"orders": true, "customers": true}}, full) {
		t.Fatal("every resource selected is not partial")
	}
	if !isPartialRoute(&routeGroup{resources: map[string]bool{"orders": true}}, full) {
		t.Fatal("one of two resources is partial")
	}
	if !isPartialRoute(&routeGroup{resources: map[string]bool{"orders": true}}, &routeGroup{all: true}) {
		t.Fatal("an all-resources route run by name is partial")
	}
}
