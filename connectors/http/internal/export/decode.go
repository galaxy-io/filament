package export

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"

	"github.com/tidwall/gjson"

	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const (
	defaultExportDownloadBytes     int64 = 1 << 30
	defaultExportUncompressedBytes int64 = 10 << 30
	maxExportRecordBytes                 = 10 << 20
	maxExportArchiveFiles                = 10000
)

// Unlike LimitReader, exhausting the budget is an error, not a successful EOF.
// It also checks cancellation while reading local ZIP files after downloading.
type exportLimitedReader struct {
	ctx       context.Context
	r         io.Reader
	remaining int64
}

func (r *exportLimitedReader) Read(p []byte) (int, error) {
	if r.remaining < 0 {
		return 0, fmt.Errorf("export byte limit exceeded")
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if int64(len(p)) > r.remaining+1 {
		p = p[:r.remaining+1]
	}
	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	if r.remaining < 0 {
		return 0, fmt.Errorf("export byte limit exceeded")
	}
	return n, err
}

func decodeExport(ctx context.Context, body io.Reader, spec manifest.ExportResult, emit func(map[string]any) error) error {
	downloadLimit := spec.MaxDownloadBytes
	if downloadLimit == 0 {
		downloadLimit = defaultExportDownloadBytes
	}
	expandedLimit := spec.MaxUncompressedBytes
	if expandedLimit == 0 {
		expandedLimit = defaultExportUncompressedBytes
	}
	input := &exportLimitedReader{ctx: ctx, r: body, remaining: downloadLimit}
	consume := func(r io.Reader) error { return decodeExportRecords(ctx, r, spec, emit) }
	if spec.Archive == "zip" {
		return decodeExportZIP(ctx, input, expandedLimit, spec.Files, consume)
	}
	var decoded io.Reader = input
	if spec.Compression == "gzip" {
		gz, err := gzip.NewReader(input)
		if err != nil {
			return fmt.Errorf("open gzip: %w", err)
		}
		defer func() { _ = gz.Close() }()
		decoded = gz
	}
	expanded := &exportLimitedReader{ctx: ctx, r: decoded, remaining: expandedLimit}
	if spec.Archive == "tar" {
		tr := tar.NewReader(expanded)
		matched := 0
		for count := 0; ; count++ {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return fmt.Errorf("read tar: %w", err)
			}
			if count >= maxExportArchiveFiles {
				return fmt.Errorf("export archive has too many files")
			}
			if h.Typeflag == tar.TypeDir {
				if !safeExportMember(strings.TrimSuffix(h.Name, "/")) {
					return fmt.Errorf("export archive contains an unsafe directory")
				}
				continue
			}
			if h.Typeflag != tar.TypeReg || !safeExportMember(h.Name) {
				return fmt.Errorf("export archive contains an unsupported member")
			}
			match, _ := path.Match(spec.Files, h.Name)
			if !match {
				continue
			}
			matched++
			if err := consume(tr); err != nil {
				return err
			}
		}
		if matched == 0 {
			return fmt.Errorf("export archive contains no matching files")
		}
	} else if err := consume(expanded); err != nil {
		return err
	}
	// Force gzip checksum/trailer and byte-budget validation even after tar EOF.
	_, err := io.Copy(io.Discard, expanded)
	return err
}

func safeExportMember(name string) bool { return fs.ValidPath(name) && !strings.Contains(name, "\\") }

