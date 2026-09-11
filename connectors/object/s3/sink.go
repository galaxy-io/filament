// Package s3 implements a resource-partitioned object sink for Amazon S3 and
// compatible object stores.
//
// Resource objects land at <prefix>/<resource>/<partition>/<run>.<ext> so one
// table per resource can point at <prefix>/<resource>/. The partition
// directories are templated; the resource root, filename, and manifest are not.
// Apply streams full multipart parts and Commit publishes
// <prefix>/_runs/<run>/_SUCCESS.json last.
package s3

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
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
	layout      keyLayout
	session     *multipartSession
	applyWG     sync.WaitGroup
	format      encoder.FileFormat
	compression encoder.Compression
	enc         map[string]*resourceEncoder
}

// New returns an unconfigured sink.
func New() *Sink { return &Sink{state: stateNew, enc: make(map[string]*resourceEncoder)} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (s *Sink) Name() string { return "s3" }

// Validate performs pure connector-specific validation and no network I/O.
func (s *Sink) Validate(cfg filament.Config) error {
	_, err := parseConfig(cfg)
	return err
}

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
	s.layout = newKeyLayout(cfg, run)
	s.format = cfg.fileFormat
	s.compression = cfg.compression
	s.session = newMultipartSession(ctx, store, cfg.bucket, cfg.partSize, cfg.uploadWorkers, objectMetadata{
		contentType: cfg.fileFormat.ContentType(), contentEncoding: cfg.compression.ContentEncoding(),
	})
	s.applyWG = sync.WaitGroup{}
	s.enc = make(map[string]*resourceEncoder)
	return nil
}

// Commit completes every resource object and publishes _SUCCESS.json last.
func (s *Sink) Commit(ctx context.Context) error {
	session, bucket, layout, err := s.beginCommit()
	if err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, session.Cancel)
	defer stop()

	if err := s.finalizeEncoders(ctx, session); err != nil {
		return err
	}
	results, err := session.Complete(ctx)
	if err != nil {
		return err
	}
	manifest := successManifest{Version: 1, Run: string(layout.run), Resources: make([]manifestResource, len(results))}
	for i, result := range results {
		manifest.Resources[i] = manifestResource{
			Name: result.resource, Key: result.key, URI: fmt.Sprintf("s3://%s/%s", bucket, result.key),
			Rows: result.rows, Bytes: result.bytes, CRC32C: fmt.Sprintf("%08x", result.crc32c),
		}
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("s3 sink: encode success manifest: %w", err)
	}
	if err := session.PutObject(ctx, layout.success(), objectMetadata{contentType: manifestContentType}, bytes.NewReader(body), int64(len(body))); err != nil {
		return fmt.Errorf("s3 sink: publish success marker: %w", err)
	}

	s.mu.Lock()
	s.state = stateCommitted
	s.mu.Unlock()
	session.Cancel()
	return nil
}

// Abort cancels in-flight requests and abandons every incomplete multipart upload.
func (s *Sink) Abort(ctx context.Context) error {
	s.mu.Lock()
	if s.state == stateAborted || s.state == stateCommitted {
		s.mu.Unlock()
		return nil
	}
	s.state = stateAborted
	session := s.session
	s.mu.Unlock()
	if session == nil {
		return nil
	}
	session.Cancel()
	s.applyWG.Wait()
	return session.Abort(ctx)
}
