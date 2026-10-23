package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const statementManifest = `
version: 1
name: statements
display_name: Statements
description: Statement pagination fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  page_size: {type: int, default: 2, minimum: 1, maximum: 1000}
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /query
    restart_on_resume: true
    primary_key: [id]
    fields:
      id: {path: Id, type: string}
      updated: {path: Meta.updated, type: timestamptz}
    query:
      query: "SELECT * FROM Item{{ state.since | default \"\" }} START {{ state.offset }} SIZE {{ state.size }}"
    response:
      records: $.Result.Item
      required_objects: [Result]
      strict_records: true
      empty_object: Result
      pagination:
        offset: {offset: state.offset, limit: state.size, page_size: 2, page_size_from: page_size, start: 1}
    incremental:
      cursor_field: updated
      start_param: since
      inject_into: state
      comparator: time
      initial: '2026-01-01T00:00:00Z'
      overlap_seconds: 300
      value_prefix: " WHERE updated >= '"
      value_suffix: "'"
      commit_on_complete: true
`

type scanSink struct {
	records       []record
	mark          string
	failAfter     int
	checkpointErr error
}

func (s *scanSink) Push(r record) error {
	if s.failAfter > 0 && len(s.records) >= s.failAfter {
		return errors.New("sink write failed")
	}
	s.records = append(s.records, r)
	return nil
}

func (s *scanSink) PushBatch(rs []record) error {
	for _, r := range rs {
		if err := s.Push(r); err != nil {
			return err
		}
	}
	return nil
}

func (s *scanSink) Checkpoint(_, mark string) error {
	if s.checkpointErr != nil {
		return s.checkpointErr
	}
	s.mark = mark
	return nil
}

func TestStatementPaginationCommitsOnlyCompleteScan(t *testing.T) {
	for _, mode := range []string{"complete", "limit", "failure", "duplicate", "empty", "full", "sink_failure", "checkpoint_failure", "before_bound"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				want := fmt.Sprintf("SELECT * FROM Item WHERE updated >= '2026-01-01T23:55:00Z' START %d SIZE 2", 1+(calls-1)*2)
				if mode == "full" {
					want = fmt.Sprintf("SELECT * FROM Item START %d SIZE 2", 1+(calls-1)*2)
				}
				if r.URL.Query().Get("query") != want || len(r.URL.Query()) != 1 {
					t.Errorf("query=%v want=%s", r.URL.Query(), want)
				}
				if mode == "before_bound" {
					fmt.Fprint(w, `{"Result":{"Item":[{"Id":"old","Meta":{"updated":"2025-01-01T00:00:00Z"}}]}}`)
					return
				}
				if mode == "empty" {
					fmt.Fprint(w, `{"Result":{}}`)
					return
				}
				if calls == 1 {
					fmt.Fprint(w, `{"Result":{"Item":[{"Id":"a","Meta":{"updated":"2026-01-05T00:00:00Z"}},{"Id":"b","Meta":{"updated":"2026-01-03T00:00:00Z"}}]}}`)
					return
				}
				if mode == "failure" {
					w.WriteHeader(400)
					fmt.Fprint(w, `{}`)
					return
				}
				if mode == "duplicate" {
					fmt.Fprint(w, `{"Result":{"Item":[{"Id":"a","Meta":{"updated":"2026-01-05T00:00:00Z"}}]}}`)
					return
				}
				fmt.Fprint(w, `{"Result":{"Item":[{"Id":"c","Meta":{"updated":"2026-01-04T00:00:00Z"}}]}}`)
			}))
			defer api.Close()
			source := NewManifest([]byte(strings.Replace(statementManifest, "base_url: https://example.com", "base_url: "+api.URL, 1)))
			if err := source.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
				t.Fatal(err)
			}
			defer source.Teardown(t.Context())
			opts := extractOptions{Resources: []string{"items"}, ResumeWatermarks: map[string]map[string]string{"items": {"updated": "2026-01-02T00:00:00Z"}}, IncrementalResources: map[string]bool{"items": mode != "full"}}
			if mode == "limit" {
				opts.Limit = 2
			}
			sink := &scanSink{}
			if mode == "sink_failure" {
				sink.failAfter = 1
			}
			if mode == "checkpoint_failure" {
				sink.checkpointErr = errors.New("checkpoint drain failed")
			}
			err := source.connector.Extract(t.Context(), sink, opts)
			if (err != nil) != (mode == "failure" || mode == "duplicate" || mode == "sink_failure" || mode == "checkpoint_failure" || mode == "before_bound") {
				t.Fatalf("err=%v", err)
			}
			wantMark := ""
			if mode == "complete" {
				wantMark = "2026-01-05T00:00:00Z"
			}
			if sink.mark != wantMark {
				t.Fatalf("checkpoint=%s want=%s", sink.mark, wantMark)
			}
			for _, r := range sink.records {
				if len(r.Key) > 0 {
					t.Fatalf("intermediate checkpoint=%v", r.Key)
				}
			}
			if mode == "limit" && calls != 1 {
				t.Fatalf("limited scan made %d calls", calls)
			}
			prev := map[string]filament.Checkpoint{"items": checkpoint.KeysetCheckpoint{Cols: []string{"pagination_state"}, Shards: []checkpoint.KeysetShard{{Key: []string{"old"}}}}.ToCheckpoint("items")}
			plan, err := source.PlanResume(t.Context(), []string{"items"}, prev)
			if err != nil || len(plan) != 0 {
				t.Fatalf("resume plan=%v err=%v", plan, err)
			}
		})
	}
}

func TestStatementManifestInvalidDeclarations(t *testing.T) {
	for _, pair := range [][2]string{
		{"start: 1", "start: -1"},
		{"page_size_from: page_size", "page_size_from: missing"},
		{"commit_on_complete: true", "commit_on_complete: true\n      response_cursor: next"},
		{"{path: Id, type: string}", "{path: Id, type: decimal, precision: 2, scale: 3}"},
	} {
		if _, err := manifest.Parse([]byte(strings.Replace(statementManifest, pair[0], pair[1], 1))); err == nil {
			t.Fatalf("accepted %s", pair[1])
		}
	}
}
