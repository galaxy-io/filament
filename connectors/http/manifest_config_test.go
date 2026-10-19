package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestConfiguredLookupLoopAndSelection(t *testing.T) {
	const data = `version: 1
name: lookups
display_name: Lookups
description: Configured lookup fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  codes: {type: string, default: ''}
  date: {type: string, default: '', format: date}
connection: {base_url: BASE}
resources:
  - name: lookup
    path: /lookup
    required_config: [codes, date]
    restart_on_resume: true
    for_each_config: codes
    primary_key: [code, date]
    query: {code: '{{ parent.codes }}', date: '{{ config.date }}'}
    fields: {code: string, date: date}
    response:
      records: $
      cardinality: one
      strict_records: true
      match: {code: '{{ parent.codes }}', date: '{{ config.date }}'}
  - name: other
    path: /other
    fields: {id: string}
discovery: {mode: static, default_resources: [other]}
`
	for _, mismatch := range []bool{false, true} {
		t.Run(fmt.Sprint(mismatch), func(t *testing.T) {
			var calls int
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				code := r.URL.Query().Get("code")
				if mismatch {
					code = "wrong"
				}
				fmt.Fprintf(w, `{"code":%q,"date":"2026-01-01"}`, code)
			}))
			defer api.Close()
			src := NewManifest([]byte(strings.ReplaceAll(data, "BASE", api.URL)))
			if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
				t.Fatal(err)
			}
			names, err := src.PlanResources(t.Context(), nil, nil)
			if err != nil || len(names) != 1 || names[0] != "other" {
				t.Fatalf("defaults=%v %v", names, err)
			}
			if _, err := src.PlanResources(t.Context(), nil, []string{"lookup"}); err == nil {
				t.Fatal("missing lookup settings accepted")
			}
			if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"codes": "EUR, GBP,EUR", "date": "2026-01-01"})); err != nil {
				t.Fatal(err)
			}
			names, err = src.PlanResources(t.Context(), nil, []string{"lookup"})
			if err != nil || len(names) != 1 || names[0] != "lookup" {
				t.Fatalf("selectors=%v %v", names, err)
			}
			sink := &collectSink{}
			err = src.Extract(t.Context(), sink, filament.ExtractOpts{Selectors: []string{"lookup"}})
			if (err != nil) != mismatch {
				t.Fatalf("err=%v", err)
			}
			if !mismatch && (calls != 2 || len(sink.records) != 2) {
				t.Fatalf("calls=%d records=%d", calls, len(sink.records))
			}
		})
	}
}

const configuredRequestManifest = `version: 1
name: configured
display_name: Configured
description: Configured request fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  environment: {type: enum, enum: [test], default: test}
  account: {type: string, pattern: '^[0-9]+$', requires: [start]}
  start: {type: string, format: date, before: end}
  end: {type: string, format: date}
  size: {type: int, default: 2, minimum: 1, maximum: 10}
connection:
  base_url: https://unused.example.com
  base_url_field: environment
  base_urls: {test: BASE}
resources:
  - name: items
    path: /items
    omit_empty_query: true
    query:
      page: '{{ state.offset }}:{{ state.size }}'
      optional: ''
    fields: {id: string}
    response:
      records: $
      pagination:
        offset: {offset: state.offset, limit: state.size, start: 1, page_size: 1, page_size_from: size}
`

func TestConfiguredEndpointAndOffsetRequests(t *testing.T) {
	var calls int
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("page") != fmt.Sprintf("%d:2", 1+(calls-1)*2) || len(r.URL.Query()) != 1 {
			t.Errorf("unexpected query: %v", r.URL.Query())
		}
		if calls == 1 {
			fmt.Fprint(w, `[{"id":"a"},{"id":"b"}]`)
		} else {
			fmt.Fprint(w, `[]`)
		}
	}))
	defer api.Close()
	src := NewManifest([]byte(strings.ReplaceAll(configuredRequestManifest, "BASE", api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	sink := &collectSink{}
	if err := src.Extract(t.Context(), sink, filament.ExtractOpts{Resources: []string{"items"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(sink.records) != 2 {
		t.Fatalf("calls=%d rows=%d", calls, len(sink.records))
	}
}

func TestManifestConfigConstraints(t *testing.T) {
	src := NewManifest([]byte(strings.ReplaceAll(configuredRequestManifest, "BASE", "https://example.com")))
	for _, values := range []map[string]any{
		{"account": "abc"},
		{"account": "123"},
		{"start": "2026-02-30"},
		{"start": "2026-02-01", "end": "2026-01-01"},
		{"size": 0},
		{"size": 11},
	} {
		if err := src.Validate(filament.NewConfig(values)); err == nil {
			t.Fatalf("accepted %v", values)
		}
	}
	if err := src.Validate(filament.NewConfig(map[string]any{"account": "123", "start": "2026-01-01", "end": "2026-02-01", "size": 10})); err != nil {
		t.Fatal(err)
	}
	for _, change := range [][2]string{
		{"pattern: '^[0-9]+$'", "pattern: '['"},
		{"requires: [start]", "requires: [missing]"},
		{"before: end", "before: missing"},
		{"minimum: 1, maximum: 10", "minimum: 11, maximum: 10"},
		{"page_size_from: size", "page_size_from: account"},
		{"start: 1, page_size", "start: -1, page_size"},
	} {
		invalid := strings.Replace(configuredRequestManifest, change[0], change[1], 1)
		if NewManifest([]byte(strings.ReplaceAll(invalid, "BASE", "https://example.com"))).manifestErr == nil {
			t.Fatalf("accepted %s", change[1])
		}
	}
}
