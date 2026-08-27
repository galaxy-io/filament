// Package s3 implements a run-versioned NDJSON sink for Amazon S3 and
// compatible object stores.
//
// Apply streams full multipart parts and Commit publishes _SUCCESS.json last.
// Consumers must ignore run prefixes without that marker.
package s3

import (
	"context"
	"fmt"
	"sync"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/internal/ndjson"
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
	mu      sync.Mutex
	state   sinkState
	bucket  string
	prefix  string
	run     filament.RunID
	session *multipartSession
	applyWG sync.WaitGroup
	enc     map[*arrow.Schema]*ndjson.Encoder
}

// New returns an unconfigured sink.
func New() *Sink { return &Sink{state: stateNew, enc: make(map[*arrow.Schema]*ndjson.Encoder)} }

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

	declared := make(map[string]string, len(run.Resources))
	for _, resource := range run.Resources {
		if resource != "" {
			declared[resource] = objectKey(cfg.prefix, run.Run, resource)
		}
	}
	s.state = stateOpen
	s.bucket = cfg.bucket
	s.prefix = cfg.prefix
	s.run = run.Run
	s.session = newMultipartSession(ctx, store, cfg.bucket, cfg.partSize, cfg.uploadWorkers, declared)
	s.applyWG = sync.WaitGroup{}
	s.enc = make(map[*arrow.Schema]*ndjson.Encoder)
	return nil
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
	scratch := session.takeEncodedBuffer()
	encoded, encodedCRC, err := s.encoderFor(rows.Schema()).EncodeBatch(scratch, rows)
	if err != nil {
		session.releaseEncodedBuffer(scratch)
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: encode %s: %w", batch.Resource, err)
	}
	defer session.releaseEncodedBuffer(encoded)
	if err := session.Append(ctx, batch.Resource, key, encoded, batch.NumRows(), encodedCRC); err != nil {
		return filament.WriteReceipt{}, fmt.Errorf("s3 sink: stream %s: %w", batch.Resource, err)
	}
	return filament.WriteReceipt{
		URI:        fmt.Sprintf("s3://%s/%s", bucket, key),
		Bytes:      int64(len(encoded)),
		Rows:       batch.NumRows(),
		WriteCRC:   batch.IntegrityCRC(),
		EncodedCRC: &encodedCRC,
	}, nil
}

func (s *Sink) encoderFor(schema *arrow.Schema) *ndjson.Encoder {
	s.mu.Lock()
	defer s.mu.Unlock()
	if enc := s.enc[schema]; enc != nil {
		return enc
	}
	enc := ndjson.NewEncoder(schema)
	s.enc[schema] = enc
	return enc
}

func (s *Sink) beginApply(resource string) (*multipartSession, string, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state != stateOpen || s.session == nil {
		return nil, "", "", fmt.Errorf("s3 sink: write requires an open run")
	}
	s.applyWG.Add(1)
	return s.session, s.bucket, objectKey(s.prefix, s.run, resource), nil
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

func objectKey(prefix string, run filament.RunID, resource string) string {
	return runKey(prefix, run) + "/" + resource + ".ndjson"
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
