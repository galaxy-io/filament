package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

const endParamManifest = `version: 1
name: windowed
display_name: Windowed
description: Whole-day export window fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection: {base_url: BASE}
resources:
  - name: events
    path: /export
    mode: stream
    stream: {type: ndjson}
    primary_key: [insert_id]
    query:
      from_date: '{{ state.from_date }}'
      to_date: '{{ state.to_date }}'
    fields:
      insert_id: {path: properties.insert_id, type: string}
      time: {path: properties.time, type: timestamptz}
    incremental:
      cursor_field: time
      start_param: from_date
      end_param: to_date
      inject_into: state
      comparator: time
      initial: '2026-11-01T00:00:00Z'
      overlap_seconds: 86400
      value_date_format: '2006-01-02'
`

// An incremental read of a whole-day export API gets both ends of the window
// from the tracker. The start rewinds a day for late events and the end is the
// run's own date.
func TestEndParamSendsTheWindowEnd(t *testing.T) {
	var seen url.Values
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query()
		fmt.Fprintln(w, `{"event":"a","properties":{"insert_id":"1","time":"2026-11-02T10:00:00Z"}}`)
		fmt.Fprintln(w, `{"event":"b","properties":{"insert_id":"2","time":"2026-11-03T10:00:00Z"}}`)
	}))
	defer api.Close()

	before := time.Now().UTC().Format("2006-01-02")
	src := NewManifest([]byte(strings.ReplaceAll(endParamManifest, "BASE", api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	sink := &scanSink{}
	opts := extractOptions{Resources: []string{"events"}, IncrementalResources: map[string]bool{"events": true}}
	if err := src.connector.Extract(t.Context(), sink, opts); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC().Format("2006-01-02")

	if got := seen.Get("from_date"); got != "2026-10-31" {
		t.Fatalf("from_date = %q, want the initial rewound by the overlap", got)
	}
	if got := seen.Get("to_date"); got != before && got != after {
		t.Fatalf("to_date = %q, want today (%s)", got, before)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want 2", len(sink.records))
	}
}

func TestEndParamValidation(t *testing.T) {
	valid := strings.ReplaceAll(endParamManifest, "BASE", "https://example.com")
	if err := NewManifest([]byte(valid)).manifestErr; err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for _, tc := range []struct{ name, old, new, want string }{
		{"same as start", "end_param: to_date", "end_param: from_date", "must differ from start_param"},
		{"lex comparator", "      comparator: time\n", "      comparator: lex\n", "requires comparator time"},
		{"lag without end", "      end_param: to_date\n", "      end_lag_seconds: 86400\n", "requires end_param"},
		{"negative lag", "      end_param: to_date\n", "      end_param: to_date\n      end_lag_seconds: -1\n", "end_lag_seconds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := strings.Replace(valid, tc.old, tc.new, 1)
			if invalid == valid {
				t.Fatalf("replacement %q did not apply", tc.old)
			}
			err := NewManifest([]byte(invalid)).manifestErr
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

// A full refresh still sends the window: an API that requires the end date
// rejects a read without it. The start is the initial value without the
// overlap, and the run observes no watermark.
func TestEndParamFullRefreshSendsTheWindow(t *testing.T) {
	var seen url.Values
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query()
		fmt.Fprintln(w, `{"event":"a","properties":{"insert_id":"1","time":"2026-11-02T10:00:00Z"}}`)
	}))
	defer api.Close()

	src := NewManifest([]byte(strings.ReplaceAll(endParamManifest, "BASE", api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	sink := &scanSink{}
	opts := extractOptions{Resources: []string{"events"}, IncrementalResources: map[string]bool{}}
	if err := src.connector.Extract(t.Context(), sink, opts); err != nil {
		t.Fatal(err)
	}
	if got := seen.Get("from_date"); got != "2026-11-01" {
		t.Fatalf("from_date = %q, want the initial value", got)
	}
	if seen.Get("to_date") == "" {
		t.Fatal("full refresh sent no to_date")
	}
	if len(sink.records) != 1 {
		t.Fatalf("records = %d, want 1", len(sink.records))
	}
	// A record of an incremental read carries the running watermark as its
	// key. A window-only tracker has none to carry.
	if key := sink.records[0].Key; len(key) != 0 {
		t.Fatalf("full refresh record key = %v, want no watermark", key)
	}
}

// A full refresh of a resource without end_param keeps sending no window.
func TestFullRefreshWithoutEndParamSendsNoWindow(t *testing.T) {
	var seen url.Values
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.URL.Query()
		fmt.Fprintln(w, `{"event":"a","properties":{"insert_id":"1","time":"2026-11-02T10:00:00Z"}}`)
	}))
	defer api.Close()

	data := strings.NewReplacer(
		"BASE", api.URL,
		"      end_param: to_date\n", "",
		"      inject_into: state\n", "      inject_into: query\n",
		"    query:\n      from_date: '{{ state.from_date }}'\n      to_date: '{{ state.to_date }}'\n", "",
	).Replace(endParamManifest)
	src := NewManifest([]byte(data))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	opts := extractOptions{Resources: []string{"events"}, IncrementalResources: map[string]bool{}}
	if err := src.connector.Extract(t.Context(), &scanSink{}, opts); err != nil {
		t.Fatal(err)
	}
	if seen.Get("from_date") != "" || seen.Get("to_date") != "" {
		t.Fatalf("full refresh sent a window: %v", seen)
	}
}

// Mixpanel's export carries event time as Unix seconds. An incremental read
// with a numeric time cursor advances the watermark instead of failing on
// the first record.
func TestEndParamWithEpochCursor(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"event":"a","properties":{"insert_id":"1","time":1795736494}}`)
		fmt.Fprintln(w, `{"event":"b","properties":{"insert_id":"2","time":1795822894}}`)
	}))
	defer api.Close()

	src := NewManifest([]byte(strings.ReplaceAll(endParamManifest, "BASE", api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	sink := &scanSink{}
	opts := extractOptions{Resources: []string{"events"}, IncrementalResources: map[string]bool{"events": true}}
	if err := src.connector.Extract(t.Context(), sink, opts); err != nil {
		t.Fatal(err)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want 2", len(sink.records))
	}
	if got := strings.Join(sink.records[1].Key, ","); got != "2026-11-27T23:41:34Z" {
		t.Fatalf("watermark after the second record = %q, want the epoch as RFC3339", got)
	}
}

// A window with no events reads as zero records when the manifest names
// Mixpanel's empty-export body, and fails as a malformed line without it.
func TestStreamEmptyBodyOnAnEmptyWindow(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "terminated early")
	}))
	defer api.Close()

	for _, tc := range []struct {
		name, stream string
		wantErr      bool
	}{
		{"with empty_body", "stream: {type: ndjson, empty_body: terminated early}", false},
		{"without empty_body", "stream: {type: ndjson}", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := strings.NewReplacer("BASE", api.URL, "stream: {type: ndjson}", tc.stream).Replace(endParamManifest)
			src := NewManifest([]byte(data))
			if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
				t.Fatal(err)
			}
			defer src.Teardown(t.Context())
			sink := &scanSink{}
			opts := extractOptions{Resources: []string{"events"}, IncrementalResources: map[string]bool{"events": true}}
			err := src.connector.Extract(t.Context(), sink, opts)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, want error %v", err, tc.wantErr)
			}
			if len(sink.records) != 0 {
				t.Fatalf("records = %d, want 0", len(sink.records))
			}
		})
	}
}

func TestStreamEmptyBodyValidation(t *testing.T) {
	valid := strings.ReplaceAll(endParamManifest, "BASE", "https://example.com")
	for _, tc := range []struct{ name, stream, want string }{
		{"sse", "stream: {type: sse, empty_body: done}", "requires stream type ndjson"},
		{"padded", `stream: {type: ndjson, empty_body: " done"}`, "whitespace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			invalid := strings.Replace(valid, "stream: {type: ndjson}", tc.stream, 1)
			err := NewManifest([]byte(invalid)).manifestErr
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}