func decodeExportZIP(ctx context.Context, input io.Reader, budget int64, pattern string, consume func(io.Reader) error) error {
	f, err := os.CreateTemp("", "filament-export-*.zip")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	defer func() { _ = f.Close() }()
	size, err := io.Copy(f, input)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(f, size)
	if err != nil {
		return fmt.Errorf("open ZIP: %w", err)
	}
	if len(zr.File) > maxExportArchiveFiles {
		return fmt.Errorf("export archive has too many files")
	}
	matched := 0
	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			if !safeExportMember(strings.TrimSuffix(file.Name, "/")) {
				return fmt.Errorf("export archive contains an unsafe directory")
			}
			continue
		}
		if !file.Mode().IsRegular() || !safeExportMember(file.Name) {
			return fmt.Errorf("export archive contains an unsupported member")
		}
		match, _ := path.Match(pattern, file.Name)
		if !match {
			continue
		}
		matched++
		r, err := file.Open()
		if err != nil {
			return fmt.Errorf("open ZIP member: %w", err)
		}
		limited := &exportLimitedReader{ctx: ctx, r: r, remaining: budget}
		err = consume(limited)
		if err == nil {
			_, err = io.Copy(io.Discard, limited)
		}
		_ = r.Close()
		if err != nil {
			return err
		}
		budget = limited.remaining
	}
	if matched == 0 {
		return fmt.Errorf("export archive contains no matching files")
	}
	return nil
}

func decodeExportRecords(ctx context.Context, r io.Reader, spec manifest.ExportResult, emit func(map[string]any) error) error {
	deliver := func(row map[string]any) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if row == nil {
			return fmt.Errorf("export record must be an object")
		}
		if spec.Envelope != nil {
			return decodeExportEnvelope(ctx, row, *spec.Envelope, emit)
		}
		return emit(row)
	}
	switch spec.Format {
	case "json":
		return decodeExportJSON(ctx, r, spec.RecordsPath, deliver)
	case "ndjson":
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 64*1024), maxExportRecordBytes)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var row map[string]any
			dec := json.NewDecoder(strings.NewReader(line))
			dec.UseNumber()
			if err := dec.Decode(&row); err != nil {
				return fmt.Errorf("invalid NDJSON export record")
			}
			if _, err := dec.Token(); err != io.EOF {
				return fmt.Errorf("trailing data in NDJSON export record")
			}
			if err := deliver(row); err != nil {
				return err
			}
		}
		return scanner.Err()
	case "csv":
		recordInput := &exportRecordReader{r: r}
		reader := csv.NewReader(recordInput)
		readRow := func() ([]string, error) {
			recordInput.start = reader.InputOffset()
			row, err := reader.Read()
			if reader.InputOffset()-recordInput.start > maxExportRecordBytes {
				return nil, fmt.Errorf("export CSV record exceeds 10 MiB")
			}
			return row, err
		}
		header, err := readRow()
		if err != nil {
			return fmt.Errorf("read CSV header: %w", err)
		}
		seen := map[string]bool{}
		for _, key := range header {
			if key == "" || seen[key] {
				return fmt.Errorf("CSV headers must be nonempty and unique")
			}
			seen[key] = true
		}
		for {
			values, err := readRow()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read CSV row: %w", err)
			}
			row := make(map[string]any, len(header))
			for i, key := range header {
				row[key] = values[i]
			}
			if err := deliver(row); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unsupported export format")
	}
}

// Walk the complete JSON document, streaming only the selected array. Sibling
// values are skipped token by token, avoiding buffering an unrelated large tree.
// UseNumber preserves numeric IDs; malformed tails and missing paths are errors.
func decodeExportJSON(ctx context.Context, r io.Reader, recordsPath string, emit func(map[string]any) error) error {
	input := &exportRecordReader{r: r}
	dec := &exportJSONDecoder{Decoder: json.NewDecoder(input), input: input}
	dec.UseNumber()
	var parts []string
	if recordsPath != "" && recordsPath != "$" {
		parts = paths.Split(exportPath(recordsPath))
	}
	if err := walkExportJSON(ctx, dec, parts, emit, 0); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return fmt.Errorf("export JSON has trailing or malformed data")
	}
	return nil
}

