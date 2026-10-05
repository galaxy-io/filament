package httpapi

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/internal/export"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const exportTestManifest = `
version: 1
name: export_test
display_name: Export test
description: Async export runtime fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  api_key: {type: secret}
connection:
  base_url: BASE_URL
  auth: {bearer: config.api_key}
  headers: {X-Private-Header: secret-header}
resources:
  - name: items
    mode: export
    primary_key: [id]
    fields:
      id: string
      raw: {type: json, mode: remainder}
    export:
      start:
        method: POST
        path: /exports
        body: {encoding: json, template: {dataset: items}}
        capture: {id: $.id, download_url: $.url}
      wait:
        type: job
        request: {method: GET, path: '/exports/{{ job.id }}'}
        state: {path: $.status, pending: [queued], ready: [finished], failed: [failed]}
        capture: {download_url: $.url}
        interval_seconds: 1
        timeout_seconds: 30
      result:
        url: '{{ job.download_url }}'
        allowed_hosts: [127.0.0.1]
        compression: gzip
        format: json
`

func exportSource(t *testing.T, handler http.HandlerFunc, edit func(string) string) (*Source, *httptest.Server) {
	t.Helper()
	api := httptest.NewTLSServer(handler)
	t.Cleanup(api.Close)
	data := strings.ReplaceAll(exportTestManifest, "BASE_URL", api.URL)
	if edit != nil {
		data = edit(data)
	}
	src := NewManifest([]byte(data))
	if err := src.Configure(t.Context(), filament.NewConfig(map[string]any{"api_key": "secret-api-key"})); err != nil {
		t.Fatal(err)
	}
	src.connector.client = api.Client()
	src.connector.streamClient = api.Client()
	t.Cleanup(func() { src.Teardown(context.Background()) })
	return src, api
}

func gzipExport(t *testing.T, raw string) []byte {
	t.Helper()
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := io.WriteString(w, raw); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestExportLifecycleResumeAndCredentialIsolation(t *testing.T) {
	var starts, polls, downloads atomic.Int32
	var fail atomic.Bool
	fail.Store(true)
	var base string
	data := gzipExport(t, `[{"id":9007199254740993,"value":"ok"}]`)
	src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/exports":
			starts.Add(1)
			if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer secret-api-key" {
				t.Error("missing creation auth or wrong method")
			}
			w.WriteHeader(202)
			fmt.Fprintf(w, `{"id":"job-1","url":%q}`, base+"/download?signature=private")
		case "/exports/job-1":
			polls.Add(1)
			fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/download?signature=private")
		case "/download":
			downloads.Add(1)
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-Private-Header") != "" {
				t.Error("credentials leaked to download")
			}
			if fail.Load() {
				w.WriteHeader(404)
				return
			}
			w.Write(data)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}, nil)
	base = api.URL
	plan, err := src.PlanResume(t.Context(), []string{"items"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var first collectSink
	if err := src.Extract(t.Context(), &first, filament.ExtractOpts{}); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("expected sanitized download failure: %v", err)
	}
	token := first.checkpoints["items"]
	if len(token) != 1 {
		t.Fatal("job checkpoint not published")
	}
	var state export.JobCheckpoint
	if err := json.Unmarshal([]byte(token[0]), &state); err != nil || state.Phase != "waiting" {
		t.Fatalf("job state: %+v %v", state, err)
	}
	cp, _ := checkpoint.ParseKeyset(plan["items"])
	cp.Shards[0].Key = token
	plan["items"] = cp.ToCheckpoint("items")
	fail.Store(false)
	var resumed collectSink
	if err := src.ExtractFrom(t.Context(), &resumed, filament.ExtractOpts{}, plan); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 1 || polls.Load() != 2 || downloads.Load() != 2 || len(resumed.records) != 1 {
		t.Fatalf("starts=%d polls=%d downloads=%d records=%d", starts.Load(), polls.Load(), downloads.Load(), len(resumed.records))
	}
	if resumed.records[0].ID != "9007199254740993" {
		t.Fatalf("numeric ID lost precision: %s", resumed.records[0].ID)
	}
	cp.Shards[0].Key = resumed.checkpoints["items"]
	plan["items"] = cp.ToCheckpoint("items")
	var done collectSink
	if err := src.ExtractFrom(t.Context(), &done, filament.ExtractOpts{}, plan); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 1 || downloads.Load() != 2 {
		t.Fatal("completed export was replayed")
	}
	// A fresh run creates a fresh job rather than carrying an old export forever.
	var next collectSink
	if err := src.Extract(t.Context(), &next, filament.ExtractOpts{}); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 2 {
		t.Fatal("fresh read did not create a job")
	}
}

func TestExportCreationIsNotRetried(t *testing.T) {
	for _, code := range []int{429, 500, 307} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			var calls atomic.Int32
			src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.Header().Set("Location", "/again")
				w.WriteHeader(code)
			}, nil)
			var sink collectSink
			if err := src.Extract(t.Context(), &sink, filament.ExtractOpts{}); err == nil {
				t.Fatal("expected failure")
			}
			if calls.Load() != 1 {
				t.Fatalf("creation retried %d times", calls.Load())
			}
			plan, err := src.PlanResume(t.Context(), []string{"items"}, nil)
			if err != nil {
				t.Fatal(err)
			}
			cp, _ := checkpoint.ParseKeyset(plan["items"])
			cp.Shards[0].Key = sink.checkpoints["items"]
			plan["items"] = cp.ToCheckpoint("items")
			if err := src.ExtractFrom(t.Context(), &collectSink{}, filament.ExtractOpts{}, plan); err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
				t.Fatalf("ambiguous resume: %v", err)
			}
			if calls.Load() != 1 {
				t.Fatal("ambiguous creation was resubmitted")
			}
		})
	}
}

func TestExportPendingCancellationAndFailure(t *testing.T) {
	for _, status := range []string{"queued", "failed", "unknown", ""} {
		t.Run(status, func(t *testing.T) {
			var starts atomic.Int32
			src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "POST" {
					starts.Add(1)
					fmt.Fprint(w, `{"id":"x","url":"https://127.0.0.1/file"}`)
					return
				}
				fmt.Fprintf(w, `{"status":%q}`, status)
			}, nil)
			ctx, cancel := context.WithTimeout(t.Context(), 150*time.Millisecond)
			defer cancel()
			err := src.Extract(ctx, &collectSink{}, filament.ExtractOpts{})
			if err == nil {
				t.Fatal("expected failure")
			}
			if status == "queued" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("cancellation: %v", err)
			}
			if starts.Load() != 1 {
				t.Fatal("pending poll created another job")
			}
		})
	}
}

func TestExportDownloadReadinessAndEmptyCompletion(t *testing.T) {
	var base string
	var gets atomic.Int32
	src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			fmt.Fprintf(w, `{"id":"x","url":%q}`, base+"/file")
			return
		}
		if gets.Add(1) == 1 {
			w.WriteHeader(403)
			return
		}
		fmt.Fprint(w, "") // empty NDJSON is a completed, empty export
	}, func(s string) string {
		start := strings.Index(s, "      wait:")
		end := strings.Index(s, "      result:")
		s = s[:start] + "      wait:\n        type: download\n        pending_statuses: [403]\n        interval_seconds: 1\n        timeout_seconds: 30\n" + s[end:]
		return strings.Replace(s, "        compression: gzip\n        format: json", "        format: ndjson", 1)
	})
	base = api.URL
	var sink collectSink
	if err := src.Extract(t.Context(), &sink, filament.ExtractOpts{}); err != nil {
		t.Fatal(err)
	}
	if gets.Load() != 2 || len(sink.records) != 0 {
		t.Fatal("readiness or empty read failed")
	}
	var state export.JobCheckpoint
	if err := json.Unmarshal([]byte(sink.checkpoints["items"][0]), &state); err != nil || state.Phase != "done" {
		t.Fatal("empty export wasn't checkpointed")
	}
}

func TestExportConnectionProbeDoesNotCreateJob(t *testing.T) {
	var calls atomic.Int32
	src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }, nil)
	if err := src.connector.TestConnection(t.Context()); err == nil {
		t.Fatal("export-only connector needs an ordinary connection probe")
	}
	if calls.Load() != 0 {
		t.Fatal("connection test created an export")
	}
}

func TestExportRejectsDownloadRedirect(t *testing.T) {
	var base string
	src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			fmt.Fprintf(w, `{"id":"x","url":%q}`, base+"/file")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/exports/") {
			fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/file")
			return
		}
		w.Header().Set("Location", "https://untrusted.example/?signature=secret")
		w.WriteHeader(302)
	}, nil)
	base = api.URL
	err := src.Extract(t.Context(), &collectSink{}, filament.ExtractOpts{})
	if err == nil || strings.Contains(err.Error(), "signature") {
		t.Fatalf("redirect error: %v", err)
	}
}

func TestExportManifestValidation(t *testing.T) {
	for _, tc := range []struct{ name, old, new string }{
		{"unknown-field", "format: json", "format: json\n        typo: true"},
		{"no-export-mode", "mode: export", "mode: paginated"},
		{"no-start", "start:\n        method: POST", "start:\n        method: DELETE"},
		{"missing-path", "        path: /exports\n", ""},
		{"start-job-scope", "path: /exports\n", "path: '/exports/{{ job.id }}'\n"},
		{"unknown-scope", "job.id", "other.id"},
		{"zero-interval", "interval_seconds: 1", "interval_seconds: 0"},
		{"unbounded-timeout", "timeout_seconds: 30", "timeout_seconds: 999999999"},
		{"overlapping-states", "ready: [finished]", "ready: [queued]"},
		{"empty-ready", "ready: [finished]", "ready: []"},
		{"wildcard-host", "allowed_hosts: [127.0.0.1]", "allowed_hosts: ['*.example.com']"},
		{"unsupported-auth", "format: json", "format: json\n        auth: inherited"},
		{"double-compression", "format: json", "format: json\n        archive: zip\n        files: '*.json'"},
		{"missing-archive-pattern", "format: json", "format: json\n        archive: tar"},
		{"bad-archive-pattern", "format: json", "format: json\n        archive: tar\n        files: '['"},
		{"parent", "primary_key: [id]", "for_each: parent\n    primary_key: [id]"},
		{"response cursor", "primary_key: [id]", "incremental: {response_cursor: $.cursor, start_param: since, inject_into: query}\n    primary_key: [id]"},
		{"resource-path", "primary_key: [id]", "path: /ignored\n    primary_key: [id]"},
		{"pagination", "primary_key: [id]", "pagination: {page: {page: query.page, page_size: 10}}\n    primary_key: [id]"},
		{"capture", "primary_key: [id]", "capture: {id: id}\n    primary_key: [id]"},
		{"job-pending-status", "interval_seconds: 1", "pending_statuses: [403]\n        interval_seconds: 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := manifest.Parse([]byte(strings.Replace(exportTestManifest, tc.old, tc.new, 1))); err == nil {
				t.Fatal("accepted invalid export declaration")
			}
		})
	}
	// Ordinary resource defaults cannot change the meaning of an export request.
	data := strings.Replace(exportTestManifest, "resources:", "defaults:\n  method: GET\n  pagination: {offset: {offset: query.offset, limit: query.limit, page_size: 10}}\n  headers: {Ignored: ordinary}\nresources:", 1)
	m, err := manifest.Parse([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if m.Resources[0].Method != "" || m.Resources[0].Pagination.Type != "" || len(m.Resources[0].Headers) != 0 {
		t.Fatal("ordinary defaults leaked into export")
	}
}

func TestExportRejectsMalformedOrChangedCheckpoint(t *testing.T) {
	var calls atomic.Int32
	src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"id":"job","url":"https://127.0.0.1/file"}`)
	}, nil)
	for _, token := range []string{`{"phase":"done"}`, `{"version":999}`, `not-json`} {
		cp := checkpoint.KeysetCheckpoint{Cols: []string{"export_state"}, Types: []string{"string"}, Shards: []checkpoint.KeysetShard{{Key: []string{token}}}}
		if err := src.ExtractFrom(t.Context(), &collectSink{}, filament.ExtractOpts{}, map[string]filament.Checkpoint{"items": cp.ToCheckpoint("items")}); err == nil {
			t.Fatal("accepted malformed checkpoint")
		}
	}
	if calls.Load() != 0 {
		t.Fatal("malformed checkpoint created a job")
	}
}

