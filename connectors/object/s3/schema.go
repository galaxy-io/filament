package s3

import (
	"context"
	"fmt"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	gzipencoder "github.com/galaxy-io/filament/connectors/internal/gzip"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
	parquetencoder "github.com/galaxy-io/filament/connectors/internal/parquet"
	"github.com/galaxy-io/filament/rowmodel"
)

// resourceEncoder serializes the stateful byte stream for one resource.
type resourceEncoder struct {
	mu      sync.Mutex
	schema  *arrow.Schema
	encoder encoder.Encoder
	hasRows bool
}

// EnsureSchema prepares a resource encoder before extraction. This is required
// to produce a valid empty Parquet file when a resource has no batches.
func (s *Sink) EnsureSchema(_ context.Context, resource string, schema rowmodel.Schema) error {
	if resource == "" {
		return fmt.Errorf("s3 sink: resource is required")
	}
	_, err := s.encoderFor(resource, arrowbatch.Schema(schema))
	return err
}

func (s *Sink) encoderFor(resource string, schema *arrow.Schema) (*resourceEncoder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateOpen && s.state != stateCommitting {
		return nil, fmt.Errorf("s3 sink: encoder requires an open run")
	}
	if enc := s.enc[resource]; enc != nil {
		if !schema.Equal(enc.schema) {
			return nil, fmt.Errorf("s3 sink: schema changed for resource %q", resource)
		}
		return enc, nil
	}
	stream, err := newEncoder(s.format, s.compression, schema)
	if err != nil {
		return nil, fmt.Errorf("s3 sink: create %s/%s encoder for %s: %w", s.format, s.compression, resource, err)
	}
	enc := &resourceEncoder{schema: schema, encoder: stream}
	s.enc[resource] = enc
	return enc, nil
}

func newEncoder(format encoder.FileFormat, compression encoder.Compression, schema *arrow.Schema) (encoder.Encoder, error) {
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
