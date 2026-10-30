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
