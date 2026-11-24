// Package sink publishes acknowledged, append-only Kinesis records.
package sink

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/sinkstate"
)

// Sink serializes Apply calls and sends one request at a time.
type Sink struct {
	mu        sync.Mutex
	client    client
	cfg       sinkConfig
	uncertain error
	lifecycle sinkstate.Lifecycle
}

// New returns a fresh connector instance.
func New() *Sink { return &Sink{} }

// Name identifies the registered sink.
func (*Sink) Name() string { return "kinesis" }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
)

// TestConnection checks credentials and API access without writing records.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	c, err := connection(cfg)
	if err != nil {
		return err
	}
	cl, err := newClient(ctx, c)
	if err != nil {
		return err
	}
	if _, err := cl.ListStreams(ctx, &kinesis.ListStreamsInput{Limit: aws.Int32(1)}); err != nil {
		return fmt.Errorf("kinesis sink: list streams: %w", err)
	}
	return nil
}

// Open creates a client and checks that every destination stream accepts records.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lifecycle.Opened() {
		return errors.New("kinesis sink: already opened")
	}
	c, err := resolve(filament.NewConfig(run.Sink.Config))
	if err != nil {
		return err
	}
	for _, p := range run.WritePolicies {
		if p.Capability.Mode != filament.WriteAppend {
			return errors.New("kinesis sink: only append supported")
		}
	}
	streams := []string{c.stream}
	if c.stream == "" {
		streams = streams[:0]
		for _, r := range run.Resources {
			stream, err := c.route(r)
			if err != nil {
				return err
			}
			streams = append(streams, stream)
		}
	}
	cl, err := newClient(ctx, c)
	if err != nil {
		return err
	}
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	for _, stream := range streams {
		if err := writable(checkCtx, cl, stream); err != nil {
			return err
		}
	}
	s.client, s.cfg = cl, c
	s.lifecycle.Open()
	return nil
}

func (s *Sink) fail(err error) error {
	s.lifecycle.Fail(err)
	return err
}

// Apply publishes a borrowed batch and returns once Kinesis accepted every record.
func (s *Sink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var empty filament.WriteReceipt
	if err := s.lifecycle.CheckApply(); err != nil {
		return empty, err
	}
	if err := ctx.Err(); err != nil {
		return empty, s.fail(err)
	}
	if opts.Policy.Capability.Mode != filament.WriteAppend {
		return empty, s.fail(errors.New("kinesis sink: only append supported"))
	}
	if b == nil || b.Rows() == nil {
		return empty, s.fail(errors.New("kinesis sink: batch required"))
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		return empty, s.fail(err)
	}
	entries, acc, err := s.encode(b)
	if err != nil {
		return empty, s.fail(err)
	}
	var crc uint32
	for _, e := range entries {
		crc = crc32.Update(crc, crcTable, e.Data)
	}
	if crc != acc.crc {
		return empty, s.fail(errors.New("kinesis sink: encoded checksum mismatch"))
	}
	publishCtx, cancel := context.WithTimeout(ctx, s.cfg.timeout)
	defer cancel()
	if err := s.publish(publishCtx, acc.stream, entries); err != nil {
		return empty, s.fail(err)
	}
	return filament.WriteReceipt{URI: "kinesis://" + acc.stream, Rows: b.NumRows(), Bytes: acc.bytes, WriteCRC: b.IntegrityCRC(), EncodedCRC: &crc}, nil
}
