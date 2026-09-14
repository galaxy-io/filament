package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
)

const completionManifest = `
version: 1
name: test
display_name: Test
description: Generic incremental request fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  archived: {type: bool, default: false}
  properties: {type: string, default: "name, custom_value"}
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    method: GET
    query: {archived: "{{ config.archived }}"}
    records: $.results
    primary_key: [id]
    fields:
      id: string
      updated_at: timestamptz
    incremental:
      cursor_field: updated_at
      start_param: filters.0.value
      end_param: filters.1.value
      inject_into: body
      comparator: time
      initial: "1970-01-01T00:00:00Z"
      overlap_seconds: 60
      checkpoint_on_complete: true
      disabled_when: archived
      request:
        path: /items/search
        method: POST
        query: {}
        body:
          template:
            properties: '{{ config.properties | split "," }}'
            filters:
              - {field: updated_at, operator: GTE, value: ""}
              - {field: updated_at, operator: LT, value: ""}
              - {field: id, operator: GT, value: "0"}
        pagination:
          cursor:
            response: results.-1.id
            request: body.filters.2.value
`

func TestIncrementalCompletionPaginationAndRestart(t *testing.T) {
	const count = 10050
	const initial = "1970-01-01T00:00:00Z"
	const newest = "2026-01-20T00:00:00Z"
	stamps := make([]string, count)
	for i := range stamps {
		stamps[i] = "2026-01-10T00:00:00Z"
	}
	stamps[0] = newest // The largest timestamp arrives before older rows.
	fail := true
	var lower, upper string
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/items/search" || r.URL.RawQuery != "" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
			w.WriteHeader(400)
			return
		}
		var body struct {
			Properties []string `json:"properties"`
			Filters    []struct {
				Value string `json:"value"`
			} `json:"filters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if !reflect.DeepEqual(body.Properties, []string{"name", "custom_value"}) {
			t.Errorf("properties = %v", body.Properties)
		}
		if len(body.Filters) != 3 {
			t.Error("missing filters")
			w.WriteHeader(400)
			return
		}
		if calls == 0 {
			lower, upper = body.Filters[0].Value, body.Filters[1].Value
		}
		calls++
		if lower != body.Filters[0].Value || upper != body.Filters[1].Value {
			t.Error("time bounds changed between pages")
		}
		start, e1 := time.Parse(time.RFC3339Nano, lower)
		end, e2 := time.Parse(time.RFC3339Nano, upper)
		if e1 != nil || e2 != nil || !start.Before(end) {
			t.Error("invalid time bounds")
			w.WriteHeader(400)
			return
		}
		after, err := strconv.Atoi(body.Filters[2].Value)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if fail && after >= 400 {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"message":"interrupted"}`)
			return
		}
		rows := []map[string]string{}
		total := 0
		for i := after; i < count; i++ {
			stamp, _ := time.Parse(time.RFC3339, stamps[i])
			if stamp.Before(start) || !stamp.Before(end) {
				continue
			}
			total++
			if len(rows) < 200 {
				rows = append(rows, map[string]string{"id": strconv.Itoa(i + 1), "updated_at": stamps[i]})
			}
		}
		// Offset pagination could not walk beyond 10,000. Every request here
		// instead asks for the first page after a new ID, including tied times.
		json.NewEncoder(w).Encode(map[string]any{"results": rows, "total": total})
	}))
	defer api.Close()
	src := NewManifest([]byte(strings.Replace(completionManifest, "https://example.com\nresources:", api.URL+"\nresources:", 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(context.Background())
	plan, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var partial collectSink
	opts := filament.ExtractOpts{Resources: []string{"items"}}
	if err = src.ExtractFrom(t.Context(), &partial, opts, plan); err == nil {
		t.Fatal("expected interrupted extraction")
	}
	if len(partial.records) != 399 {
		t.Fatalf("partial rows = %d", len(partial.records))
	}
	for _, row := range partial.records {
		if !reflect.DeepEqual(row.Key, []string{initial}) {
			t.Fatalf("premature checkpoint: %v", row.Key)
		}
	}
	prior := map[string]filament.Checkpoint{"items": checkpoint.KeysetCheckpoint{Mode: checkpoint.ModeIncremental, Cols: []string{"updated_at"}, Types: []string{"timestamptz"}, Shards: []checkpoint.KeysetShard{{Key: partial.records[len(partial.records)-1].Key}}}.ToCheckpoint("items")}
	plan, err = src.PlanIncremental(t.Context(), []string{"items"}, prior, nil)
	if err != nil {
		t.Fatal(err)
	}
	fail = false
	calls = 0
	var complete collectSink
	if err = src.ExtractFrom(t.Context(), &complete, opts, plan); err != nil {
		t.Fatal(err)
	}
	if len(complete.records) != count {
		t.Fatalf("got %d rows, want %d", len(complete.records), count)
	}
	for i, row := range complete.records {
		if row.ID != strconv.Itoa(i+1) {
			t.Fatalf("row %d ID = %s", i, row.ID)
		}
		want := initial
		if i == count-1 {
			want = newest
		}
		if !reflect.DeepEqual(row.Key, []string{want}) {
			t.Fatalf("row %d checkpoint = %v, want %s", i, row.Key, want)
		}
	}
	prior["items"] = checkpoint.KeysetCheckpoint{Mode: checkpoint.ModeIncremental, Cols: []string{"updated_at"}, Types: []string{"timestamptz"}, Shards: []checkpoint.KeysetShard{{Key: complete.records[count-1].Key}}}.ToCheckpoint("items")
	for _, mutate := range []bool{false, true} {
		if mutate {
			stamps[count-1] = "2026-01-21T00:00:00Z"
		}
		plan, err = src.PlanIncremental(t.Context(), []string{"items"}, prior, nil)
		if err != nil {
			t.Fatal(err)
		}
		calls = 0
		var next collectSink
		if err = src.ExtractFrom(t.Context(), &next, opts, plan); err != nil {
			t.Fatal(err)
		}
		want := 1
		if mutate {
			want = 2
		}
		if len(next.records) != want {
			t.Fatalf("mutate=%v: got %d rows, want %d", mutate, len(next.records), want)
		}
		if lower != "2026-01-19T23:59:00Z" {
			t.Fatalf("lower = %s", lower)
		}
	}
}

func TestIncrementalRequestPreservesFullAndArchivedReads(t *testing.T) {
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/items" || r.URL.Query().Get("archived") != "true" {
			t.Errorf("unexpected full request: %s %s", r.Method, r.URL)
		}
		fmt.Fprint(w, `{"results":[{"id":"1","updated_at":"2026-01-01T00:00:00Z"}]}`)
	}))
	defer api.Close()
	src := NewManifest([]byte(strings.Replace(completionManifest, "https://example.com\nresources:", api.URL+"\nresources:", 1)))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"archived": true})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(context.Background())
	columns, err := src.CursorColumns(t.Context(), "items")
	if err != nil || len(columns) != 0 {
		t.Fatalf("archived cursor columns = %v, %v", columns, err)
	}
	if _, err = src.PlanIncremental(t.Context(), []string{"items"}, nil, nil); err == nil {
		t.Fatal("expected archived incremental rejection")
	}
	var sink collectSink
	if err = src.Extract(t.Context(), &sink, filament.ExtractOpts{Resources: []string{"items"}}); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || len(sink.records) != 1 {
		t.Fatalf("calls=%d records=%d", calls, len(sink.records))
	}
}

