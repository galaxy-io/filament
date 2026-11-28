package incremental

import (
	"encoding/json"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func epochSpec(comparator string) manifest.IncrementalSpec {
	return manifest.IncrementalSpec{
		CursorField: "time",
		CursorPath:  "properties.time",
		StartParam:  "from_date",
		InjectInto:  "query",
		Comparator:  comparator,
	}
}

func epochRecord(v any) map[string]any {
	return map[string]any{"properties": map[string]any{"time": v}}
}

// Mixpanel's raw export carries event time as Unix seconds. A time cursor
// reads a JSON number with the rule timestamptz columns use.
func TestTimeCursorReadsJSONNumbersAsEpoch(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  any
		want string
	}{
		{"seconds", float64(1795822894), "2026-11-27T23:41:34Z"},
		{"fractional seconds", 1795822894.5, "2026-11-27T23:41:34.5Z"},
		{"milliseconds", float64(1795822894123), "2026-11-27T23:41:34.123Z"},
		{"json number", json.Number("1795822894"), "2026-11-27T23:41:34Z"},
		{"int64", int64(1795822894), "2026-11-27T23:41:34Z"},
		{"rfc3339 string", "2026-11-27T23:41:34Z", "2026-11-27T23:41:34Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr, err := New(epochSpec("time"), "events", "")
			if err != nil {
				t.Fatal(err)
			}
			got, err := tr.Cursor(epochRecord(tc.raw))
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("cursor = %q, want %q", got, tc.want)
			}
			if _, err := tr.ObserveChecked(epochRecord(tc.raw)); err != nil {
				t.Fatal(err)
			}
			if tr.Current() != tc.want {
				t.Fatalf("watermark = %q, want %q", tr.Current(), tc.want)
			}
		})
	}
}

// A digit string is not reinterpreted as an epoch: only JSON numbers are.
func TestTimeCursorRejectsDigitStrings(t *testing.T) {
	tr, err := New(epochSpec("time"), "events", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.ObserveChecked(epochRecord("1795822894")); err == nil {
		t.Fatal("digit string accepted as a timestamp")
	}
}

// The numeric comparator still compares the number itself.
func TestNumericCursorKeepsNumbers(t *testing.T) {
	tr, err := New(epochSpec("numeric"), "events", "")
	if err != nil {
		t.Fatal(err)
	}
	got, err := tr.Cursor(epochRecord(float64(1795822894)))
	if err != nil {
		t.Fatal(err)
	}
	if got != "1795822894" {
		t.Fatalf("numeric cursor = %q, want the number", got)
	}
}

// An epoch watermark feeds the whole-day window like a string one.
func TestEpochCursorWindow(t *testing.T) {
	spec := endSpec("query")
	spec.CursorPath = "properties.time"
	first, err := New(spec, "events", "", fixedClock(t, "2026-11-28T12:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.ObserveChecked(epochRecord(float64(1795822894))); err != nil {
		t.Fatal(err)
	}
	next, err := New(spec, "events", first.Current(), fixedClock(t, "2026-11-29T12:00:00Z"))
	if err != nil {
		t.Fatal(err)
	}
	scope := next.Scope()
	if scope["from_date"] != "2026-11-26" || scope["to_date"] != "2026-11-29" {
		t.Fatalf("window = %v, want 2026-11-26..2026-11-29 (one day of overlap)", scope)
	}
}

func TestRecordBelowReadsEpochCursor(t *testing.T) {
	tr, err := New(epochSpec("time"), "events", "2026-11-27T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if below, err := tr.RecordBelow(epochRecord(float64(1795822894))); below || err != nil {
		t.Fatalf("RecordBelow(after start) = %v, %v", below, err)
	}
	if below, err := tr.RecordBelow(epochRecord(float64(1795000000))); !below || err != nil {
		t.Fatalf("RecordBelow(before start) = %v, %v", below, err)
	}
	window, err := New(epochSpec("time"), "events", "", WindowOnly())
	if err != nil {
		t.Fatal(err)
	}
	if below, err := window.RecordBelow(map[string]any{}); below || err != nil {
		t.Fatalf("window-only RecordBelow = %v, %v; want no cursor read", below, err)
	}
}
