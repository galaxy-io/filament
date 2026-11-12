package runner

import (
	"testing"

	"github.com/galaxy-io/filament"
)

type plainSource struct{ filament.Source }

func (plainSource) Spec() filament.ConnectorSpec { return filament.ConnectorSpec{Name: "plain"} }

type selectingSource struct {
	plainSource
	columns map[string][]string
}

func (s *selectingSource) SelectColumns(columns map[string][]string) error {
	s.columns = columns
	return nil
}

func TestSelectColumnsRequiresSourceSupport(t *testing.T) {
	if err := SelectColumns(plainSource{}, nil); err != nil {
		t.Fatalf("no selection must be a no-op: %v", err)
	}
	if err := SelectColumns(plainSource{}, map[string][]string{"orders": {"id"}}); err == nil {
		t.Fatal("a selection on a source without support must fail")
	}
	src := &selectingSource{}
	if err := SelectColumns(src, map[string][]string{"orders": {"id"}}); err != nil || len(src.columns["orders"]) != 1 {
		t.Fatalf("selection not handed over: %v %v", err, src.columns)
	}
}
