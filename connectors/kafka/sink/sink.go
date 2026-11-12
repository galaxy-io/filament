// Package sink publishes acknowledged, append-only Kafka records.
package sink

import (
	"context"
	"errors"
	"fmt"
	"hash/crc32"
	"sync"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/kafka/internal/client"
	"github.com/galaxy-io/filament/internal/sinkstate"
)

type producer interface {
	Produce(context.Context, *kgo.Record, func(*kgo.Record, error))
	Close()
}

// Sink serializes Apply calls; callbacks own encoded data, never borrowed Arrow buffers.
type Sink struct {
	mu         sync.Mutex
	producer   producer
	cfg        config
	checkTopic func(context.Context, string) (map[int32]bool, error)
	uncertain  error
	lifecycle  sinkstate.Lifecycle
	closeOnce  sync.Once
	closeDone  chan struct{}
}

// New returns a fresh connector instance.
func New() *Sink { return &Sink{} }

// Name identifies the registered sink.
func (*Sink) Name() string { return "kafka" }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
)

// TestConnection checks authentication and metadata access without producing records.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	return client.TestConnection(ctx, cfg)
}

// Open creates a producer for one bounded run or continuous session.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lifecycle.Opened() {
		return errors.New("kafka sink: already opened")
	}
	cfg := filament.NewConfig(run.Sink.Config)
	opts, err := client.Options(cfg)
	if err != nil {
		return err
	}
	c, err := resolve(cfg)
	if err != nil {
		return err
	}
	for _, p := range run.WritePolicies {
		if p.Capability.Mode != filament.WriteAppend {
			return errors.New("kafka sink: only append supported")
		}
	}
	opts = append(opts, c.producerOptions()...)
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return err
	}
	ready := false
	defer func() {
		if !ready {
			cl.Close()
		}
	}()
	check := func(ctx context.Context, topic string) (map[int32]bool, error) {
		m, err := client.Metadata(ctx, cl, []string{topic})
		if err != nil {
			return nil, err
		}
		out := map[int32]bool{}
		for _, t := range m.Topics {
			for _, p := range t.Partitions {
				out[p.Partition] = true
			}
		}
		if len(out) == 0 {
			return nil, errors.New("kafka sink: topic has no partitions")
		}
		return out, nil
	}
	checkCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	if c.topic != "" {
		if _, err = check(checkCtx, c.topic); err != nil {
			return err
		}
	} else {
		for _, r := range run.Resources {
			topic, err := c.route(r)
			if err != nil {
				return err
			}
			if _, err = check(checkCtx, topic); err != nil {
				return err
			}
		}
	}
	if err := checkCtx.Err(); err != nil {
		return err
	}
	s.producer, s.checkTopic, s.cfg = cl, check, c
	s.lifecycle.Open()
	ready = true
	return nil
}

func (s *Sink) fail(err error) error {
	s.lifecycle.Fail(err)
	return err
}

// Apply publishes a borrowed batch and waits for every record acknowledgment.
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
		return empty, s.fail(errors.New("kafka sink: only append supported"))
	}
	if b == nil || b.Rows() == nil {
		return empty, s.fail(errors.New("kafka sink: batch required"))
	}
	if b.Operations().Len() != 0 && b.Operations().Len() != b.NumRows() {
		return empty, s.fail(errors.New("kafka sink: operation count differs from row count"))
	}
	if err := opts.Policy.ValidateBatch(b.Resource, b); err != nil {
		return empty, s.fail(err)
	}
	records, acc, err := s.encode(b)
	if err != nil {
		return empty, s.fail(err)
	}
	publishCtx, cancel := context.WithTimeout(ctx, s.cfg.timeout)
	defer cancel()
	if len(records) > 0 {
		partitions, err := s.checkTopic(publishCtx, records[0].Topic)
		if err != nil {
			return empty, s.fail(err)
		}
		if s.cfg.partitionField != "" {
			for _, r := range records {
				if !partitions[r.Partition] {
					return empty, s.fail(fmt.Errorf("kafka sink: unknown partition %d", r.Partition))
				}
			}
		}
	}
	var crc uint32
	for _, r := range records {
		crc = crc32.Update(crc, crcTable, r.Value)
	}
	if crc != acc.crc {
		return empty, s.fail(errors.New("kafka sink: encoded checksum mismatch"))
	}
	if err := s.publish(publishCtx, records); err != nil {
		return empty, s.fail(err)
	}
	topic, _ := s.cfg.route(b.Resource)
	receipt := filament.WriteReceipt{URI: "kafka://" + topic, Rows: b.NumRows(), Bytes: acc.bytes, WriteCRC: b.IntegrityCRC(), EncodedCRC: &crc}

	return receipt, nil
}

// Shutdown is bounded by caller context. A timed-out close remains unsuccessful.
func (s *Sink) cleanupPublisher(ctx context.Context) error {
	s.closeOnce.Do(func() {
		s.closeDone = make(chan struct{})
		go func() {
			defer close(s.closeDone)
			if s.producer != nil {
				s.producer.Close()
			}
		}()
	})
	select {
	case <-s.closeDone:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}
