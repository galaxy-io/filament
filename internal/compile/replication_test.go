package compile

import (
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestReplicationStreamContinuityFingerprintIgnoresResources(t *testing.T) {
	group := &routeGroup{
		source:    &ingestionv1.PipelineNode{ConnectionId: "source"},
		sink:      &ingestionv1.PipelineNode{ConnectionId: "sink"},
		resources: map[string]bool{"customers": true},
		writeMode: filament.WriteMerge,
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
