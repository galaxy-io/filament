package postgres

import (
	"testing"

	"github.com/jackc/pglogrepl"
)

func TestPostgresCDCBootstrapFloorIncludesBoundaryRecord(t *testing.T) {
	const resource = "customers"
	floor := pglogrepl.LSN(100)
	run := pgCDCRun{floors: map[string]pglogrepl.LSN{resource: floor}}

	if run.pastFloor(resource, floor-1) {
		t.Fatal("record before bootstrap floor must be filtered")
	}
	if !run.pastFloor(resource, floor) {
		t.Fatal("record at bootstrap floor must be replayed")
	}
	if !run.pastFloor(resource, floor+1) {
		t.Fatal("record after bootstrap floor must be replayed")
	}
	if !run.pastFloor("orders", floor-1) {
		t.Fatal("resource without a floor must not be filtered")
	}
}
