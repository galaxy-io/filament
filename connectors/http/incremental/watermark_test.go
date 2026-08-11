package incremental

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func numericSpec() manifest.IncrementalSpec {
	return manifest.IncrementalSpec{
		CursorField: "created",
		StartParam:  "created[gte]",
		InjectInto:  "query",
		Comparator:  "numeric",
	}
}

func applied(t *testing.T, tracker *Tracker) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "https://api.example.com/v1/charges?limit=100", nil)
	if _, err := tracker.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	return req.URL.Query().Get("created[gte]")
}

// Observing records must not tighten the filter for the pages that follow.
// Reverse-chronological APIs hand back the max on page 1, so an advancing
// floor would ask page 2 for records newer than the newest one already seen
// and read the empty result as the end of the list.
func TestApplyKeepsRunStartFloorWhileWatermarkAdvances(t *testing.T) {
	tracker, err := New(numericSpec(), "charges", "1700000000")
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	if got := applied(t, tracker); got != "1700000000" {
		t.Fatalf("page 1 floor = %q, want the seed", got)
	}

	if !tracker.Observe(map[string]any{"created": float64(1800000000)}) {
		t.Fatal("watermark did not advance on a newer record")
	}

	if got := applied(t, tracker); got != "1700000000" {
		t.Fatalf("page 2 floor = %q, want the seed unchanged", got)
	}
	if got := tracker.Current(); got != "1800000000" {
		t.Fatalf("current = %q, want the observed max for the next run", got)
	}
}

// A first run has no seed, so no floor is injected at all — the request must
// go out unfiltered rather than with an empty parameter the API would reject.
func TestApplyInjectsNothingWithoutSeed(t *testing.T) {
	tracker, err := New(numericSpec(), "charges", "")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	tracker.Observe(map[string]any{"created": float64(1800000000)})

	req := httptest.NewRequest(http.MethodGet, "https://api.example.com/v1/charges?limit=100", nil)
	if _, err := tracker.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := req.URL.Query()["created[gte]"]; ok {
		t.Fatalf("query = %q, want no floor parameter", req.URL.RawQuery)
	}
}

func TestScopeExposesFrozenFloor(t *testing.T) {
	spec := manifest.IncrementalSpec{
		CursorField:    "updated_at",
		StartParam:     "since",
		InjectInto:     "query",
		Comparator:     "time",
		OverlapSeconds: 3600,
	}
	tracker, err := New(spec, "items", "2026-01-02T12:00:00Z")
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	tracker.Observe(map[string]any{"updated_at": "2026-01-03T12:00:00Z"})

	if got, want := tracker.Scope()["since"], "2026-01-02T11:00:00Z"; got != want {
		t.Fatalf("scope since = %q, want %q (seed minus overlap)", got, want)
	}
}
