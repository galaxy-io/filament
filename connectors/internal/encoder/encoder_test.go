package encoder

import "testing"

func TestFormats(t *testing.T) {
	tests := []struct {
		formatInput      string
		compressionInput string
		want             FileFormat
		wantCompression  Compression
		extension        string
		contentType      string
		contentEncoding  string
	}{
		{"", "", FileFormatNDJSON, CompressionNone, ".ndjson", "application/x-ndjson", ""},
		{"ndjson", "gzip", FileFormatNDJSON, CompressionGZIP, ".ndjson.gz", "application/x-ndjson", "gzip"},
		{"jsonl", "none", FileFormatJSONL, CompressionNone, ".jsonl", "application/x-ndjson", ""},
		{"json", "gzip", FileFormatJSON, CompressionGZIP, ".json.gz", "application/json", "gzip"},
		{"parquet", "none", FileFormatParquet, CompressionNone, ".parquet", "application/vnd.apache.parquet", ""},
	}
	for _, test := range tests {
		format, err := ParseFileFormat(test.formatInput)
		if err != nil {
			t.Fatalf("ParseFileFormat(%q): %v", test.formatInput, err)
		}
		compression, err := ParseCompression(test.compressionInput)
		if err != nil {
			t.Fatalf("ParseCompression(%q): %v", test.compressionInput, err)
		}
		options := Options{FileFormat: format, Compression: compression}
		if format != test.want || compression != test.wantCompression || options.Extension() != test.extension || format.ContentType() != test.contentType || compression.ContentEncoding() != test.contentEncoding {
			t.Fatalf("format metadata = %q, %q, %q, %q, %q", format, compression, options.Extension(), format.ContentType(), compression.ContentEncoding())
		}
	}
	if _, err := ParseFileFormat("csv"); err == nil {
		t.Fatal("ParseFileFormat accepted unsupported format")
	}
	if _, err := ParseCompression("snappy"); err == nil {
		t.Fatal("ParseCompression accepted unsupported compression")
	}
	if err := (Options{FileFormat: FileFormatParquet, Compression: CompressionGZIP}).Validate(); err == nil {
		t.Fatal("Options.Validate accepted gzip-wrapped Parquet")
	}
}
