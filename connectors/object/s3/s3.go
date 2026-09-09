// Package s3 implements a run-versioned object sink for Amazon S3 and
// compatible object stores.
//
// Apply streams full multipart parts and Commit publishes _SUCCESS.json last.
// Consumers must ignore run prefixes without that marker.
package s3

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	gzipencoder "github.com/galaxy-io/filament/connectors/internal/gzip"
	jsonencoder "github.com/galaxy-io/filament/connectors/internal/json"
	parquetencoder "github.com/galaxy-io/filament/connectors/internal/parquet"
	"github.com/galaxy-io/filament/rowmodel"
)

type sinkState uint8

const (
	stateNew sinkState = iota
	stateOpen
	stateCommitting
	stateCommitted
	stateAborted
)

// Sink adapts Filament batches and lifecycle calls to one multipart session.
type Sink struct {
	mu          sync.Mutex
	state       sinkState
	bucket      string
	prefix      string
	run         filament.RunID
	session     *multipartSession
	applyWG     sync.WaitGroup
	format      encoder.FileFormat
	compression encoder.Compression
	enc         map[string]*resourceEncoder
}

// New returns an unconfigured sink.
func New() *Sink { return &Sink{state: stateNew, enc: make(map[string]*resourceEncoder)} }

// resourceEncoder serializes the stateful byte stream for one resource.
type resourceEncoder struct {
	mu      sync.Mutex
	schema  *arrow.Schema
	encoder encoder.Encoder
	hasRows bool
}

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "s3" }

// TestConnection performs a read-only bucket access check.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	parsed, err := parseConfig(cfg)
	if err != nil {
		return err
	}
	store, err := newAWSStore(ctx, parsed)
	if err != nil {
		return err
	}
	if err := store.HeadBucket(ctx, parsed.bucket); err != nil {
		return fmt.Errorf("s3 sink: access bucket %q: %w", parsed.bucket, err)
	}
	return nil
}

// Open creates the S3 transport and starts a fresh multipart session.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg, err := parseConfig(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return err
	}
	store, err := newAWSStore(ctx, cfg)
	if err != nil {
		return err
	}
	return s.open(ctx, run, cfg, store)
}

func (s *Sink) open(ctx context.Context, run filament.RunSpec, cfg sinkConfig, store multipartStore) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == stateOpen || s.state == stateCommitting {
		return fmt.Errorf("s3 sink: a run is already open")
	}
	if s.session != nil {
		s.session.Cancel()
	}

	s.state = stateOpen
	s.bucket = cfg.bucket
	s.prefix = cfg.prefix
	s.run = run.Run
	s.format = cfg.fileFormat
	s.compression = cfg.compression
	s.session = newMultipartSession(ctx, store, cfg.bucket, cfg.partSize, cfg.uploadWorkers, objectMetadata{
		contentType: cfg.fileFormat.ContentType(), contentEncoding: cfg.compression.ContentEncoding(),
	})
	s.applyWG = sync.WaitGroup{}
	s.enc = make(map[string]*resourceEncoder)
	return nil
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

// Write encodes a batch and appends it to the resource's multipart stream.
func (s *Sink) Write(ctx context.Context, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if batch.Resource == "" {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: resource is required")
	}
	session, bucket, key, err := s.beginApply(batch.Resource)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	defer s.applyWG.Done()

	rows := batch.Rows()
	if batch.NumRows() == 0 {
		encodedCRC := uint32(0)
		return filament.WriteReceipt{
			WriteCRC:   batch.IntegrityCRC(),
			EncodedCRC: &encodedCRC,
		}, nil
	}
	scratch := session.takeEncodedBuffer()
	resourceEncoder, err := s.encoderFor(batch.Resource, rows.Schema())
	if err != nil {
		session.releaseEncodedBuffer(scratch)
		return filament.WriteReceipt{}, err
	}
	resourceEncoder.mu.Lock()
	defer resourceEncoder.mu.Unlock()
	encoded, encodedCRC, err := resourceEncoder.encoder.EncodeBatch(scratch, rows)
	if err != nil {
		session.releaseEncodedBuffer(scratch)
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: encode %s: %w", batch.Resource, err)
	}
	defer session.releaseEncodedBuffer(encoded)
	if err := session.Append(ctx, batch.Resource, key, encoded, batch.NumRows(), encodedCRC); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: stream %s: %w", batch.Resource, err)
	}
	resourceEncoder.hasRows = true
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("s3://%s/%s", bucket, key),
		Bytes:      int64(len(encoded)),
		Rows:       batch.NumRows(),
		WriteCRC:   batch.IntegrityCRC(),
		EncodedCRC: &encodedCRC,
	}, nil
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

func (s *Sink) beginApply(resource string) (*multipartSession, string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateOpen || s.session == nil {
		return nil, "", "", fmt.Errorf("s3 sink: write requires an open run")
	}
	s.applyWG.Add(1)
	return s.session, s.bucket, encodedObjectKey(s.prefix, s.run, resource, s.format, s.compression), nil
}

// Apply validates the batch against the run's write policy, then streams it.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case filament.WriteAppend, filament.WriteReplace:
		if err := opts.Policy.ValidateBatch(batch.Resource, batch); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("s3 sink: %w", err)
		}
		return s.Write(ctx, batch)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

func encodedObjectKey(prefix string, run filament.RunID, resource string, format encoder.FileFormat, compression encoder.Compression) string {
	options := encoder.Options{FileFormat: format, Compression: compression}
	return runKey(prefix, run) + "/" + resource + options.Extension()
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
		return parquetencoder.NewEncoder(schema)
	default:
		return nil, fmt.Errorf("unsupported file format %q", format)
	}
	if compression == encoder.CompressionGZIP {
		stream = gzipencoder.NewEncoder(stream)
	}
	return stream, nil
}

// finalizeEncoders appends stream trailers before multipart completion. Parquet
// writes its file footer here; gzip writes its trailer; NDJSON emits nothing.
func (s *Sink) finalizeEncoders(ctx context.Context, session *multipartSession) error {
	s.mu.Lock()
	resources := make([]string, 0, len(s.enc))
	encoders := make(map[string]*resourceEncoder, len(s.enc))
	for resource, encoder := range s.enc {
		resources = append(resources, resource)
		encoders[resource] = encoder
	}
	format, compression, prefix, run := s.format, s.compression, s.prefix, s.run
	s.mu.Unlock()
	sort.Strings(resources)

	for _, resource := range resources {
		resourceEncoder := encoders[resource]
		resourceEncoder.mu.Lock()
		buffer := session.takeEncodedBuffer()
		encoded, crc, err := resourceEncoder.encoder.Finalize(buffer)
		if err == nil && len(encoded) > 0 && resourceEncoder.hasRows {
			err = session.Append(ctx, resource, encodedObjectKey(prefix, run, resource, format, compression), encoded, 0, crc)
		}
		if err != nil {
			session.releaseEncodedBuffer(buffer)
			resourceEncoder.mu.Unlock()
			return fmt.Errorf("s3 sink: finalize %s: %w", resource, err)
		}
		session.releaseEncodedBuffer(encoded)
		resourceEncoder.mu.Unlock()
	}
	return nil
}

func successKey(prefix string, run filament.RunID) string {
	return runKey(prefix, run) + "/_SUCCESS.json"
}

func runKey(prefix string, run filament.RunID) string {
	if prefix == "" {
		return string(run)
	}
	return prefix + "/" + string(run)
}
