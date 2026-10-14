package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/internal/export"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

func directExportFixture(s string) string {
	s = s[:strings.Index(s, "    export:")] + `    export:
      direct: true
      wait: {type: download, interval_seconds: 1, timeout_seconds: 30}
      result: {url: /download, auth: connection, format: csv}
`
	return strings.Replace(s, "id: string", "id: {path: 'Record ID', type: string}", 1)
}

func TestDirectExportCSVReplay(t *testing.T) {
	var complete atomic.Bool
	var requests atomic.Int32
	src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != "GET" || r.URL.Path != "/download" || r.Header.Get("Authorization") != "Bearer secret-api-key" {
			t.Errorf("unexpected direct request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(400)
			return
		}
		fmt.Fprint(w, "Record ID,Extra\r\none,first\r\n")
		if complete.Load() {
			fmt.Fprint(w, "two,second\r\n")
		} else {
			fmt.Fprint(w, "two,\"unterminated")
		}
	}, directExportFixture)
	var first collectSink
	if err := src.Extract(t.Context(), &first, filament.ExtractOpts{}); err == nil || len(first.records) != 1 {
		t.Fatalf("partial CSV: rows=%d err=%v", len(first.records), err)
	}
	if first.records[0].ID != "one" || !strings.Contains(string(first.records[0].Data), "first") {
		t.Fatalf("projection lost CSV fields: %s", first.records[0].Data)
	}
	plan, err := src.PlanResume(t.Context(), []string{"items"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := checkpoint.ParseKeyset(plan["items"])
	cp.Shards[0].Key = first.records[0].Key
	plan["items"] = cp.ToCheckpoint("items")
	complete.Store(true)
	var resumed collectSink
	if err := src.ExtractFrom(t.Context(), &resumed, filament.ExtractOpts{}, plan); err != nil {
		t.Fatal(err)
	}
	if requests.Load() != 2 || len(resumed.records) != 2 || resumed.records[0].ID != "one" {
		t.Fatal("direct export did not replay from its beginning")
	}
	var state export.JobCheckpoint
	if err := json.Unmarshal([]byte(resumed.checkpoints["items"][0]), &state); err != nil || state.Phase != "done" {
		t.Fatalf("completion: %+v %v", state, err)
	}
}

func TestDirectExportEmptyAndErrors(t *testing.T) {
	for _, status := range []int{200, 403, 404} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var requests atomic.Int32
			src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
				fmt.Fprint(w, "Record ID,Extra\r\n")
			}, directExportFixture)
			var sink collectSink
			err := src.Extract(t.Context(), &sink, filament.ExtractOpts{})
			if (err == nil) != (status == 200) || requests.Load() != 1 || len(sink.records) != 0 {
				t.Fatalf("requests=%d rows=%d err=%v", requests.Load(), len(sink.records), err)
			}
			var state export.JobCheckpoint
			if err := json.Unmarshal([]byte(sink.checkpoints["items"][0]), &state); err != nil {
				t.Fatal(err)
			}
			if (state.Phase == "done") != (status == 200) {
				t.Fatalf("unexpected checkpoint phase: %s", state.Phase)
			}
		})
	}
}

func TestDirectExportValidation(t *testing.T) {
	base := directExportFixture(exportTestManifest)
	if _, err := manifest.Parse([]byte(base)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ old, new string }{
		{"direct: true", "direct: false"},
		{"direct: true", "direct: true\n      start: {method: GET, path: /start, capture: {id: id}}"},
		{"url: /download", "url: '{{ job.url }}'"},
		{"url: /download", "url: '{{ parent.url }}'"},
		{"type: download", "type: job"},
		{"primary_key: [id]", "incremental: {cursor_field: id, start_param: since, inject_into: query}\n    primary_key: [id]"},
	} {
		t.Run(tc.new, func(t *testing.T) {
			if _, err := manifest.Parse([]byte(strings.Replace(base, tc.old, tc.new, 1))); err == nil {
				t.Fatal("accepted invalid direct export")
			}
		})
	}
}

// A GDELT-style export: a ZIP holding one tab-separated file with no header
// row, whose unquoted values may contain a stray double quote.
func headerlessTSVFixture(s string) string {
	s = directExportFixture(s)
	return strings.Replace(s, "format: csv}", `archive: zip, files: '*.export.CSV', format: csv,
        csv: {delimiter: "\t", header: false, columns: ['Record ID', Title, Url], quoting: none}}`, 1)
}

func TestDirectExportHeaderlessTSV(t *testing.T) {
	var archive bytes.Buffer
	zw := zip.NewWriter(&archive)
	w, _ := zw.Create("20261006220000.export.CSV")
	fmt.Fprint(w, "one\t\"Starts quoted\thttps://a.example/x\ntwo\tsays \"hi\"\thttps://b.example/y\n")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(archive.Bytes())
	}, headerlessTSVFixture)
	var sink collectSink
	if err := src.Extract(t.Context(), &sink, filament.ExtractOpts{}); err != nil {
		t.Fatal(err)
	}
	if len(sink.records) != 2 || sink.records[0].ID != "one" || sink.records[1].ID != "two" {
		t.Fatalf("records = %d", len(sink.records))
	}
	for i, want := range []string{`"Title":"\"Starts quoted"`, `"Title":"says \"hi\""`} {
		if !strings.Contains(string(sink.records[i].Data), want) {
			t.Fatalf("record %d lost its quote: %s", i, sink.records[i].Data)
		}
	}
}

func TestExportCSVDialectValidation(t *testing.T) {
	base := headerlessTSVFixture(exportTestManifest)
	if _, err := manifest.Parse([]byte(base)); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, new string }{
		{"columns-with-header", "header: false, ", ""},
		{"header-false-no-columns", "columns: ['Record ID', Title, Url], ", ""},
		{"duplicate-columns", "Title, Url]", "Title, Title]"},
		{"empty-column", "Title, Url]", "Title, '']"},
		{"two-char-delimiter", `delimiter: "\t"`, `delimiter: "ab"`},
		{"quote-delimiter", `delimiter: "\t"`, `delimiter: '"'`},
		{"newline-delimiter", `delimiter: "\t"`, `delimiter: "\n"`},
		{"unknown-quoting", "quoting: none", "quoting: lazy"},
		{"unknown-field", "quoting: none", "quoting: none, comment: '#'"},
		{"not-csv", "format: csv,", "format: ndjson,"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := manifest.Parse([]byte(strings.Replace(base, tc.old, tc.new, 1))); err == nil {
				t.Fatal("accepted invalid CSV dialect")
			}
		})
	}
}
