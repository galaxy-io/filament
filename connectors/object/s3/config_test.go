package s3

import (
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

// testRun starts late on the 9th in US Eastern so the partition date proves UTC.
var testRun = filament.RunSpec{Run: "run", StartedAt: time.Date(2026, 9, 9, 22, 0, 0, 0, time.FixedZone("EDT", -4*3600))}

func TestFileOptionsAndObjectKey(t *testing.T) {
	for _, test := range []struct {
		format       string
		compression  string
		wantFormat   encoder.FileFormat
		wantCompress encoder.Compression
		key          string
	}{
		{"", "", encoder.FileFormatNDJSON, encoder.CompressionNone, "exports/accounts/dt=2026-09-10/run.ndjson"},
		{"ndjson", "gzip", encoder.FileFormatNDJSON, encoder.CompressionGZIP, "exports/accounts/dt=2026-09-10/run.ndjson.gz"},
		{"jsonl", "none", encoder.FileFormatJSONL, encoder.CompressionNone, "exports/accounts/dt=2026-09-10/run.jsonl"},
		{"json", "gzip", encoder.FileFormatJSON, encoder.CompressionGZIP, "exports/accounts/dt=2026-09-10/run.json.gz"},
		{"parquet", "snappy", encoder.FileFormatParquet, encoder.CompressionSnappy, "exports/accounts/dt=2026-09-10/run.parquet"},
	} {
		cfg, err := parseConfig(filament.NewConfig(map[string]any{
			"bucket": "bucket", "prefix": "exports", "file_format": test.format, "compression": test.compression,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.fileFormat != test.wantFormat || cfg.compression != test.wantCompress {
			t.Fatalf("options %q/%q parsed as %q/%q", test.format, test.compression, cfg.fileFormat, cfg.compression)
		}
		layout := newKeyLayout(cfg, testRun)
		if got, err := layout.object("accounts"); err != nil || got != test.key {
			t.Fatalf("object key = %q, %v, want %q", got, err, test.key)
		}
		if got, want := layout.success(), "exports/_runs/run/_SUCCESS.json"; got != want {
			t.Fatalf("success key = %q, want %q", got, want)
		}
	}
	if _, err := parseConfig(filament.NewConfig(map[string]any{"bucket": "bucket", "file_format": "csv"})); err == nil {
		t.Fatal("parseConfig accepted unsupported file format")
	}
	if _, err := parseConfig(filament.NewConfig(map[string]any{"bucket": "bucket", "file_format": "parquet", "compression": "gzip"})); err == nil {
		t.Fatal("parseConfig accepted unsupported Parquet compression")
	}
}

func TestLegacyEncodingCompatibility(t *testing.T) {
	tests := []struct {
		config      map[string]any
		wantFormat  encoder.FileFormat
		compression encoder.Compression
		key         string
	}{
		{map[string]any{"encoding": "json_gzip"}, encoder.FileFormatNDJSON, encoder.CompressionGZIP, "accounts/dt=2026-09-10/run.ndjson.gz"},
		{map[string]any{"encoding": "json"}, encoder.FileFormatNDJSON, encoder.CompressionNone, "accounts/dt=2026-09-10/run.ndjson"},
		{map[string]any{"file_type": "jsonl"}, encoder.FileFormatNDJSON, encoder.CompressionNone, "accounts/dt=2026-09-10/run.ndjson"},
		{map[string]any{"file_type": "parquet"}, encoder.FileFormatParquet, encoder.CompressionSnappy, "accounts/dt=2026-09-10/run.parquet"},
	}
	for _, test := range tests {
		test.config["bucket"] = "bucket"
		cfg, err := parseConfig(filament.NewConfig(test.config))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.fileFormat != test.wantFormat || cfg.compression != test.compression {
			t.Fatalf("legacy options parsed as %q/%q", cfg.fileFormat, cfg.compression)
		}
		if got, err := newKeyLayout(cfg, testRun).object("accounts"); err != nil || got != test.key {
			t.Fatalf("legacy object key = %q, %v, want %q", got, err, test.key)
		}
	}
}

func TestPartitionTemplate(t *testing.T) {
	for _, test := range []struct {
		partition string
		key       string
	}{
		{"dt={{.Date}}", "exports/accounts/dt=2026-09-10/run.ndjson"},
		{"", "exports/accounts/run.ndjson"},
		{"/month={{.StartedAt.Format \"2006-01\"}}/", "exports/accounts/month=2026-09/run.ndjson"},
		{"{{.StartedAt.Format \"2006/01/02\"}}", "exports/accounts/2026/09/10/run.ndjson"},
		{"{{.Resource}}/{{.Run}}", "exports/accounts/accounts/run/run.ndjson"},
	} {
		cfg, err := parseConfig(filament.NewConfig(map[string]any{"bucket": "bucket", "prefix": "exports", "partition": test.partition}))
		if err != nil {
			t.Fatalf("partition %q: %v", test.partition, err)
		}
		if got, err := newKeyLayout(cfg, testRun).object("accounts"); err != nil || got != test.key {
			t.Fatalf("partition %q key = %q, %v, want %q", test.partition, got, err, test.key)
		}
	}
	for _, partition := range []string{"{{.Date", "{{.Nope}}", "a//b", "../{{.Date}}", "{{.Date}}/.", "{{if .Date}}{{end}}/x"} {
		if _, err := parseConfig(filament.NewConfig(map[string]any{"bucket": "bucket", "partition": partition})); err == nil {
			t.Fatalf("parseConfig accepted partition %q", partition)
		}
	}
}
