package httpapi

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
)

const pageSessionManifest = `version: 1
name: engage
display_name: Engage
description: Zero-based session paging fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection: {base_url: BASE}
resources:
  - name: profiles
    path: /engage
    method: POST
    body: {encoding: form, template: {}}
    primary_key: [distinct_id]
    fields: {distinct_id: {path: $distinct_id, type: string}}
    response:
      records: $.results
      pagination:
        page:
          number: body.page
          size: body.page_size
          page_size: 2
          start: 0
          session: {response: session_id, request: body.session_id}
`

// The walk mirrors Mixpanel Engage. Page 0 carries no session, the response
// hands one back, and every later page must send it with the next number.
func TestPageSessionWalk(t *testing.T) {
	var requests []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		page, session := r.PostForm.Get("page"), r.PostForm.Get("session_id")
		requests = append(requests, "page="+page+" session="+session+" size="+r.PostForm.Get("page_size"))
		if page != "0" && session != "abc" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"must have session_id when requesting page > 0"}`)
			return
		}
		rows := map[string]string{"0": `{"$distinct_id":"a"},{"$distinct_id":"b"}`, "1": `{"$distinct_id":"c"},{"$distinct_id":"d"}`, "2": `{"$distinct_id":"e"}`}[page]
		fmt.Fprintf(w, `{"results":[%s],"page":%s,"session_id":"abc","status":"ok"}`, rows, page)
	}))
	defer api.Close()

	src := NewManifest([]byte(strings.ReplaceAll(pageSessionManifest, "BASE", api.URL)))
	if err := src.Configure(t.Context(), filament.NewConfig(nil)); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(t.Context())
	sink := &collectSink{}
	if err := src.Extract(t.Context(), sink, filament.ExtractOpts{Resources: []string{"profiles"}}); err != nil {
		t.Fatal(err)
	}
	want := "page=0 session= size=2,page=1 session=abc size=2,page=2 session=abc size=2"
	if got := strings.Join(requests, ","); got != want {
		t.Fatalf("requests = %s\nwant       %s", got, want)
	}
	if len(sink.records) != 5 {
		t.Fatalf("records = %d, want 5", len(sink.records))
	}
}

func TestPageSessionValidation(t *testing.T) {
	valid := strings.ReplaceAll(pageSessionManifest, "BASE", "https://example.com")
	if err := NewManifest([]byte(valid)).manifestErr; err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for _, tc := range []struct{ name, old, new, want string }{
		{"negative start", "start: 0", "start: -1", "pagination"},
		{"bad target", "request: body.session_id", "request: state.session_id", "session.request"},
		{"overwrites number", "request: body.session_id", "request: body.page", "conflicts with the page number"},
		{"missing response", "session: {response: session_id, request: body.session_id}", "session: {request: body.session_id}", "response"},
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

// An incremental bound sent under the session parameter's name would replace
// the token on every continuation page, so the manifest is rejected.
func TestPageSessionRejectsIncrementalBoundOnTheSameParam(t *testing.T) {
	base := strings.NewReplacer(
		"BASE", "https://example.com",
		"    fields: {distinct_id: {path: $distinct_id, type: string}}\n",
		"    fields:\n      distinct_id: {path: $distinct_id, type: string}\n      last_seen: {path: $last_seen, type: timestamptz}\n"+
			"    incremental: {cursor_field: last_seen, start_param: since, end_param: until, inject_into: body, comparator: time}\n",
	).Replace(pageSessionManifest)
	if err := NewManifest([]byte(base)).manifestErr; err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for _, tc := range []struct{ name, old, want string }{
		{"start_param", "start_param: since", "incremental.start_param"},
		{"end_param", "end_param: until", "incremental.end_param"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			param := tc.old[:strings.Index(tc.old, ":")]
			invalid := strings.Replace(base, tc.old, param+": session_id", 1)
			err := NewManifest([]byte(invalid)).manifestErr
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "session token") {
				t.Fatalf("error = %v, want a session collision on %s", err, tc.want)
			}
		})
	}
	queryTarget := strings.Replace(strings.Replace(base, "start_param: since", "start_param: session_id", 1), "inject_into: body", "inject_into: query", 1)
	if err := NewManifest([]byte(queryTarget)).manifestErr; err != nil && strings.Contains(err.Error(), "session token") {
		t.Fatalf("different targets reported as a collision: %v", err)
	}
}
