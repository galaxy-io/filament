package encoder

import (
	"fmt"
	"strings"
)

// FileFormat identifies the representation stored in an object.
type FileFormat string

// Supported object file formats.
const (
	FileFormatNDJSON  FileFormat = "ndjson"
	FileFormatJSONL   FileFormat = "jsonl"
	FileFormatJSON    FileFormat = "json"
	FileFormatParquet FileFormat = "parquet"
	DefaultFileFormat            = FileFormatNDJSON
)

// ParseFileFormat normalizes a configured file format while preserving JSON
// filename variants that share an encoder.
func ParseFileFormat(value string) (FileFormat, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "":
		return DefaultFileFormat, nil
	case string(FileFormatNDJSON):
		return FileFormatNDJSON, nil
	case string(FileFormatJSONL):
		return FileFormatJSONL, nil
	case string(FileFormatJSON):
		return FileFormatJSON, nil
	case string(FileFormatParquet):
		return FileFormatParquet, nil
	default:
		return "", fmt.Errorf("unsupported file format %q", value)
	}
}

// IsLineDelimited reports whether the format stores one JSON object per line.
func (f FileFormat) IsLineDelimited() bool {
	return f == FileFormatNDJSON || f == FileFormatJSONL
}

// Extension returns the format's uncompressed file suffix.
func (f FileFormat) Extension() string {
	switch f {
	case FileFormatJSONL:
		return ".jsonl"
	case FileFormatJSON:
		return ".json"
	case FileFormatParquet:
		return ".parquet"
	default:
		return ".ndjson"
	}
}

// ContentType returns the media type stored on the object.
func (f FileFormat) ContentType() string {
	switch f {
	case FileFormatJSON:
		return "application/json"
	case FileFormatParquet:
		return "application/vnd.apache.parquet"
	default:
		return "application/x-ndjson"
	}
}
