package arrowbatch

import (
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

func TestControlOwnership(t *testing.T) {
	c := rowmodel.Control{Kind: rowmodel.ProgressBoundary, Domain: rowmodel.DomainKey{Incarnation: "source", Domain: "log"}, Position: rowmodel.Position{Codec: "opaque", Value: []byte("position")}}
	b, err := NewControl(c)
	if err != nil {
		t.Fatal(err)
	}
	c.Position.Value[0] = 'X'
	if b.Drained || b.Rows() != nil || b.NumRows() != 0 || b.Bytes() != 0 || string(b.Control.Position.Value) != "position" {
		t.Fatalf("invalid control batch: %+v", b)
	}
	b.Retain()
	b.Release()
	if b.Control == nil {
		t.Fatal("retained control released early")
	}
	b.Release()
	if b.Control != nil {
		t.Fatal("control not released")
	}
	marker := NewMarker()
	defer marker.Release()
	if !marker.Drained || marker.Control != nil {
		t.Fatal("bounded marker changed")
	}
}

func TestControlRejectsInvalid(t *testing.T) {
	if b, err := NewControl(rowmodel.Control{}); err == nil || b != nil {
		t.Fatal("accepted invalid control")
	}
}
