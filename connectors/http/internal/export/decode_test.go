package export

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"

	"github.com/galaxy-io/filament/connectors/http/manifest"
)

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

func TestExportURLValidation(t *testing.T) {
	for _, u := range []string{"http://files.example/a", "https://user:pass@files.example/a", "https://other.example/a", "https://files.example.evil/a", "https://files.example/a#fragment"} {
		if _, err := validateExportURL(u, []string{"files.example"}); err == nil {
			t.Fatalf("accepted %s", u)
		}
	}
	if _, err := validateExportURL("https://files.example/a?signature=secret", []string{"files.example"}); err != nil {
		t.Fatal(err)
	}
}

func TestExportDecoders(t *testing.T) {
	largeID := `[{"id":9007199254740993}]`
	var zipped bytes.Buffer
	zw := zip.NewWriter(&zipped)
	for _, name := range []string{"a.json", "b.json"} {
		f, _ := zw.Create(name)
		io.WriteString(f, "{\"id\":9007199254740993}\n")
	}
	zw.Close()
	var tarred bytes.Buffer
	tw := tar.NewWriter(&tarred)
	body := `[{"status_code":200,"response":"{\"items\":[{\"id\":9007199254740993}]}"}]`
	tw.WriteHeader(&tar.Header{Name: "part.json", Mode: 0o600, Size: int64(len(body))})
	io.WriteString(tw, body)
	tw.Close()
	for _, tc := range []struct {
		name string
		data []byte
		spec manifest.ExportResult
		rows int
	}{
		{"gzip", gzipExport(t, largeID), manifest.ExportResult{Format: "json", Compression: "gzip"}, 1},
		{"nested", []byte(`{"ignored":[1,2],"data":{"items":` + largeID + `},"tail":true}`), manifest.ExportResult{Format: "json", RecordsPath: "$.data.items"}, 1},
		{"zip", zipped.Bytes(), manifest.ExportResult{Archive: "zip", Files: "*.json", Format: "ndjson"}, 2},
		{"tar-envelope", gzipExport(t, tarred.String()), manifest.ExportResult{Archive: "tar", Compression: "gzip", Files: "*.json", Format: "json", Envelope: &manifest.ExportEnvelope{StatusPath: "status_code", Success: []int{200}, BodyPath: "response", RecordsPath: "items"}}, 1},
		{"csv", []byte("id,name\n9007199254740993,Example\n"), manifest.ExportResult{Format: "csv"}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			artifact := &artifactReader{Reader: bytes.NewReader(tc.data)}
			err := decodeExport(t.Context(), artifact, tc.spec, func(row map[string]any) error {
				count++
				if fmt.Sprint(row["id"]) != "9007199254740993" {
					t.Fatalf("ID corrupted: %v", row)
				}
				return nil
			})
			if err != nil || count != tc.rows {
				t.Fatalf("rows=%d err=%v", count, err)
			}
			if artifact.bytes != int64(len(tc.data)) {
				t.Fatalf("download bytes = %d, want %d", artifact.bytes, len(tc.data))
			}
		})
	}
}

func TestExportDecoderRejectsIncompleteResults(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		spec      manifest.ExportResult
	}{
		{"truncated", `[{"id":1}`, manifest.ExportResult{Format: "json"}},
		{"truncated-tail", `{"items":[],"tail":`, manifest.ExportResult{Format: "json", RecordsPath: "items"}},
		{"missing-path", `{}`, manifest.ExportResult{Format: "json", RecordsPath: "items"}},
		{"null-record", `[null]`, manifest.ExportResult{Format: "json"}},
		{"trailing", `[] garbage`, manifest.ExportResult{Format: "json"}},
		{"ndjson-trailing", "{\"id\":1} {}\n", manifest.ExportResult{Format: "ndjson"}},
		{"download-limit", `[{"id":1}]`, manifest.ExportResult{Format: "json", MaxDownloadBytes: 4}},
		{"expanded-limit", `[{"id":1}]`, manifest.ExportResult{Format: "json", MaxUncompressedBytes: 4}},
		{"duplicate-csv", "id,id\n1,2\n", manifest.ExportResult{Format: "csv"}},
		{"operation-failure", `[{"status":500,"response":"[]"}]`, manifest.ExportResult{Format: "json", Envelope: &manifest.ExportEnvelope{StatusPath: "status", Success: []int{200}, BodyPath: "response"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := decodeExport(t.Context(), strings.NewReader(tc.raw), tc.spec, func(map[string]any) error { return nil }); err == nil {
				t.Fatal("accepted incomplete export")
			}
		})
	}
}

