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

// Jira JQL rejects RFC3339 but accepts an unquoted epoch-milliseconds
// number. value_date_format renders the watermark for the wire only; the
// checkpoint and comparisons keep the raw RFC3339 value.
func TestTrackerValueDateFormatEpochMillis(t *testing.T) {
	spec := manifest.IncrementalSpec{
		CursorField:     "updated",
		StartParam:      "jql",
		InjectInto:      "body",
		Comparator:      "time",
		ValueDateFormat: "epoch_millis",
		ValuePrefix:     "updated >= ",
		ValueSuffix:     " ORDER BY updated ASC",
	}
	tr, err := New(spec, "issues", "2024-11-06T12:00:00Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	if got := tr.Scope()["jql"]; got != "updated >= 1730894400000 ORDER BY updated ASC" {
		t.Fatalf("scope = %q, want epoch-millis JQL", got)
	}
	req := httptest.NewRequest("POST", "https://example.atlassian.net/rest/api/3/search/jql", nil)
	overrides, err := tr.Apply(req)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got, _ := overrides["jql"].(string); got != "updated >= 1730894400000 ORDER BY updated ASC" {
		t.Fatalf("body override = %q, want epoch-millis JQL", got)
	}
	if _, err := tr.ObserveChecked(map[string]any{"updated": "2024-11-07T12:00:00Z"}); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if got := tr.Current(); got != "2024-11-07T12:00:00Z" {
		t.Fatalf("checkpoint = %q, want the raw advanced watermark", got)
	}
}

// A Go layout reformats the wire value the same way, and the format is
// rejected without the time comparator.
func TestTrackerValueDateFormatLayoutAndComparatorGuard(t *testing.T) {
	spec := manifest.IncrementalSpec{
		CursorField:     "updated",
		StartParam:      "since",
		InjectInto:      "query",
		Comparator:      "time",
		ValueDateFormat: "2006-01-02 15:04",
	}
	tr, err := New(spec, "issues", "2024-11-06T12:34:56Z")
	if err != nil {
		t.Fatalf("new tracker: %v", err)
	}
	if got := tr.Scope()["since"]; got != "2024-11-06 12:34" {
		t.Fatalf("scope = %q, want layout-formatted watermark", got)
	}
	spec.Comparator = "lex"
	if _, err := New(spec, "issues", "2024-11-06T12:34:56Z"); err == nil {
		t.Fatal("new tracker with value_date_format and lex comparator: want error, got nil")
	}
}

// A layout with no Go reference-time element, such as YYYY-MM-DD, would
// render the same literal string for every watermark. New rejects it so a
// typo fails at setup instead of silently sending a literal to the API.
func TestTrackerValueDateFormatRejectsBadLayout(t *testing.T) {
	for _, format := range []string{"YYYY-MM-DD", "updated"} {
		spec := manifest.IncrementalSpec{
			CursorField:     "updated",
			StartParam:      "since",
			InjectInto:      "query",
			Comparator:      "time",
			ValueDateFormat: format,
		}
		if _, err := New(spec, "issues", "2024-11-06T12:34:56Z"); err == nil {
			t.Fatalf("new tracker with value_date_format %q: want error, got nil", format)
		}
	}
}

// An initial watermark that cannot be formatted with the configured wire
// format fails at setup instead of reaching the API as raw RFC3339.
func TestTrackerValueDateFormatRejectsUnformattableInitial(t *testing.T) {
	spec := manifest.IncrementalSpec{
		CursorField:     "updated",
		StartParam:      "since",
		InjectInto:      "query",
		Comparator:      "time",
		ValueDateFormat: "epoch_millis",
	}
	if _, err := New(spec, "issues", "not-a-time"); err == nil {
		t.Fatal("new tracker with unformattable initial watermark: want error, got nil")
	}
}