func walkExportJSON(ctx context.Context, dec *exportJSONDecoder, parts []string, emit func(map[string]any) error, depth int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if depth > 100 {
		return fmt.Errorf("export JSON nesting limit exceeded")
	}
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("read export JSON: %w", err)
	}
	if len(parts) == 0 && emit != nil {
		if tok != json.Delim('[') {
			return fmt.Errorf("export records_path must select an array")
		}
		return readExportJSONArray(ctx, dec, emit)
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		if emit != nil {
			return fmt.Errorf("export records_path is missing")
		}
		return nil
	}
	if delim != '{' && delim != '[' {
		return fmt.Errorf("invalid export JSON delimiter")
	}
	if emit != nil && delim != '{' {
		return fmt.Errorf("export records_path must traverse objects")
	}
	found := false
	for dec.More() {
		var key string
		if delim == '{' {
			t, err := dec.Token()
			if err != nil {
				return err
			}
			var ok bool
			key, ok = t.(string)
			if !ok {
				return fmt.Errorf("invalid export JSON key")
			}
		}
		if emit != nil && key == parts[0] {
			if found {
				return fmt.Errorf("duplicate export records_path")
			}
			found = true
			if err := walkExportJSON(ctx, dec, parts[1:], emit, depth+1); err != nil {
				return err
			}
		} else if err := walkExportJSON(ctx, dec, nil, nil, depth+1); err != nil {
			return err
		}
	}
	closing, err := dec.Token()
	expected := json.Delim('}')
	if delim == '[' {
		expected = ']'
	}
	if err != nil || closing != expected {
		return fmt.Errorf("truncated export JSON document")
	}
	if emit != nil && !found {
		return fmt.Errorf("export records_path is missing")
	}
	return nil
}

func readExportJSONArray(ctx context.Context, dec *exportJSONDecoder, emit func(map[string]any) error) error {
	for dec.More() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var row map[string]any
		if err := dec.Decode(&row); err != nil {
			return fmt.Errorf("invalid JSON export record")
		}
		if row == nil {
			return fmt.Errorf("export record must be an object")
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	closing, err := dec.Token()
	if err != nil || closing != json.Delim(']') {
		return fmt.Errorf("truncated export JSON array")
	}
	return nil
}

func decodeExportEnvelope(ctx context.Context, row map[string]any, spec manifest.ExportEnvelope, emit func(map[string]any) error) error {
	raw, err := json.Marshal(row)
	if err != nil {
		return err
	}
	status := gjson.GetBytes(raw, exportPath(spec.StatusPath))
	if !slices.ContainsFunc(spec.Success, func(code int) bool { return status.String() == fmt.Sprint(code) }) {
		return fmt.Errorf("export operation failed or has an invalid status")
	}
	body := gjson.GetBytes(raw, exportPath(spec.BodyPath))
	if body.Type != gjson.String {
		return fmt.Errorf("export operation body must be a JSON string")
	}
	return decodeExportJSON(ctx, strings.NewReader(body.Str), spec.RecordsPath, emit)
}

// JSON's decoder and CSV's reader may buffer past a record boundary. Track
// offsets across their buffers so a single huge value cannot allocate the
// entire uncompressed export limit before failing.
type exportRecordReader struct {
	r     io.Reader
	read  int64
	start int64
}

func (r *exportRecordReader) Read(p []byte) (int, error) {
	left := int64(maxExportRecordBytes) - (r.read - r.start)
	if left <= 0 {
		return 0, fmt.Errorf("export record exceeds 10 MiB")
	}
	if int64(len(p)) > left {
		p = p[:left]
	}
	n, err := r.r.Read(p)
	r.read += int64(n)
	return n, err
}

type exportJSONDecoder struct {
	*json.Decoder
	input *exportRecordReader
}

func (d *exportJSONDecoder) Token() (json.Token, error) {
	d.input.start = d.InputOffset()
	value, err := d.Decoder.Token()
	if d.InputOffset()-d.input.start > maxExportRecordBytes {
		return nil, fmt.Errorf("export JSON token exceeds 10 MiB")
	}
	return value, err
}

func (d *exportJSONDecoder) Decode(value any) error {
	d.input.start = d.InputOffset()
	err := d.Decoder.Decode(value)
	if d.InputOffset()-d.input.start > maxExportRecordBytes {
		return fmt.Errorf("export record exceeds 10 MiB")
	}
	return err
}
