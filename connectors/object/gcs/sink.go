// Package gcs implements a resource-partitioned object sink for Google Cloud
// Storage. Resource objects land at
// <prefix>/<resource>/<partition>/<run>.<ext>, and Commit publishes
// <prefix>/_runs/<run>/_SUCCESS.json last.
package gcs

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"cloud.google.com/go/storage"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
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

// Sink adapts Filament batches and lifecycle calls to GCS resumable writers.
type Sink struct {
	mu          sync.Mutex
	state       sinkState
	bucket      string
	layout      keyLayout
	session     *uploadSession
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
func (s *Sink) Name() string { return "gcs" }

// Validate performs pure connector-specific validation and no network I/O.
func (s *Sink) Validate(cfg filament.Config) error {
	_, err := parseConfig(cfg)
	return err
}

// TestConnection performs a read-only bucket metadata request using ADC.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	parsed, err := parseConfig(cfg)
	if err != nil {
		return err
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("gcs sink: create client: %w", err)
	}
	defer func() { _ = client.Close() }()
	if _, err := client.Bucket(parsed.bucket).Attrs(ctx); err != nil {
		return fmt.Errorf("gcs sink: access bucket %q: %w", parsed.bucket, err)
	}
	return nil
}

// Open creates the GCS transport and starts a fresh resumable-upload session.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg, err := parseConfig(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return err
	}
	client, err := storage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("gcs sink: create client: %w", err)
	}
	if err := s.open(ctx, run, cfg, client); err != nil {
		_ = client.Close()
		return err
	}
	return nil
}

func (s *Sink) open(ctx context.Context, run filament.RunSpec, cfg sinkConfig, client *storage.Client) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == stateOpen || s.state == stateCommitting {
		return fmt.Errorf("gcs sink: a run is already open")
	}
	if s.session != nil {
		s.session.Cancel()
	}
	s.state = stateOpen
	s.bucket = cfg.bucket
	s.layout = newKeyLayout(cfg, run)
	s.format = cfg.fileFormat
	s.compression = cfg.compression
	s.session = newUploadSession(ctx, client, cfg)
	s.applyWG = sync.WaitGroup{}
	s.enc = make(map[string]*resourceEncoder)
	return nil
}

func (s *Sink) Write(ctx context.Context, batch *arrowbatch.Batch) (filament.WriteReceipt, error) {
	if batch.Resource == "" {
		return filament.WriteReceipt{}, fmt.Errorf("gcs sink: resource is required")
	}
	session, bucket, key, err := s.beginApply(batch.Resource)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
	defer s.applyWG.Done()
	rows := batch.Rows()
	if batch.NumRows() == 0 {
		encodedCRC := uint32(0)
		return filament.WriteReceipt{WriteCRC: batch.IntegrityCRC(), EncodedCRC: &encodedCRC}, nil
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
		return filament.WriteReceipt{}, fmt.Errorf("gcs sink: encode %s: %w", batch.Resource, err)
	}
	defer session.releaseEncodedBuffer(encoded)
	if err := session.Append(ctx, batch.Resource, key, encoded, batch.NumRows()); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("gcs sink: stream %s: %w", batch.Resource, err)
	}
	resourceEncoder.hasRows = true
	return filament.WriteReceipt{
		URI: fmt.Sprintf("gs://%s/%s", bucket, key), Bytes: int64(len(encoded)), Rows: batch.NumRows(),
		WriteCRC: batch.IntegrityCRC(), EncodedCRC: &encodedCRC,
	}, nil
}

func (s *Sink) beginApply(resource string) (*uploadSession, string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateOpen || s.session == nil {
		return nil, "", "", fmt.Errorf("gcs sink: write requires an open run")
	}
	key, err := s.layout.object(resource)
	if err != nil {
		return nil, "", "", err
	}
	s.applyWG.Add(1)
	return s.session, s.bucket, key, nil
}

// Apply validates the batch against the run's write policy, then streams it.
func (s *Sink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	switch opts.Policy.Capability.Mode {
	case filament.WriteAppend, filament.WriteReplace:
		if err := opts.Policy.ValidateBatch(batch.Resource, batch); err != nil {
			return filament.WriteReceipt{}, fmt.Errorf("gcs sink: %w", err)
		}
		return s.Write(ctx, batch)
	default:
		return filament.WriteReceipt{}, fmt.Errorf("gcs sink: write policy %q is not implemented", opts.Policy.Capability.Mode)
	}
}

func (s *Sink) finalizeEncoders(ctx context.Context, session *uploadSession) error {
	s.mu.Lock()
	resources := make([]string, 0, len(s.enc))
	encoders := make(map[string]*resourceEncoder, len(s.enc))
	for resource, resourceEncoder := range s.enc {
		resources = append(resources, resource)
		encoders[resource] = resourceEncoder
	}
	layout := s.layout
	s.mu.Unlock()
	sort.Strings(resources)
	for _, resource := range resources {
		resourceEncoder := encoders[resource]
		resourceEncoder.mu.Lock()
		buffer := session.takeEncodedBuffer()
		encoded, _, err := resourceEncoder.encoder.Finalize(buffer)
		if err == nil && len(encoded) > 0 && resourceEncoder.hasRows {
			var key string
			if key, err = layout.object(resource); err == nil {
				err = session.Append(ctx, resource, key, encoded, 0)
			}
		}
		if err != nil {
			session.releaseEncodedBuffer(buffer)
			resourceEncoder.mu.Unlock()
			return fmt.Errorf("gcs sink: finalize %s: %w", resource, err)
		}
		session.releaseEncodedBuffer(encoded)
		resourceEncoder.mu.Unlock()
	}
	return nil
}

type successManifest struct {
	Version   int                `json:"version"`
	Run       string             `json:"run"`
	Resources []manifestResource `json:"resources"`
}

type manifestResource struct {
	Name   string `json:"name"`
	Key    string `json:"key"`
	URI    string `json:"uri"`
	Rows   int64  `json:"rows"`
	Bytes  int64  `json:"bytes"`
	CRC32C string `json:"crc32c"`
}

func (s *Sink) beginCommit() (*uploadSession, string, keyLayout, error) {
	s.mu.Lock()
	if s.state != stateOpen || s.session == nil {
		s.mu.Unlock()
		return nil, "", keyLayout{}, fmt.Errorf("gcs sink: commit requires an open run")
	}
	s.state = stateCommitting
	session, bucket, layout := s.session, s.bucket, s.layout
	s.mu.Unlock()
	s.applyWG.Wait()
	return session, bucket, layout, nil
}

// Commit finalizes all resource objects and publishes _SUCCESS.json last.
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
			Name: result.resource, Key: result.key, URI: fmt.Sprintf("gs://%s/%s", bucket, result.key),
			Rows: result.rows, Bytes: result.bytes, CRC32C: fmt.Sprintf("%08x", result.crc32c),
		}
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("gcs sink: encode success manifest: %w", err)
	}
	if err := session.PutObject(ctx, layout.success(), objectMetadata{contentType: manifestContentType}, body); err != nil {
		return fmt.Errorf("gcs sink: publish success marker: %w", err)
	}
	s.mu.Lock()
	s.state = stateCommitted
	s.mu.Unlock()
	session.Cancel()
	_ = session.closeClient()
	return nil
}

// Abort cancels all resumable writers. Unfinalized objects remain invisible.
func (s *Sink) Abort(_ context.Context) error {
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
	return session.Abort()
}
