package atomicwatermark

import "testing"

func TestTimeLessAcceptsColonLessOffset(t *testing.T) {
	var cmp Time
	less, err := cmp.Less("2026-11-15T01:09:44.630+0000", "2026-11-15T01:09:45.630+00:00")
	if err != nil {
		t.Fatalf("Less = %v, want no error", err)
	}
	if !less {
		t.Errorf("Less = false, want true for an earlier colon-less offset")
	}
	less, err = cmp.Less("2026-11-15T01:09:45.630+00:00", "2026-11-15T01:09:44.630+0000")
	if err != nil {
		t.Fatalf("Less = %v, want no error", err)
	}
	if less {
		t.Errorf("Less = true, want false for a later value")
	}
	if _, err := cmp.Less("not a timestamp", "2026-11-15T01:09:44.630+00:00"); err == nil {
		t.Errorf("Less = no error, want an error for an unparseable value")
	}
}
