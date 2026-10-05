package incremental

import (
	"net/http/httptest"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func prefixSpec() manifest.IncrementalSpec {
	return manifest.IncrementalSpec{
		CursorField: "lastUpdated",
		StartParam:  "_lastUpdated",
		InjectInto:  "query",
		Comparator:  "time",
		ValuePrefix: "ge",
	}
}

// FHIR search prefixes the _lastUpdated comparator onto the value itself
// (`_lastUpdated=ge<ts>`); a bare timestamp carries `eq` semantics and would
// silently return the wrong result set. The prefix applies to the injected
// request value only — the checkpoint and all comparisons keep the raw value.
func TestTrackerValuePrefixOnQueryInjection(t *testing.T) {
	tr, err := New(prefixSpec(), "patients", "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}

	if got := tr.Scope()["_lastUpdated"]; got != "ge2026-09-01T00:00:00Z" {
		t.Fatalf("scope = %q, want the ge-prefixed watermark", got)
	}

	req := httptest.NewRequest("GET", "https://hapi.fhir.org/baseR4/Patient", nil)
	if _, err := tr.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := req.URL.Query().Get("_lastUpdated"); got != "ge2026-09-01T00:00:00Z" {
		t.Fatalf("query _lastUpdated = %q, want ge-prefixed", got)
	}

	if got := tr.Current(); got != "2026-09-01T00:00:00Z" {
		t.Fatalf("checkpoint = %q, want the raw watermark", got)
	}
}

// The running watermark advances on raw values; injection stays prefixed.
func TestTrackerValuePrefixFollowsWatermark(t *testing.T) {
	tr, err := New(prefixSpec(), "patients", "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	if _, err := tr.ObserveChecked(map[string]any{"lastUpdated": "2026-09-02T12:00:00Z"}); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if got := tr.Current(); got != "2026-09-02T12:00:00Z" {
		t.Fatalf("checkpoint = %q, want the raw advanced watermark", got)
	}
	// The request lower bound stays fixed for the extraction, so the
	// injected value keeps the original start, prefixed.
	if got := tr.Scope()["_lastUpdated"]; got != "ge2026-09-01T00:00:00Z" {
		t.Fatalf("scope = %q, want the ge-prefixed start", got)
	}
}

func TestTrackerValuePrefixOnBodyAndHeader(t *testing.T) {
	spec := prefixSpec()

	spec.InjectInto = "body"
	tr, err := New(spec, "patients", "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	req := httptest.NewRequest("POST", "https://example.com/x", nil)
	overrides, err := tr.Apply(req)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, _ := overrides["_lastUpdated"].(string); got != "ge2026-09-01T00:00:00Z" {
		t.Fatalf("body override = %q, want ge-prefixed", got)
	}

	spec.InjectInto = "header"
	tr, err = New(spec, "patients", "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	if _, err := tr.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := req.Header.Get("_lastUpdated"); got != "ge2026-09-01T00:00:00Z" {
		t.Fatalf("header _lastUpdated = %q, want ge-prefixed", got)
	}
}

// Without a prefix, behavior is exactly as before.
func TestTrackerNoValuePrefixIsUnchanged(t *testing.T) {
	spec := prefixSpec()
	spec.ValuePrefix = ""
	tr, err := New(spec, "patients", "2026-09-01T00:00:00Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	if got := tr.Scope()["_lastUpdated"]; got != "2026-09-01T00:00:00Z" {
		t.Fatalf("scope = %q, want the raw watermark", got)
	}
	req := httptest.NewRequest("GET", "https://hapi.fhir.org/baseR4/Patient", nil)
	if _, err := tr.Apply(req); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := req.URL.Query().Get("_lastUpdated"); got != "2026-09-01T00:00:00Z" {
		t.Fatalf("query _lastUpdated = %q, want the raw watermark", got)
	}
}
