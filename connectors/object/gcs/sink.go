// Package gcs implements a resource-partitioned object sink for Google Cloud
// Storage. Resource objects land at
// <prefix>/<resource>/<partition>/<run>.<ext>, and Commit publishes
// <prefix>/_runs/<run>/_SUCCESS.json last.
package gcs

import (
	"context"
	"fmt"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/internal/encoder"
	object "github.com/galaxy-io/filament/connectors/object/internal"
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
	layout      object.Layout
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
	store, err := newGCPStore(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()
	if err := store.BucketAttrs(ctx, parsed.bucket); err != nil {
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
	store, err := newGCPStore(ctx)
	if err != nil {
		return err
	}
	if err := s.open(ctx, run, cfg, store); err != nil {
		_ = store.Close()
		return err
	}
	return nil
}

func (s *Sink) open(ctx context.Context, run filament.RunSpec, cfg sinkConfig, store resumableStore) error {
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
	options := encoder.Options{FileFormat: cfg.fileFormat, Compression: cfg.compression}
	s.layout = object.NewLayout(cfg.prefix, cfg.partition, options.Extension(), run)
	s.format = cfg.fileFormat
	s.compression = cfg.compression
	s.session = newUploadSession(ctx, store, cfg)
	s.applyWG = sync.WaitGroup{}
	s.enc = make(map[string]*resourceEncoder)
	return nil
}
