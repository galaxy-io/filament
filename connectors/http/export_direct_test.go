package httpapi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
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

// A GDELT-style locator: a plain-text file of "<size> <md5> <url>" lines whose
// URLs name a new file every fifteen minutes.
func locateExportFixture(s string) string {
	return strings.Replace(directExportFixture(s), "      result: {url: /download,", `      locate:
        request: {method: GET, path: /lastupdate.txt}
        capture: {export_file: {regex: 'gdeltv2/(\d{14}\.export\.CSV\.zip)'}}
      result: {url: '/files/{{ job.export_file }}',`, 1)
}

func lastUpdate(release string) string {
	return "150383 297a16b493de7cf6ca809a7cc31d0b93 http://data.gdeltproject.org/gdeltv2/" + release + ".export.CSV.zip\n" +
		"318084 bb27f78ba45f69a17ea6ed7755e9f8ff http://data.gdeltproject.org/gdeltv2/" + release + ".mentions.CSV.zip\n" +
		"10768507 ea8dde0beb0ba98810a92db068c0ce99 http://data.gdeltproject.org/gdeltv2/" + release + ".gkg.csv.zip\n"
}

func TestDirectExportLocateReplaysLocatedFile(t *testing.T) {
	var release atomic.Value
	release.Store("20261006220000")
	var complete atomic.Bool
	var locates atomic.Int32
	var mu sync.Mutex
	var downloads []string
	src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/lastupdate.txt" {
			locates.Add(1)
			fmt.Fprint(w, lastUpdate(release.Load().(string)))
			return
		}
		mu.Lock()
		downloads = append(downloads, r.URL.Path)
		mu.Unlock()
		fmt.Fprint(w, "Record ID,Extra\r\none,first\r\n")
		if complete.Load() {
			fmt.Fprint(w, "two,second\r\n")
		} else {
			fmt.Fprint(w, "two,\"unterminated")
		}
	}, locateExportFixture)
	var first collectSink
	if err := src.Extract(t.Context(), &first, filament.ExtractOpts{}); err == nil || len(first.records) != 1 {
		t.Fatalf("partial CSV: rows=%d err=%v", len(first.records), err)
	}
	plan, err := src.PlanResume(t.Context(), []string{"items"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cp, _ := checkpoint.ParseKeyset(plan["items"])
	cp.Shards[0].Key = first.records[0].Key
	plan["items"] = cp.ToCheckpoint("items")
	// The locator moves on before the retry. Recovery must finish the file it
	// started rather than switch to the newer one.
	release.Store("20261006221500")
	complete.Store(true)
	var resumed collectSink
	if err := src.ExtractFrom(t.Context(), &resumed, filament.ExtractOpts{}, plan); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := "/files/20261006220000.export.CSV.zip"
	if locates.Load() != 1 || len(downloads) != 2 || downloads[0] != want || downloads[1] != want {
		t.Fatalf("locates=%d downloads=%v, want one locate and two downloads of %s", locates.Load(), downloads, want)
	}
	if len(resumed.records) != 2 || resumed.records[1].ID != "two" {
		t.Fatalf("resumed rows = %d", len(resumed.records))
	}
}

func TestDirectExportLocateFailures(t *testing.T) {
	for name, locator := range map[string]func(http.ResponseWriter){
		"no-match":  func(w http.ResponseWriter) { fmt.Fprint(w, "nothing to download\n") },
		"not-found": func(w http.ResponseWriter) { w.WriteHeader(404) },
	} {
		t.Run(name, func(t *testing.T) {
			var downloads atomic.Int32
			src, _ := exportSource(t, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/lastupdate.txt" {
					locator(w)
					return
				}
				downloads.Add(1)
			}, locateExportFixture)
			var sink collectSink
			err := src.Extract(t.Context(), &sink, filament.ExtractOpts{})
			if err == nil || downloads.Load() != 0 || len(sink.checkpoints["items"]) != 0 {
				t.Fatalf("downloads=%d checkpoints=%v err=%v", downloads.Load(), sink.checkpoints["items"], err)
			}
		})
	}
}

func TestDirectExportLocateValidation(t *testing.T) {
	base := locateExportFixture(exportTestManifest)
	if _, err := manifest.Parse([]byte(base)); err != nil {
		t.Fatal(err)
	}
	jobLocate := strings.Replace(exportTestManifest, "      start:", `      locate:
        request: {method: GET, path: /lastupdate.txt}
        capture: {file: {regex: '(\S+)'}}
      start:`, 1)
	if _, err := manifest.Parse([]byte(jobLocate)); err == nil {
		t.Fatal("accepted locate without direct: true")
	}
	for _, tc := range []struct{ name, old, new string }{
		{"no-group", `'gdeltv2/(\d{14}\.export\.CSV\.zip)'`, `'gdeltv2/\d{14}'`},
		{"bad-regex", `'gdeltv2/(\d{14}\.export\.CSV\.zip)'`, `'gdeltv2/(\d{14}'`},
		{"empty-regex", `'gdeltv2/(\d{14}\.export\.CSV\.zip)'`, `''`},
		{"no-captures", `{export_file: {regex: 'gdeltv2/(\d{14}\.export\.CSV\.zip)'}}`, `{}`},
		{"post", "request: {method: GET, path: /lastupdate.txt}", "request: {method: POST, path: /lastupdate.txt}"},
		{"absolute-path", "path: /lastupdate.txt}", "path: 'https://example.com/lastupdate.txt'}"},
		{"job-template", "path: /lastupdate.txt}", "path: '/{{ job.export_file }}'}"},
		{"unknown-field", "capture: {export_file:", "pattern: x\n        capture: {export_file:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := manifest.Parse([]byte(strings.Replace(base, tc.old, tc.new, 1))); err == nil {
				t.Fatal("accepted invalid locator")
			}
		})
	}
}
