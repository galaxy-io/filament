package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/pagination"
)

func TestUnselectedExportPreservesParentResume(t *testing.T) {
	var cursor string
	src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/segments" {
			t.Errorf("unexpected request: %s", r.URL.Path)
		}
		cursor = r.URL.Query().Get("after")
		fmt.Fprint(w, `{"items":[{"id":"b"}]}`)
	}, func(s string) string {
		s = scopedExportFixture(s)
		return strings.Replace(s, "    capture_only: true", "    response: {records: $.items, pagination: {cursor: {response: next, request: query.after, null_terminates: true}}}", 1)
	})
	previous := map[string]filament.Checkpoint{"segments": checkpoint.KeysetCheckpoint{
		Mode: checkpoint.ModeKeyset, Cols: []string{"pagination_state"}, Types: []string{"string"},
		Shards: []checkpoint.KeysetShard{{Key: []string{pagination.State{Cursor: "page-two"}.Checkpoint()}}},
	}.ToCheckpoint("segments")}
	var sink collectSink
	if err := src.ExtractFrom(t.Context(), &sink, filament.ExtractOpts{Resources: []string{"segments"}}, previous); err != nil {
		t.Fatal(err)
	}
	if cursor != "page-two" {
		t.Fatalf("parent resume cursor = %q, want page-two", cursor)
	}
}

func scopedExportFixture(s string) string {
	s = strings.Replace(s, "primary_key: [id]", "for_each: segments\n    parent: {concurrency: 1}\n    incremental: {cursor_field: updated_at, start_param: since, inject_into: body, initial: '2026-01-01T00:00:00Z', comparator: time, overlap_seconds: 60}\n    primary_key: [id]", 1)
	s = strings.Replace(s, "id: string", "id: string\n      updated_at: timestamptz\n      segment: {type: string, path: parent.segment_id}", 1)
	s = strings.Replace(s, "export:\n", "export:\n      parent_key: [segment_id]\n", 1)
	s = strings.Replace(s, "dataset: items", "segment: '{{ parent.segment_id }}'", 1)
	s = strings.Replace(s, "compression: gzip", "compression: none", 1)
	return s + `
  - name: segments
    path: /segments
    capture_only: true
    capture: {segment_id: id}
    records: $.items
`
}

func exportAttempt(t *testing.T, plan map[string]filament.Checkpoint, sink *collectSink) map[string]filament.Checkpoint {
	t.Helper()
	cp, ok := checkpoint.ParseKeyset(plan["items"])
	if !ok || len(sink.checkpoints["items"]) != 1 {
		t.Fatal("missing export snapshot")
	}
	cp.Shards[0].Key = sink.checkpoints["items"]
	return map[string]filament.Checkpoint{"items": cp.ToCheckpoint("items")}
}