func TestExportReplaysPartialArtifactWithoutRecreatingJob(t *testing.T) {
	var base string
	var starts, requests atomic.Int32
	var complete atomic.Bool
	src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method == "POST" {
			starts.Add(1)
			fmt.Fprintf(w, `{"id":"job","url":%q}`, base+"/file")
			return
		}
		if r.URL.Path != "/file" {
			fmt.Fprintf(w, `{"status":"finished","url":%q}`, base+"/file")
			return
		}
		if !complete.Load() {
			fmt.Fprint(w, `[{"id":"one"},`)
			return
		}
		fmt.Fprint(w, `[{"id":"one"},{"id":"two"}]`)
	}, func(s string) string { return strings.Replace(s, "compression: gzip", "compression: none", 1) })
	base = api.URL
	var first collectSink
	if err := src.Extract(t.Context(), &first, filament.ExtractOpts{}); err == nil {
		t.Fatal("truncated export succeeded")
	}
	if len(first.records) != 1 {
		t.Fatalf("expected one delivered row: %d", len(first.records))
	}
	plan, err := src.PlanResume(t.Context(), []string{"items"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := checkpoint.ParseKeyset(plan["items"])
	cp.Shards[0].Key = first.records[0].Key
	// Even record checkpoints retain the existing job, not a row watermark.
	var state export.JobCheckpoint
	if err := json.Unmarshal([]byte(cp.Shards[0].Key[0]), &state); err != nil {
		t.Fatal(err)
	}
	saved := state
	before := requests.Load()
	state.Deadline = time.Now().Add(-time.Hour)
	cp.Shards[0].Key = []string{state.Token()}
	plan["items"] = cp.ToCheckpoint("items")
	if err := src.ExtractFrom(t.Context(), &collectSink{}, filament.ExtractOpts{}, plan); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expired job: %v", err)
	}
	state = saved
	state.Identity = "different-request"
	cp.Shards[0].Key = []string{state.Token()}
	plan["items"] = cp.ToCheckpoint("items")
	if err := src.ExtractFrom(t.Context(), &collectSink{}, filament.ExtractOpts{}, plan); err == nil {
		t.Fatal("accepted different job identity")
	}
	if requests.Load() != before {
		t.Fatal("invalid or expired checkpoint issued requests")
	}
	cp.Shards[0].Key = []string{saved.Token()}
	plan["items"] = cp.ToCheckpoint("items")
	complete.Store(true)
	var resumed collectSink
	if err := src.ExtractFrom(t.Context(), &resumed, filament.ExtractOpts{}, plan); err != nil {
		t.Fatal(err)
	}
	if starts.Load() != 1 || len(resumed.records) != 2 || resumed.records[0].ID != "one" {
		t.Fatal("did not replay the original artifact")
	}
}

func TestExportRefreshesExpiredURLOnce(t *testing.T) {
	for _, success := range []bool{true, false} {
		t.Run(fmt.Sprint(success), func(t *testing.T) {
			var base string
			var starts, polls atomic.Int32
			data := gzipExport(t, `[]`)
			src, api := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == "POST":
					starts.Add(1)
					fmt.Fprintf(w, `{"id":"job","url":%q}`, base+"/expired")
				case strings.HasPrefix(r.URL.Path, "/exports/"):
					location := base + "/expired"
					if polls.Add(1) > 1 {
						location = base + "/fresh"
					}
					fmt.Fprintf(w, `{"status":"finished","url":%q}`, location)
				case r.URL.Path == "/fresh" && success:
					w.Write(data)
				default:
					w.WriteHeader(403)
				}
			}, nil)
			base = api.URL
			err := src.Extract(t.Context(), &collectSink{}, filament.ExtractOpts{})
			if (err == nil) != success || starts.Load() != 1 || polls.Load() != 2 {
				t.Fatalf("starts=%d polls=%d err=%v", starts.Load(), polls.Load(), err)
			}
		})
	}
}
