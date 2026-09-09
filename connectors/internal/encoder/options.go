package encoder

import "fmt"

// Options selects a file representation and optional outer compression.
type Options struct {
	FileFormat  FileFormat
	Compression Compression
}

// DefaultOptions returns the backward-compatible NDJSON configuration.
func DefaultOptions() Options {
	return Options{FileFormat: DefaultFileFormat, Compression: DefaultFileFormat.DefaultCompression()}
}

// Validate checks whether the selected format supports the compression mode.
func (o Options) Validate() error {
	if !o.FileFormat.SupportsCompression(o.Compression) {
		return fmt.Errorf("compression %q is not supported for file format %q", o.Compression, o.FileFormat)
	}
	return nil
}

// Extension returns the complete suffix for the selected options.
func (o Options) Extension() string {
	extension := o.FileFormat.Extension()
	if o.Compression == CompressionGZIP {
		extension += ".gz"
	}
	return extension
}