func TestScopedExportFrozenParentsAndCommittedWatermarks(t *testing.T) {
	var mu sync.Mutex
	var base string
	parents := `{"items":[{"id":"a"},{"id":"b"}]}`
	broken := true
	starts := map[string][]string{}
	src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.URL.Path == "/segments":
			fmt.Fprint(w, parents)
		case r.Method == "POST":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			id := body["segment"]
			starts[id] = append(starts[id], body["since"])
			fmt.Fprintf(w, `{"id":%q,"url":%q}`, id, base+"/file/"+id)
		case strings.HasPrefix(r.URL.Path, "/exports/"):
			fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/file/"+strings.TrimPrefix(r.URL.Path, "/exports/"))
		case r.URL.Path == "/file/a":
			// Deliberately unordered: the first record is the maximum.
			fmt.Fprint(w, `[{"id":"a1","updated_at":"2026-01-10T00:00:00Z"},{"id":"a2","updated_at":"2026-01-02T00:00:00Z"}]`)
		case r.URL.Path == "/file/b":
			if broken {
				fmt.Fprint(w, `[{"id":"b1","updated_at":"2026-01-05T00:00:00Z"},`)
				return
			}
			fmt.Fprint(w, `[{"id":"b1","updated_at":"2026-01-05T00:00:00Z"}]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}, scopedExportFixture)
	base = api.URL
	plan, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var failed collectSink
	if err := src.ExtractFrom(t.Context(), &failed, filament.ExtractOpts{Resources: []string{"items"}}, plan); err == nil {
		t.Fatal("truncated parent succeeded")
	}
	attempt := exportAttempt(t, plan, &failed)
	res, _ := src.manifestResource("items")
	state, err := src.exportPlanState(res, attempt["items"], true)
	if err != nil {
		t.Fatal(err)
	}
	a, b := state.Parents[`["a"]`], state.Parents[`["b"]`]
	if a.Committed != "" || a.Job.Candidate != "2026-01-10T00:00:00Z" || b.Job.Candidate != "" || b.Job.Phase != "waiting" {
		t.Fatalf("partial state: %#v %#v", a, b)
	}
	for _, row := range failed.records {
		if len(row.Key) != 0 {
			t.Fatal("row advanced export checkpoint")
		}
	}
	mu.Lock()
	broken = false
	parents = `{"items":[]}`
	mu.Unlock()
	planned, err := src.PlanIncremental(t.Context(), []string{"items"}, plan, nil)
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := src.PlanIncrementalResume(t.Context(), planned, attempt)
	if err != nil {
		t.Fatal(err)
	}
	var success collectSink
	if err := src.ExtractFrom(t.Context(), &success, filament.ExtractOpts{Resources: []string{"items"}}, resumed); err != nil {
		t.Fatal(err)
	}
	if len(success.records) != 3 {
		t.Fatalf("replayed rows = %d", len(success.records))
	}
	mu.Lock()
	if len(starts["a"]) != 1 || len(starts["b"]) != 1 || starts["a"][0] != "2025-12-31T23:59:00Z" {
		t.Errorf("start requests = %#v", starts)
	}
	parents = `{"items":[{"id":"a"},{"id":"c"}]}`
	mu.Unlock()
	committed := exportAttempt(t, resumed, &success)
	next, err := src.PlanIncremental(t.Context(), []string{"items"}, committed, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The previous attempt is stale once a committed generation advances.
	if _, err := src.PlanIncrementalResume(t.Context(), next, attempt); err == nil {
		t.Fatal("accepted stale attempt")
	}
	var fresh collectSink
	if err := src.ExtractFrom(t.Context(), &fresh, filament.ExtractOpts{Resources: []string{"items"}}, next); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if len(starts["b"]) != 1 || len(starts["a"]) != 2 || starts["a"][1] != "2026-01-09T23:59:00Z" || starts["c"][0] != "2025-12-31T23:59:00Z" {
		t.Errorf("next requests = %#v", starts)
	}
	mu.Unlock()
	final := exportAttempt(t, next, &fresh)
	next, err = src.PlanIncremental(t.Context(), []string{"items"}, final, nil)
	if err != nil {
		t.Fatal(err)
	}
	state, err = src.exportPlanState(res, next["items"], true)
	if err != nil {
		t.Fatal(err)
	}
	if state.Parents[`["b"]`].Committed != "2026-01-05T00:00:00Z" || state.Parents[`["c"]`].Committed != "2026-01-01T00:00:00Z" {
		t.Fatalf("historical/empty watermark lost: %#v", state.Parents)
	}
}

func TestScopedExportRejectsAmbiguousParentsBeforeCreatingJobs(t *testing.T) {
	for _, parents := range []string{`[{"id":"a"},{"id":"a"}]`, `[{"id":""}]`} {
		t.Run(parents, func(t *testing.T) {
			src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					t.Error("created job before validating parents")
				}
				fmt.Fprintf(w, `{"items":%s}`, parents)
			}, scopedExportFixture)
			plan, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := src.ExtractFrom(t.Context(), &collectSink{}, filament.ExtractOpts{Resources: []string{"items"}}, plan); err == nil {
				t.Fatal("accepted ambiguous parent set")
			}
		})
	}
}

func TestScopedExportConcurrentCompletionPreservesEveryParent(t *testing.T) {
	var base string
	bDelivered := make(chan struct{})
	src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/segments":
			fmt.Fprint(w, `{"items":[{"id":"a"},{"id":"b"}]}`)
		case r.Method == "POST":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				return
			}
			fmt.Fprintf(w, `{"id":%q,"url":%q}`, body["segment"], base+"/file/"+body["segment"])
		case strings.HasPrefix(r.URL.Path, "/exports/"):
			fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/file/"+strings.TrimPrefix(r.URL.Path, "/exports/"))
		case r.URL.Path == "/file/a":
			select {
			case <-bDelivered:
			case <-r.Context().Done():
				return
			}
			fmt.Fprint(w, `[{"id":"a","updated_at":"2026-01-10T00:00:00Z"}]`)
		case r.URL.Path == "/file/b":
			fmt.Fprint(w, `[{"id":"b","updated_at":"2026-01-05T00:00:00Z"}]`)
			close(bDelivered)
		}
	}, func(s string) string {
		return strings.Replace(scopedExportFixture(s), "concurrency: 1", "concurrency: 2", 1)
	})
	base = api.URL
	plan, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	var sink collectSink
	if err := src.ExtractFrom(t.Context(), &sink, filament.ExtractOpts{Resources: []string{"items"}}, plan); err != nil {
		t.Fatal(err)
	}
	res, _ := src.manifestResource("items")
	state, err := src.exportPlanState(res, exportAttempt(t, plan, &sink)["items"], true)
	if err != nil {
		t.Fatal(err)
	}
	for _, parent := range state.Parents {
		if parent.Job.Phase != "done" || parent.Job.Candidate == "" {
			t.Fatalf("lost completed parent: %#v", parent)
		}
	}
	if len(sink.records) != 2 {
		t.Fatalf("records: %d", len(sink.records))
	}
}

func TestExportTopLevelIncrementalInjection(t *testing.T) {
	for _, injection := range []string{"query", "header", "body"} {
		t.Run(injection, func(t *testing.T) {
			var base string
			src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					got := ""
					switch injection {
					case "query":
						got = r.URL.Query().Get("since")
					case "header":
						got = r.Header.Get("since")
					case "body":
						var body map[string]any
						if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
							t.Error(err)
						}
						got, _ = body["since"].(string)
					}
					if got != "10" {
						t.Errorf("injected lower bound = %q", got)
					}
					fmt.Fprintf(w, `{"id":"job","url":%q}`, base+"/file")
				} else if r.URL.Path == "/file" {
					fmt.Fprint(w, `[{"id":"1","sequence":30},{"id":"2","sequence":20}]`)
				} else {
					fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/file")
				}
			}, func(s string) string {
				s = strings.Replace(s, "primary_key: [id]", "incremental: {cursor_field: sequence, start_param: since, inject_into: "+injection+", initial: '10', comparator: numeric}\n    primary_key: [id]", 1)
				s = strings.Replace(s, "id: string", "id: string\n      sequence: int64", 1)
				return strings.Replace(s, "compression: gzip", "compression: none", 1)
			})
			base = api.URL
			plan, err := src.PlanIncremental(t.Context(), []string{"items"}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			var sink collectSink
			if err := src.ExtractFrom(t.Context(), &sink, filament.ExtractOpts{}, plan); err != nil {
				t.Fatal(err)
			}
			res, _ := src.manifestResource("items")
			next, err := src.PlanIncremental(t.Context(), []string{"items"}, exportAttempt(t, plan, &sink), nil)
			if err != nil {
				t.Fatal(err)
			}
			state, err := src.exportPlanState(res, next["items"], true)
			if err != nil {
				t.Fatal(err)
			}
			if state.Parents["$"].Committed != "30" {
				t.Fatalf("watermark = %q", state.Parents["$"].Committed)
			}
		})
	}
}

func TestScopedExportFullReadAndEmptyParents(t *testing.T) {
	for _, parents := range []string{`[]`, `[{"id":"a"}]`} {
		t.Run(parents, func(t *testing.T) {
			var base string
			src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.URL.Path == "/segments":
					fmt.Fprintf(w, `{"items":%s}`, parents)
				case r.Method == "POST":
					fmt.Fprintf(w, `{"id":"job","url":%q}`, base+"/file")
				case r.URL.Path == "/file":
					fmt.Fprint(w, `[{"id":"one","updated_at":"2026-01-03T00:00:00Z"}]`)
				default:
					fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/file")
				}
			}, scopedExportFixture)
			base = api.URL
			plan, err := src.PlanResume(t.Context(), []string{"items"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var sink collectSink
			if err := src.ExtractFrom(t.Context(), &sink, filament.ExtractOpts{Resources: []string{"items"}}, plan); err != nil {
				t.Fatal(err)
			}
			res, _ := src.manifestResource("items")
			state, err := src.exportPlanState(res, exportAttempt(t, plan, &sink)["items"], false)
			if err != nil {
				t.Fatal(err)
			}
			if !state.Initialized || state.Incremental {
				t.Fatalf("full state: %#v", state)
			}
			if parents == `[]` && len(state.Selected) != 0 {
				t.Fatal("empty index created jobs")
			}
		})
	}
}

func TestScopedExportManifestValidation(t *testing.T) {
	fixture := scopedExportFixture(exportTestManifest)
	for _, tc := range []struct{ name, old, replacement string }{
		{"missing-key", "      parent_key: [segment_id]\n", ""},
		{"unknown-key", "parent_key: [segment_id]", "parent_key: [unknown]"},
		{"duplicate-key", "parent_key: [segment_id]", "parent_key: [segment_id, segment_id]"},
		{"parent-gating", "parent: {concurrency: 1}", "parent: {concurrency: 1, since: segment_id}"},
		{"incremental-parent", "capture_only: true", "incremental: {cursor_field: id, start_param: since, inject_into: query}\n    fields: {id: string}\n    primary_key: [id]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := manifest.Parse([]byte(strings.Replace(fixture, tc.old, tc.replacement, 1))); err == nil {
				t.Fatal("accepted unsafe export parent declaration")
			}
		})
	}
	// Check the entire ancestry, not just the direct parent.
	nested := strings.Replace(fixture, "    capture_only: true", "    for_each: root\n    capture_only: true", 1) + `
  - name: root
    path: /root
    fields: {id: string}
    primary_key: [id]
    capture: {id: id}
    incremental: {cursor_field: id, start_param: since, inject_into: query}
`
	if _, err := manifest.Parse([]byte(nested)); err == nil || !strings.Contains(err.Error(), "export ancestors") {
		t.Fatalf("incremental ancestor validation: %v", err)
	}
}
