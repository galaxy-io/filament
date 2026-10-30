package compile

import (
	"testing"

	"github.com/galaxy-io/filament"
)

// A change stream names its ingestion per resource so the runner shapes the
// sink and lineage like a bounded CDC run. A message stream names none.
func TestContinuousIngestionTypesFollowReplication(t *testing.T) {
	cdc := filament.ConnectorSpec{SourcePolicies: []filament.SourcePolicy{{Mode: filament.ModeCDC}}}
	ref := filament.Ref{Config: map[string]any{"replication": "cdc"}}
	resources := []string{"orders", "items"}

	merge := continuousIngestionTypes(cdc, ref, resources, filament.WriteMerge)
	if len(merge) != 2 || merge["orders"] != filament.IngestionCDCMerge || merge["items"] != filament.IngestionCDCMerge {
		t.Fatalf("merge: %v", merge)
	}
	if history := continuousIngestionTypes(cdc, ref, resources, filament.WriteAppend); history["orders"] != filament.IngestionCDCAppend {
		t.Fatalf("append: %v", history)
	}
	if got := continuousIngestionTypes(cdc, filament.Ref{}, resources, filament.WriteMerge); got != nil {
		t.Fatalf("standard replication: %v", got)
	}
	if got := continuousIngestionTypes(filament.ConnectorSpec{}, ref, resources, filament.WriteMerge); got != nil {
		t.Fatalf("message source: %v", got)
	}
}
