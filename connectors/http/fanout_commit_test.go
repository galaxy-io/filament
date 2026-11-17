package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

// fanoutCommitManifest is a parent resource fanned out over two boards with
// an incremental child that only promotes its watermark once the whole
// fan-out completes.
const fanoutCommitManifest = `
version: 1
name: test_fanout
display_name: Test Fanout
description: Fan-out commit_on_complete fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: %s
resources:
  - name: boards
    path: /boards
    primary_key: [id]
    fields:
      id: string
      name: string?
    capture:
      board_id: id
    response:
      records: $
  - name: actions
    path: /actions
    query:
      board: "{{ parent.board_id }}"
    for_each: boards
    primary_key: [board_id, id]
    fields:
      board_id: {path: parent.board_id, type: string}
      id: string
      date: timestamptz
    response:
      records: $
    incremental:
      cursor_field: date
      start_param: since
      inject_into: query
      checkpoint_key: actions_date
      comparator: time
      overlap_seconds: 300
      initial: "1970-01-01T00:00:00Z"
      commit_on_complete: true
`

const fanoutTopLevelManifest = `
version: 1
name: test_top_level
display_name: Test Top Level
description: Top-level commit_on_complete fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: %s
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields:
      id: string
      updated_at: timestamptz
    response:
      records: $.items
      pagination:
        cursor:
          response: next_cursor
          request: query.start_cursor
          more: has_more
    incremental:
      cursor_field: updated_at
      start_param: since
      inject_into: query
      checkpoint_key: items_since
      comparator: time
      commit_on_complete: true
`

func fanoutTestSource(t *testing.T, manifestYAML string, handler http.HandlerFunc) (*Source, *collectSink) {
	t.Helper()
	api := httptest.NewServer(handler)
	t.Cleanup(api.Close)
	src := NewManifest([]byte(fmt.Sprintf(manifestYAML, api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = src.Teardown(context.Background()) })
	return src, &collectSink{}
}

// fanoutExtractIncremental plans the resource incrementally, then extracts
// from the planned checkpoint, mirroring a scheduled incremental run. Plain
// Extract leaves incrementalResources empty, which disables trackers and
// the complete-only guard.
func fanoutExtractIncremental(t *testing.T, src *Source, sink *collectSink, ctx context.Context, resource string) error {
	t.Helper()
	plan, err := src.PlanIncremental(ctx, []string{resource}, nil, nil)
	if err != nil {
		t.Fatalf("plan incremental: %v", err)
	}
	return src.ExtractFrom(ctx, sink, filament.ExtractOpts{Resources: []string{resource}}, plan)
}

func fanoutBoardsHandler(t *testing.T, actions http.HandlerFunc) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/boards" {
			fmt.Fprint(w, `[{"id":"B1","name":"one"},{"id":"B2","name":"two"}]`)
			return
		}
		actions(w, r)
	}
}