func TestExportArchiveLimitsAndChecksums(t *testing.T) {
	for _, name := range []string{"../bad.json", "other.txt", "data.json"} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			zw := zip.NewWriter(&b)
			w, _ := zw.Create(name)
			io.WriteString(w, strings.Repeat("{\"id\":1}\n", 20))
			zw.Close()
			spec := manifest.ExportResult{Archive: "zip", Files: "*.json", Format: "ndjson", MaxUncompressedBytes: 10}
			if err := decodeExport(t.Context(), &b, spec, func(map[string]any) error { return nil }); err == nil {
				t.Fatal("expected archive validation failure")
			}
		})
	}
	data := gzipExport(t, "[]")
	data[len(data)-8] ^= 1
	if err := decodeExport(t.Context(), bytes.NewReader(data), manifest.ExportResult{Compression: "gzip", Format: "json"}, func(map[string]any) error { return nil }); err == nil {
		t.Fatal("ignored gzip checksum")
	}
}

// Verify the URL parser itself never appears in a public error with a token.
func TestExportTransportErrorRedaction(t *testing.T) {
	err := TransportError(context.Background(), &url.Error{Op: "Get", URL: "https://files.example/?token=secret", Err: io.ErrUnexpectedEOF})
	if strings.Contains(err.Error(), "secret") {
		t.Fatal("signed URL leaked")
	}
}

func TestExportRecordSizeLimit(t *testing.T) {
	for _, format := range []string{"json", "ndjson", "csv"} {
		t.Run(format, func(t *testing.T) {
			huge := strings.Repeat("x", maxExportRecordBytes+1)
			data := `[{"id":"` + huge + `"}]`
			if format == "ndjson" {
				data = `{"id":"` + huge + `"}`
			}
			if format == "csv" {
				data = "id\n" + huge + "\n"
			}
			count := 0
			err := decodeExport(t.Context(), strings.NewReader(data), manifest.ExportResult{Format: format}, func(map[string]any) error { count++; return nil })
			if err == nil || count != 0 {
				t.Fatalf("oversize record emitted: rows=%d err=%v", count, err)
			}
		})
	}
}

func TestExportCSVDialect(t *testing.T) {
	headerless := func(quoting string) *manifest.ExportCSV {
		no := false
		return &manifest.ExportCSV{Delimiter: "\t", Header: &no, Columns: []string{"id", "title", "url"}, Quoting: quoting}
	}
	// GDELT-style rows: tab-separated, no header, never quoted, but values may
	// contain a stray quote, including at the start of a field.
	tsv := "9007199254740993\t\"Starts quoted\thttps://a.example/x\n\n2\tsays \"hi\"\thttps://b.example/y\n"
	for _, tc := range []struct {
		name, raw string
		dialect   *manifest.ExportCSV
		want      []map[string]any
	}{
		{"headed-tab", "id\ttitle\n1\tA\n", &manifest.ExportCSV{Delimiter: "\t"}, []map[string]any{{"id": "1", "title": "A"}}},
		{"headerless-unquoted", tsv, headerless("none"), []map[string]any{
			{"id": "9007199254740993", "title": `"Starts quoted`, "url": "https://a.example/x"},
			{"id": "2", "title": `says "hi"`, "url": "https://b.example/y"},
		}},
		{"headerless-crlf", "1\tA\tu\r\n", headerless("none"), []map[string]any{{"id": "1", "title": "A", "url": "u"}}},
		{"headerless-standard", "1\t\"A\tB\"\tu\n", headerless("standard"), []map[string]any{{"id": "1", "title": "A\tB", "url": "u"}}},
		{"default-unchanged", "id,title\n1,\"A,B\"\n", nil, []map[string]any{{"id": "1", "title": "A,B"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []map[string]any
			err := decodeExport(t.Context(), strings.NewReader(tc.raw), manifest.ExportResult{Format: "csv", CSV: tc.dialect}, func(row map[string]any) error {
				got = append(got, row)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("rows = %v, want %v", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, raw string
		dialect   *manifest.ExportCSV
	}{
		// The reason quoting: none exists: standard parsing reads the opening
		// quote as a quoted field and fails on the GDELT-style rows above.
		{"stray-quote-standard", tsv, headerless("standard")},
		{"short-row", "1\tA\n", headerless("none")},
		{"long-row", "1\tA\tu\textra\n", headerless("none")},
		{"short-row-standard", "1\tA\n", headerless("standard")},
		{"oversize-unquoted", "1\t" + strings.Repeat("x", maxExportRecordBytes+1) + "\tu\n", headerless("none")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			count := 0
			err := decodeExport(t.Context(), strings.NewReader(tc.raw), manifest.ExportResult{Format: "csv", CSV: tc.dialect}, func(map[string]any) error { count++; return nil })
			if err == nil {
				t.Fatalf("accepted malformed CSV: rows=%d", count)
			}
		})
	}
}

// Without a csv block, a short row still fails with encoding/csv's own error,
// which names the line in the file.
func TestExportCSVDefaultRowErrorUnchanged(t *testing.T) {
	err := decodeExport(t.Context(), strings.NewReader("id,name\n1,A\n2\n"), manifest.ExportResult{Format: "csv"}, func(map[string]any) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "record on line 3: wrong number of fields") {
		t.Fatalf("err = %v", err)
	}
}
