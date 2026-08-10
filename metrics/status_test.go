package metrics

import (
	"strconv"
	"testing"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// TestStatusMappingCoversEveryProtoValue guards the DIMENSION_STATUS mapping
// against a new run status landing on UNSPECIFIED. The two enums are mapped
// here and again in server/convert.go, so adding a status means editing both;
// miss this one and a grouped metrics response silently buckets that status as
// UNSPECIFIED — no compile error, no other test failure.
//
// Driven by the generated RunStatus_name rather than a hand-written list, so a
// new proto value fails this test the moment it is generated.
func TestStatusMappingCoversEveryProtoValue(t *testing.T) {
	for value := range ingestionv1.RunStatus_name {
		proto := ingestionv1.RunStatus(value)
		if proto == ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED {
			continue
		}
		domain, err := statusFromProto(proto)
		if err != nil {
			t.Errorf("statusFromProto(%v): %v", proto, err)
			continue
		}
		if got := statusToProto(domain); got != proto {
			t.Errorf("round trip %v -> %v -> %v, want %v", proto, domain, got, proto)
		}
	}
}

// TestStatusFilterValueRoundTrip covers the string encoding the metrics wire
// actually carries: a filter value arrives as an ingestionv1 ordinal in decimal
// and must land on the domain ordinal the runs.status column stores, which
// keyToProtoStatus then re-encodes for the response key.
func TestStatusFilterValueRoundTrip(t *testing.T) {
	for value := range ingestionv1.RunStatus_name {
		proto := ingestionv1.RunStatus(value)
		if proto == ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED {
			continue
		}
		wire := strconv.Itoa(int(proto))
		stored, err := statusFilterValue(wire)
		if err != nil {
			t.Errorf("statusFilterValue(%s): %v", wire, err)
			continue
		}
		if got := keyToProtoStatus(stored); got != wire {
			t.Errorf("round trip %v: stored %q -> key %q, want %q", proto, stored, got, wire)
		}
	}
}
