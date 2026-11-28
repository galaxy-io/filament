package incremental

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func endSpec(inject string) manifest.IncrementalSpec {
	return manifest.IncrementalSpec{
		CursorField:     "time",
		StartParam:      "from_date",
		EndParam:        "to_date",
		InjectInto:      inject,
		Comparator:      "time",
		ValueDateFormat: "2006-01-02",
		OverlapSeconds:  86400,
	}
}

func fixedClock(t *testing.T, value string) Option {
	t.Helper()
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return WithClock(func() time.Time { return at })
}

// Mixpanel's raw export takes a whole-day window and rejects a request
// without to_date, so the end bound rides along with the start on every
// request target.
func TestTrackerEndParamOnEveryTarget(t *testing.T) {
	for _, inject := range []string{"query", "header", "body", "state"} {
		t.Run(inject, func(t *testing.T) {
			tr, err := New(endSpec(inject), "events", "2026-11-20T08:00:00Z", fixedClock(t, "2026-11-26T03:04:05+05:30"))
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"from_date": "2026-11-19", "to_date": "2026-11-25"}
			scope := tr.Scope()
			for param, value := range want {
				if scope[param] != value {
					t.Fatalf("scope[%s] = %q, want %q (scope %v)", param, scope[param], value, scope)
				}
			}
			req := httptest.NewRequest("GET", "https://data.mixpanel.com/api/2.0/export", nil)
			overrides, err := tr.Apply(req)
			if err != nil {
				t.Fatal(err)
			}
			for param, value := range want {
				var got string
				switch inject {
				case "query":
					got = req.URL.Query().Get(param)
				case "header":
					got = req.Header.Get(param)
				case "body":
					got, _ = overrides[param].(string)
				case "state":
					got = value // state is read through Scope only
				}
				if got != value {
					t.Fatalf("%s %s = %q, want %q", inject, param, got, value)
				}
			}
		})
	}
}

// The end bound is fixed when the tracker is created, so every page of one
// extraction asks for the same window even when the run crosses midnight.
func TestTrackerEndParamIsFixedForTheExtraction(t *testing.T) {
	now := time.Date(2026, 11, 25, 23, 59, 59, 0, time.UTC)
	tr, err := New(endSpec("query"), "events", "2026-11-20T00:00:00Z", WithClock(func() time.Time { return now }))
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	if _, err := tr.ObserveChecked(map[string]any{"time": "2026-11-25T23:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	if got := tr.Scope()["to_date"]; got != "2026-11-25" {
		t.Fatalf("to_date = %q, want the bound fixed at creation", got)
	}
}

// A first run with no checkpoint and no initial still sends the end bound,
// and an RFC3339 end is sent as is without the start's prefix or suffix.
func TestTrackerEndParamWithoutStartOrFormat(t *testing.T) {
	spec := endSpec("query")
	spec.ValueDateFormat = ""
	spec.OverlapSeconds = 0
	spec.ValuePrefix = "ge"
	tr, err := New(spec, "events", "", fixedClock(t, "2026-11-26T06:13:16.5Z"))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "https://example.com/export", nil)
	if _, err := tr.Apply(req); err != nil {
		t.Fatal(err)
	}
	if got := req.URL.Query(); got.Has("from_date") || got.Get("to_date") != "2026-11-26T06:13:16Z" {
		t.Fatalf("query = %v, want only the end bound", got)
	}
}

func TestTrackerWithoutEndParamIsUnchanged(t *testing.T) {
	spec := endSpec("query")
	spec.EndParam = ""
	tr, err := New(spec, "events", "2026-11-20T08:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if scope := tr.Scope(); len(scope) != 1 || scope["from_date"] != "2026-11-19" {
		t.Fatalf("scope = %v, want only the start bound", scope)
	}
	empty, err := New(spec, "events", "")
	if err != nil {
		t.Fatal(err)
	}
	if scope := empty.Scope(); scope != nil {
		t.Fatalf("scope = %v, want nil without any bound", scope)
	}
}

func TestTrackerEndParamRequiresTimeComparator(t *testing.T) {
	spec := endSpec("query")
	spec.Comparator = "numeric"
	spec.ValueDateFormat = ""
	spec.OverlapSeconds = 0
	if _, err := New(spec, "events", ""); err == nil {
		t.Fatal("end_param with a numeric comparator was accepted")
	}
}

// Mixpanel rejects a to_date after today in the project's time zone. At 00:30
// UTC it is still the previous day in US time zones, so a one-day lag keeps
// the end on a date every zone has reached.
func TestTrackerEndLagMovesTheEndBack(t *testing.T) {
	spec := endSpec("query")
	spec.EndLagSeconds = 86400
	tr, err := New(spec, "events", "", fixedClock(t, "2026-11-27T00:30:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.Scope()["to_date"]; got != "2026-11-26" {
		t.Fatalf("to_date = %q, want the previous day", got)
	}

	spec.EndLagSeconds = 0
	tr, err = New(spec, "events", "", fixedClock(t, "2026-11-27T00:30:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if got := tr.Scope()["to_date"]; got != "2026-11-27" {
		t.Fatalf("to_date without lag = %q, want the run date", got)
	}
}

func TestTrackerEndLagRequiresEndParam(t *testing.T) {
	spec := endSpec("query")
	spec.EndParam, spec.EndLagSeconds = "", 86400
	if _, err := New(spec, "events", ""); err == nil {
		t.Fatal("end_lag_seconds without end_param was accepted")
	}
	spec = endSpec("query")
	spec.EndLagSeconds = -1
	if _, err := New(spec, "events", ""); err == nil {
		t.Fatal("negative end_lag_seconds was accepted")
	}
}

// A window-only tracker sends the window but never observes records, so a
// full refresh neither advances nor filters on the watermark.
func TestTrackerWindowOnly(t *testing.T) {
	spec := endSpec("query")
	spec.Initial, spec.OverlapSeconds = "2026-11-01T00:00:00Z", 0
	tr, err := New(spec, "events", "", WindowOnly(), fixedClock(t, "2026-11-27T12:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	scope := tr.Scope()
	if scope["from_date"] != "2026-11-01" || scope["to_date"] != "2026-11-27" {
		t.Fatalf("scope = %v, want the initial to run-date window", scope)
	}
	advanced, err := tr.ObserveChecked(map[string]any{})
	if advanced || err != nil {
		t.Fatalf("ObserveChecked = %v, %v; want no-op", advanced, err)
	}
	if tr.Current() != "" {
		t.Fatalf("Current = %q, want none", tr.Current())
	}
	if below, err := tr.Below("2020-01-01T00:00:00Z"); below || err != nil {
		t.Fatalf("Below = %v, %v; want nothing filtered", below, err)
	}
}