func TestFanOutCommitOnCompletePromotesMaxOnlyOnCompletion(t *testing.T) {
	src, sink := fanoutTestSource(t, fanoutCommitManifest, fanoutBoardsHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("board") {
		case "B1":
			fmt.Fprint(w, `[{"id":"a1","date":"2026-01-02T00:00:00Z"},{"id":"a2","date":"2026-01-03T00:00:00Z"}]`)
		case "B2":
			fmt.Fprint(w, `[{"id":"b1","date":"2026-01-04T00:00:00Z"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	if err := fanoutExtractIncremental(t, src, sink, t.Context(), "actions"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(sink.records) != 3 {
		t.Fatalf("records = %d, want 3 actions", len(sink.records))
	}
	for _, rec := range sink.records {
		if rec.Resource != "actions" {
			t.Fatalf("record resource = %q, want actions only", rec.Resource)
		}
		if len(rec.Key) != 0 {
			t.Fatalf("record key = %v, want stripped keys while complete-only", rec.Key)
		}
	}
	if got := sink.checkpoints["actions"]; len(got) != 1 || got[0] != "2026-01-04T00:00:00Z" {
		t.Fatalf("checkpoints = %v, want [2026-01-04T00:00:00Z]", got)
	}
}

func TestFanOutCommitOnCompleteSkipsPromotionOnParentFailure(t *testing.T) {
	src, sink := fanoutTestSource(t, fanoutCommitManifest, fanoutBoardsHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("board") {
		case "B1":
			fmt.Fprint(w, `[{"id":"a1","date":"2026-01-02T00:00:00Z"},{"id":"a2","date":"2026-01-03T00:00:00Z"}]`)
		case "B2":
			// Malformed body: the read fails without retryable status handling.
			fmt.Fprint(w, `{invalid`)
		default:
			http.NotFound(w, r)
		}
	}))
	err := fanoutExtractIncremental(t, src, sink, t.Context(), "actions")
	if err == nil {
		t.Fatalf("extract: want error, got nil")
	}
	// Board B1's records may or may not land before the shared context is
	// cancelled, but no watermark may be promoted either way, so the next
	// run replays the whole window from the old checkpoint.
	if got := sink.checkpoints["actions"]; len(got) != 0 {
		t.Fatalf("checkpoints = %v, want nothing promoted after a failed parent", got)
	}
}

func TestFanOutCommitOnCompleteSkipsPromotionOnPause(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	src, sink := fanoutTestSource(t, fanoutCommitManifest, fanoutBoardsHandler(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("board") {
		case "B1":
			fmt.Fprint(w, `[{"id":"a1","date":"2026-01-02T00:00:00Z"},{"id":"a2","date":"2026-01-03T00:00:00Z"}]`)
			// Simulate a pause arriving mid-fan-out, after B1 was read.
			cancel()
		case "B2":
			// Never answer while the run is live; fail once the pause lands.
			select {
			case <-r.Context().Done():
				http.Error(w, "cancelled", http.StatusInternalServerError)
			case <-time.After(10 * time.Second):
				http.Error(w, "timeout", http.StatusInternalServerError)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	err := fanoutExtractIncremental(t, src, sink, ctx, "actions")
	if err == nil {
		t.Fatalf("extract: want error, got nil")
	}
	if got := sink.checkpoints["actions"]; len(got) != 0 {
		t.Fatalf("checkpoints = %v, want nothing promoted after a paused run", got)
	}
}

func TestTopLevelCommitOnCompleteUnchanged(t *testing.T) {
	pages := map[string]string{
		"":   `{"items":[{"id":"i1","updated_at":"2026-01-02T00:00:00Z"}],"next_cursor":"p2","has_more":true}`,
		"p2": `{"items":[{"id":"i2","updated_at":"2026-01-03T00:00:00Z"}],"next_cursor":"","has_more":false}`,
	}
	src, sink := fanoutTestSource(t, fanoutTopLevelManifest, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, pages[r.URL.Query().Get("start_cursor")])
	})
	if err := fanoutExtractIncremental(t, src, sink, t.Context(), "items"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want 2 items", len(sink.records))
	}
	if got := sink.checkpoints["items"]; len(got) != 1 || got[0] != "2026-01-03T00:00:00Z" {
		t.Fatalf("checkpoints = %v, want [2026-01-03T00:00:00Z]", got)
	}
}

func TestTopLevelCommitOnCompleteSkipsPromotionOnFailure(t *testing.T) {
	src, sink := fanoutTestSource(t, fanoutTopLevelManifest, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("start_cursor") == "" {
			fmt.Fprint(w, `{"items":[{"id":"i1","updated_at":"2026-01-02T00:00:00Z"}],"next_cursor":"p2","has_more":true}`)
			return
		}
		fmt.Fprint(w, `{invalid`)
	})
	err := fanoutExtractIncremental(t, src, sink, t.Context(), "items")
	if err == nil {
		t.Fatalf("extract: want error, got nil")
	}
	if got := sink.checkpoints["items"]; len(got) != 0 {
		t.Fatalf("checkpoints = %v, want nothing promoted after a failed page", got)
	}
}

func TestFanOutCommitOnCompleteValidates(t *testing.T) {
	// A fan-out child with commit_on_complete must load: the manifest in
	// this file already proves it by configuring the source above. A stream
	// child with the flag must still fail validation.
	bad := strings.Replace(fanoutCommitManifest, "    for_each: boards\n", "    mode: stream\n    for_each: boards\n", 1)
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer api.Close()
	src := NewManifest([]byte(fmt.Sprintf(bad, api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err == nil {
		t.Fatalf("configure: want validation error for stream commit_on_complete, got nil")
	} else if !strings.Contains(err.Error(), "commit_on_complete") {
		t.Fatalf("configure: err = %v, want a commit_on_complete error", err)
	}
}
