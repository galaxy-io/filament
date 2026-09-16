package object

import (
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/connectors/internal/encoder"
	gzipencoder "github.com/galaxy-io/filament/connectors/internal/gzip"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
	parquetencoder "github.com/galaxy-io/filament/connectors/internal/parquet"
)

// NewEncoder constructs a streaming encoder shared by object sinks.
func NewEncoder(format encoder.FileFormat, compression encoder.Compression, schema *arrow.Schema) (encoder.Encoder, error) {
	options := encoder.Options{FileFormat: format, Compression: compression}
	if err := options.Validate(); err != nil {
		return nil, err
	}
	var stream encoder.Encoder
	switch format {
	case encoder.FileFormatNDJSON, encoder.FileFormatJSONL:
		stream = jsonencoder.NewEncoder(schema)
	case encoder.FileFormatJSON:
		stream = jsonencoder.NewArrayEncoder(schema)
	case encoder.FileFormatParquet:
		return parquetencoder.NewEncoder(schema, compression)
	default:
		return nil, fmt.Errorf("unsupported file format %q", format)
	}
	if compression == encoder.CompressionGZIP {
		stream = gzipencoder.NewEncoder(stream)
	}
	return stream, nil
}
