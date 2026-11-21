package incremental

import (
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func numericTracker(t *testing.T, initial string, overlap int) *Tracker {
	t.Helper()
	tr, err := New(manifest.IncrementalSpec{
		CursorField:    "id",
		StartParam:     "since_id",
		InjectInto:     "query",
		Comparator:     "numeric",
		OverlapSeconds: overlap,
	}, "things", initial)
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

func TestNumericOverlapExactPast2To53(t *testing.T) {
	// 9007199254740993 - 10 must stay exact: the float path rounds it back
	// to 9007199254740992 and the overlap window vanishes.
	if got := numericTracker(t, "9007199254740993", 10).Start(); got != "9007199254740983" {
		t.Errorf("Start() = %s, want 9007199254740983", got)
	}
	// Ordinary integers subtract as before.
	if got := numericTracker(t, "1000", 10).Start(); got != "990" {
		t.Errorf("Start() = %s, want 990", got)
	}
	// A decimal watermark keeps the float path.
	if got := numericTracker(t, "100.5", 10).Start(); got != "90.5" {
		t.Errorf("Start() = %s, want 90.5", got)
	}
	// No overlap, no change.
	if got := numericTracker(t, "9007199254740993", 0).Start(); got != "9007199254740993" {
		t.Errorf("Start() = %s, want the watermark itself", got)
	}
}