func TestIncrementalCompletionWaitsForAllParents(t *testing.T) {
	text := strings.Replace(completionManifest, "resources:\n", `resources:
  - name: groups
    path: /groups
    records: $.results
    capture_only: true
    capture: {group: id}
`, 1)
	text = strings.Replace(text, "    path: /items\n", "    path: /items/{group}\n    params: {group: parent.group}\n    for_each: groups\n", 1)
	text = strings.Replace(text, "        path: /items/search", "        path: /items/{group}/search", 1)
	firstDone := make(chan struct{})
	fail := true
	bounds := make(chan string, 10)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/groups" {
			fmt.Fprint(w, `{"results":[{"id":"a"},{"id":"b"}]}`)
			return
		}
		var body struct {
			Filters []struct {
				Value string `json:"value"`
			} `json:"filters"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		bounds <- body.Filters[1].Value
		if r.URL.Path == "/items/b/search" && fail {
			<-firstDone
			w.WriteHeader(400)
			return
		}
		if body.Filters[2].Value != "0" {
			fmt.Fprint(w, `{"results":[]}`)
			if r.URL.Path == "/items/a/search" && fail {
				close(firstDone)
			}
			return
		}
		if r.URL.Path == "/items/a/search" {
			fmt.Fprint(w, `{"results":[{"id":"1","updated_at":"2026-01-20T00:00:00Z"},{"id":"2","updated_at":"2026-01-20T00:00:00Z"}]}`)
		} else {
			fmt.Fprint(w, `{"results":[{"id":"3","updated_at":"2026-01-10T00:00:00Z"},{"id":"4","updated_at":"2026-01-10T00:00:00Z"}]}`)
		}
	}))
	defer api.Close()
	text = strings.Replace(text, "https://example.com\nresources:", api.URL+"\nresources:", 1)
	src := NewManifest([]byte(text))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(context.Background())
	plan, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var partial collectSink
	opts := filament.ExtractOpts{Resources: []string{"items"}}
	if err = src.ExtractFrom(t.Context(), &partial, opts, plan); err == nil {
		t.Fatal("expected failed parent")
	}
	for _, row := range partial.records {
		if !reflect.DeepEqual(row.Key, []string{"1970-01-01T00:00:00Z"}) {
			t.Fatalf("parent advanced shared checkpoint: %v", row.Key)
		}
	}
	bound := <-bounds
	for len(bounds) > 0 {
		if got := <-bounds; got != bound {
			t.Fatalf("parents had different upper bounds: %s and %s", bound, got)
		}
	}
	fail = false
	var complete collectSink
	if err = src.ExtractFrom(t.Context(), &complete, opts, plan); err != nil {
		t.Fatal(err)
	}
	if len(complete.records) != 4 {
		t.Fatalf("rows=%d", len(complete.records))
	}
	seen := map[string]bool{}
	for i, row := range complete.records {
		seen[row.ID] = true
		want := "1970-01-01T00:00:00Z"
		if i == 3 {
			want = "2026-01-20T00:00:00Z"
		}
		if !reflect.DeepEqual(row.Key, []string{want}) {
			t.Fatalf("row %d checkpoint %v", i, row.Key)
		}
	}
	if len(seen) != 4 {
		t.Fatal("missing records after failed parent")
	}
}
