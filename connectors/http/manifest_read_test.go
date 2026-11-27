package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestNullableCursorSurvivesFullReadButFailsIncremental(t *testing.T) {
	const nullableCursorManifest = `
version: 1
name: nullable-cursor
display_name: Nullable Cursor
description: Nullable cursor fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://example.com
resources:
  - name: items
    path: /items
    primary_key: [id]
    fields:
      id: string
      updated: timestamptz?
    response:
      records: $.items
    incremental:
      cursor_field: updated
      start_param: since
      inject_into: query
      comparator: time
      initial: '2026-01-01T00:00:00Z'
      overlap_seconds: 86400
`
	for _, mode := range []string{"full", "incremental"} {
		t.Run(mode, func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"items":[{"id":"a","updated":"2026-01-05T00:00:00Z"},{"id":"b","updated":null}]}`)
			}))
			defer api.Close()
			source := NewManifest([]byte(strings.Replace(nullableCursorManifest, "base_url: https://example.com", "base_url: "+api.URL, 1)))
			if err := source.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
				t.Fatal(err)
			}
			defer source.Teardown(t.Context())
			opts := extractOptions{Resources: []string{"items"}, IncrementalResources: map[string]bool{"items": mode == "incremental"}}
			sink := &scanSink{}
			err := source.connector.Extract(t.Context(), sink, opts)
			if mode == "full" {
				if err != nil {
					t.Fatalf("extract: %v", err)
				}
				if len(sink.records) != 2 {
					t.Fatalf("records = %d, want 2", len(sink.records))
				}
				var row map[string]any
				if err := json.Unmarshal(sink.records[1].Data, &row); err != nil {
					t.Fatalf("unmarshal record: %v", err)
				}
				v, ok := row["updated"]
				if !ok {
					t.Fatalf("null cursor row is missing the updated key")
				}
				if v != nil {
					t.Fatalf("null cursor row updated = %v, want nil", v)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "cursor field") {
				t.Fatalf("err = %v, want a cursor field error", err)
			}
		})
	}
}

// A scheduled Twilio message has a null sender and a null date_sent. The
// messages manifest declares from as nullable, so a full read must accept
// that row and keep going instead of stopping the resource read.
func TestTwilioScheduledMessageNullSenderSurvivesFullRead(t *testing.T) {
	data, err := os.ReadFile("manifests/twilio/2010-04-01/manifest.yaml")
	if err != nil {
		t.Fatalf("read Twilio manifest: %v", err)
	}
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"messages":[
			{"sid":"SMscheduled","account_sid":"ACtest","api_version":"2010-04-01","date_created":"Fri, 18 Oct 2019 17:00:00 +0000","date_sent":null,"date_updated":"Fri, 18 Oct 2019 17:00:00 +0000","status":"scheduled","direction":"outbound-api","from":null,"to":"+15551234567","uri":"/2010-04-01/Accounts/ACtest/Messages/SMscheduled.json","subresource_uris":{}},
			{"sid":"SMsent","account_sid":"ACtest","api_version":"2010-04-01","date_created":"Fri, 18 Oct 2019 17:00:00 +0000","date_sent":"Fri, 18 Oct 2019 17:02:00 +0000","date_updated":"Fri, 18 Oct 2019 17:02:00 +0000","status":"delivered","direction":"outbound-api","from":"+15559876543","to":"+15551234567","uri":"/2010-04-01/Accounts/ACtest/Messages/SMsent.json","subresource_uris":{}}
		]}`)
	}))
	defer api.Close()
	manifestText := strings.Replace(string(data), "base_url: https://api.twilio.com", "base_url: "+api.URL, 1)
	source := NewManifest([]byte(manifestText))
	if err := source.Configure(t.Context(), filament.NewConfig(map[string]any{"account_sid": "ACtest", "auth_token": "test-token"})); err != nil {
		t.Fatal(err)
	}
	defer source.Teardown(t.Context())
	sink := &scanSink{}
	opts := extractOptions{Resources: []string{"messages"}, IncrementalResources: map[string]bool{"messages": false}}
	if err := source.connector.Extract(t.Context(), sink, opts); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if len(sink.records) != 2 {
		t.Fatalf("records = %d, want 2", len(sink.records))
	}
	rows := map[string]map[string]any{}
	for _, rec := range sink.records {
		var row map[string]any
		if err := json.Unmarshal(rec.Data, &row); err != nil {
			t.Fatalf("unmarshal record: %v", err)
		}
		rows[row["sid"].(string)] = row
	}
	scheduled, ok := rows["SMscheduled"]
	if !ok {
		t.Fatal("scheduled row missing")
	}
	if v, ok := scheduled["from"]; !ok || v != nil {
		t.Fatalf("scheduled from = %v, want nil", v)
	}
	if v, ok := scheduled["date_sent"]; !ok || v != nil {
		t.Fatalf("scheduled date_sent = %v, want nil", v)
	}
	if got := rows["SMsent"]["from"]; got != "+15559876543" {
		t.Fatalf("sent from = %v, want +15559876543", got)
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
