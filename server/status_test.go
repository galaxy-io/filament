package server

import (
	"testing"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// TestRunStatusMappingCoversEveryProtoValue is the IngestionService half of the
// guard in metrics/status_test.go. runStatusesFromProto drops values it does
// not recognize, so a status missing from that switch silently narrows a
// ListRuns filter instead of failing it.
//
// Driven by the generated RunStatus_name so a new proto value fails here the
// moment it is generated, rather than at the first user report.
func TestRunStatusMappingCoversEveryProtoValue(t *testing.T) {
	for value := range ingestionv1.RunStatus_name {
		proto := ingestionv1.RunStatus(value)
		if proto == ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED {
			continue
		}
		domain := runStatusesFromProto([]ingestionv1.RunStatus{proto})
		if len(domain) != 1 {
			t.Errorf("runStatusesFromProto(%v) dropped the value", proto)
			continue
		}
		if got := runStatusToProto(domain[0]); got != proto {
			t.Errorf("round trip %v -> %v -> %v, want %v", proto, domain[0], got, proto)
		}
	}
}
