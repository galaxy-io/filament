package encoder

import (
	"fmt"
	"strings"
)

// Compression identifies a file compression mode.
type Compression string

// Supported compression modes.
const (
	CompressionNone   Compression = "none"
	CompressionGZIP   Compression = "gzip"
	CompressionSnappy Compression = "snappy"
)

// ParseCompression normalizes a configured compression mode.
func ParseCompression(value string) (Compression, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "none", "uncompressed":
		return CompressionNone, nil
	case "gzip", "gz":
		return CompressionGZIP, nil
	case "snappy":
		return CompressionSnappy, nil
	default:
		return "", fmt.Errorf("unsupported compression %q", value)
	}
}

// ContentEncoding returns the HTTP content coding stored on the object.
func (c Compression) ContentEncoding() string {
	if c == CompressionGZIP {
		return "gzip"
	}
	return ""
}
