package s3

import (
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
)

func TestFileOptionsAndObjectKey(t *testing.T) {
	for _, test := range []struct {
		format       string
		compression  string
		wantFormat   encoder.FileFormat
		wantCompress encoder.Compression
		key          string
	}{
		{"", "", encoder.FileFormatNDJSON, encoder.CompressionNone, "exports/run/accounts.ndjson"},
		{"ndjson", "gzip", encoder.FileFormatNDJSON, encoder.CompressionGZIP, "exports/run/accounts.ndjson.gz"},
		{"jsonl", "none", encoder.FileFormatJSONL, encoder.CompressionNone, "exports/run/accounts.jsonl"},
		{"json", "gzip", encoder.FileFormatJSON, encoder.CompressionGZIP, "exports/run/accounts.json.gz"},
		{"parquet", "none", encoder.FileFormatParquet, encoder.CompressionNone, "exports/run/accounts.parquet"},
	} {
		cfg, err := parseConfig(filament.NewConfig(map[string]any{
			"bucket": "bucket", "file_format": test.format, "compression": test.compression,
		}))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.fileFormat != test.wantFormat || cfg.compression != test.wantCompress {
			t.Fatalf("options %q/%q parsed as %q/%q", test.format, test.compression, cfg.fileFormat, cfg.compression)
		}
		if got := encodedObjectKey("exports", "run", "accounts", cfg.fileFormat, cfg.compression); got != test.key {
			t.Fatalf("object key = %q, want %q", got, test.key)
		}
	}
	if _, err := parseConfig(filament.NewConfig(map[string]any{"bucket": "bucket", "file_format": "csv"})); err == nil {
		t.Fatal("parseConfig accepted unsupported file format")
	}
	if _, err := parseConfig(filament.NewConfig(map[string]any{"bucket": "bucket", "file_format": "parquet", "compression": "gzip"})); err == nil {
		t.Fatal("parseConfig accepted gzip-wrapped Parquet")
	}
}

func TestLegacyEncodingCompatibility(t *testing.T) {
	tests := []struct {
		config      map[string]any
		wantFormat  encoder.FileFormat
		compression encoder.Compression
		key         string
	}{
		{map[string]any{"encoding": "json_gzip"}, encoder.FileFormatNDJSON, encoder.CompressionGZIP, "run/accounts.ndjson.gz"},
		{map[string]any{"encoding": "json"}, encoder.FileFormatNDJSON, encoder.CompressionNone, "run/accounts.ndjson"},
		{map[string]any{"file_type": "jsonl"}, encoder.FileFormatNDJSON, encoder.CompressionNone, "run/accounts.ndjson"},
		{map[string]any{"file_type": "parquet"}, encoder.FileFormatParquet, encoder.CompressionNone, "run/accounts.parquet"},
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
		if got := encodedObjectKey("", "run", "accounts", cfg.fileFormat, cfg.compression); got != test.key {
			t.Fatalf("legacy object key = %q, want %q", got, test.key)
		}
	}
}
