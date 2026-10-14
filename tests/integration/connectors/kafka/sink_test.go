//go:build integration

package kafka_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	kafkasink "github.com/galaxy-io/filament/connectors/kafka/sink"
	"github.com/galaxy-io/filament/rowmodel"
	tc "github.com/galaxy-io/filament/tests/testcontainers"
)

func input(t *testing.T) *arrowbatch.Batch {
	t.Helper()
	schema := rowmodel.Schema{Fields: []rowmodel.Field{{Name: "key", Logical: rowmodel.LogicalString}, {Name: "target_partition", Logical: rowmodel.LogicalInt32}}}
	rb := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
	defer rb.Release()
	for range 3 {
		rb.Field(0).(*array.StringBuilder).Append("key")
		rb.Field(1).(*array.Int32Builder).Append(1)
	}
	b := arrowbatch.NewBatch(rb.NewRecordBatch(), arrowbatch.NewOperations([]filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}))
	b.Resource = "events"
	t.Cleanup(b.Release)
	return b
}

func TestKafkaConnector(t *testing.T) {
	brokers := tc.KafkaContainer(t)
	c, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, continuous := range []bool{false, true} {
		t.Run(fmt.Sprintf("continuous=%v", continuous), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			topic := "filament_" + uuid.NewString()
			createTopic(t, c, topic)
			cfg := map[string]any{"brokers": brokers, "topic": topic, "key_field": "key", "partition_field": "target_partition", "delete_mode": "tombstone"}
			s := kafkasink.New()
			run := filament.RunSpec{Sink: filament.Ref{Config: cfg}}
			ref := filament.EpochRef{Attempt: filament.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: uuid.NewString(), Generation: 1, Token: 1}, Epoch: 1, MembershipRevision: 1}
			if continuous {
				run.Options.Execution = filament.ExecutionContinuous
				run.StreamAttempt = &ref.Attempt
			}
			if err := s.Open(ctx, run); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if continuous {
					_ = s.CloseSession(context.Background())
				} else {
					_ = s.Abort(context.Background())
				}
			}()
			opts := filament.ApplyOptions{Policy: filament.WritePolicy{Capability: s.Spec().Capabilities.Stream.WritePolicies[0]}}
			if continuous {
				if err := s.BeginEpoch(ctx, ref); err != nil {
					t.Fatal(err)
				}
				opts.Epoch = &ref
			}
			receipt, err := s.Apply(ctx, input(t), opts)
			if err != nil || receipt.Rows != 3 || receipt.EncodedCRC == nil {
				t.Fatalf("Apply: %+v %v", receipt, err)
			}
			if continuous {
				rs, err := s.CommitEpoch(ctx, ref)
				if err != nil || len(rs) != 1 || rs[0].Rows != 3 {
					t.Fatalf("epoch %+v %v", rs, err)
				}
				if err = s.CloseSession(ctx); err != nil {
					t.Fatal(err)
				}
			} else if err = s.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			reader, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{topic: {1: kgo.NewOffset().AtStart()}}))
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			var records []*kgo.Record
			for len(records) < 3 {
				f := reader.PollRecords(ctx, 3-len(records))
				if err := f.Err(); err != nil {
					t.Fatal(err)
				}
				records = append(records, f.Records()...)
			}
			for _, r := range records {
				if r.Partition != 1 || string(r.Key) != "key" {
					t.Fatalf("routing %+v", r)
				}
			}
			if records[0].Value == nil || records[2].Value != nil {
				t.Fatal("delete tombstone mapping")
			}
			saved := testSource(t, brokers, topic, ref.Attempt)
			if continuous {
				testSourceGuardrails(t, c, brokers, topic, ref.Attempt, saved)
			}
		})
	}
	t.Run("small-producer-buffer", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		topic := "filament_" + uuid.NewString()
		createTopic(t, c, topic)
		s := kafkasink.New()
		cfg := map[string]any{"brokers": brokers, "topic": topic, "max_in_flight": 1}
		if err := s.Open(ctx, filament.RunSpec{Sink: filament.Ref{Config: cfg}}); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = s.Abort(ctx) }()
		b := input(t)
		opts := filament.ApplyOptions{Policy: filament.WritePolicy{Capability: s.Spec().Capabilities.Stream.WritePolicies[0]}}
		// Refill a single client slot repeatedly, including across Apply calls.
		// Callback delivery can precede the client's release of that slot.
		const batches = 1000
		for i := range batches {
			receipt, err := s.Apply(ctx, b, opts)
			if err != nil || receipt.Rows != 3 {
				t.Fatalf("batch %d: receipt %+v, error %v", i, receipt, err)
			}
		}
		if err := s.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		reader, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.ConsumeTopics(topic), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		count := 0
		for count < batches*3 {
			fetches := reader.PollRecords(ctx, batches*3-count)
			if err := fetches.Err(); err != nil {
				t.Fatal(err)
			}
			count += fetches.NumRecords()
		}
	})
	t.Run("missing-topic", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		s := kafkasink.New()
		err := s.Open(ctx, filament.RunSpec{Sink: filament.Ref{Config: map[string]any{"brokers": brokers, "topic": "missing_" + uuid.NewString()}}})
		if err == nil {
			_ = s.Abort(ctx)
			t.Fatal("missing topic accepted")
		}
	})
}
