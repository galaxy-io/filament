package s3

import (
	"context"
	"fmt"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	object "github.com/galaxy-io/filament/connectors/object/internal"
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
	stream, err := object.NewEncoder(s.format, s.compression, schema)
	if err != nil {
		return nil, fmt.Errorf("s3 sink: create %s/%s encoder for %s: %w", s.format, s.compression, resource, err)
	}
	enc := &resourceEncoder{schema: schema, encoder: stream}
	s.enc[resource] = enc
	return enc, nil
}
