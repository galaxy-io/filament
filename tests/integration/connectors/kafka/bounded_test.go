//go:build integration

package kafka_test

import (
	"context"
	"fmt"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	kafkasource "github.com/galaxy-io/filament/connectors/kafka/source"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/rowmodel"
	tc "github.com/galaxy-io/filament/tests/testcontainers"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"reflect"
	"testing"
	"time"
)

type boundedSink struct {
	filament.Sink
	values  []string
	schemas map[string]rowmodel.Schema
}

func (*boundedSink) Spec() filament.SinkSpec { return filament.SinkSpec{} }
func (s *boundedSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	if !arrowbatch.Schema(s.schemas[b.Resource]).Equal(b.Rows().Schema()) {
		return filament.WriteReceipt{}, fmt.Errorf("extraction schema differs from planned schema")
	}
	column := b.Rows().Column(b.Rows().Schema().FieldIndices("value")[0]).(*array.String)
	for i := 0; i < b.NumRows(); i++ {
		if column.IsNull(i) {
			s.values = append(s.values, "<tombstone>")
		} else {
			s.values = append(s.values, column.Value(i))
		}
	}
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}
func extractBounded(ctx context.Context, src *kafkasource.Source, resources []string, limit int) ([]string, error) {
	sink := &boundedSink{schemas: map[string]rowmodel.Schema{}}
	policies := map[string]filament.WritePolicy{}
	for _, topic := range resources {
		schema, err := src.Schema(ctx, topic)
		if err != nil {
			return nil, err
		}
		schema, err = rowmodel.WithAuditFields(schema, false)
		if err != nil {
			return nil, err
		}
		sink.schemas[topic] = schema
		policies[topic] = filament.WritePolicy{Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}
	}
	p := pipeline.New(pipeline.Config{Sink: sink, WritePolicies: policies, Audit: &pipeline.AuditConfig{RunStartedAt: time.Now()}})
	p.Start(ctx)
	err := src.Extract(ctx, p.Records(), filament.ExtractOpts{Resources: resources, Limit: limit})
	p.CloseIngest(err)
	if waitErr := p.Wait(); err == nil {
		err = waitErr
	}
	return sink.values, err
}
func TestKafkaBoundedSource(t *testing.T) {
	for _, profile := range []string{"kafka", "redpanda"} {
		t.Run(profile, func(t *testing.T) {
			brokers := tc.KafkaContainer(t, profile == "redpanda")
			ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
			defer cancel()
			producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RecordPartitioner(kgo.ManualPartitioner()))
			if err != nil {
				t.Fatal(err)
			}
			defer producer.Close()
			publish := func(t *testing.T, topic string, partition int32, value []byte) {
				t.Helper()
				if err := producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Partition: partition, Value: value}).FirstErr(); err != nil {
					t.Fatal(err)
				}
			}
			source := func(t *testing.T, topics []string) *kafkasource.Source {
				t.Helper()
				s := kafkasource.New()
				if err := s.Configure(ctx, filament.NewConfig(map[string]any{"brokers": brokers, "topics": topics, "start_position": "latest"})); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { s.Teardown(context.Background()) })
				return s
			}
			t.Run("fixed-window-schema-limit-and-empty", func(t *testing.T) {
				topic, empty := "bounded_"+uuid.NewString(), "empty_"+uuid.NewString()
				createTopic(t, producer, topic)
				createTopic(t, producer, empty)
				publish(t, topic, 0, []byte(`{"value":"before"}`))
				publish(t, topic, 0, nil)
				publish(t, topic, 1, []byte(`{"value":"partition-1"}`))
				src := source(t, []string{topic, empty})
				resources, err := src.PlanResources(ctx, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				publish(t, topic, 0, []byte(`{"value":"after"}`))
				publish(t, empty, 0, []byte(`{"value":"after-empty"}`))
				got, err := extractBounded(ctx, src, resources, 0)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, []string{"before", "<tombstone>", "partition-1"}) {
					t.Fatalf("bounded records %v", got)
				}
				got, err = extractBounded(ctx, src, resources, 1)
				if err != nil || !reflect.DeepEqual(got, []string{"before"}) {
					t.Fatalf("limit: %v %v", got, err)
				}
				fresh := source(t, []string{topic})
				selected, err := fresh.PlanResources(ctx, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				got, err = extractBounded(ctx, fresh, selected, 0)
				if err != nil || len(got) != 4 {
					t.Fatalf("new run: %v %v", got, err)
				}
				cancelled, stop := context.WithCancel(ctx)
				stop()
				if _, err = extractBounded(cancelled, src, resources, 0); err == nil {
					t.Fatal("ignored cancellation")
				}
			})
			t.Run("transactions", func(t *testing.T) {
				topic := "transactions_" + uuid.NewString()
				createTopic(t, producer, topic)
				tx, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.TransactionalID(uuid.NewString()), kgo.RecordPartitioner(kgo.ManualPartitioner()))
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Close()
				for _, commit := range []bool{true, false} {
					if err = tx.BeginTransaction(); err != nil {
						t.Fatal(err)
					}
					value := []byte(`{"value":"committed"}`)
					if !commit {
						value = []byte(`{"value":"aborted"}`)
					}
					if err = tx.ProduceSync(ctx, &kgo.Record{Topic: topic, Partition: 0, Value: value}).FirstErr(); err != nil {
						t.Fatal(err)
					}
					if err = tx.EndTransaction(ctx, kgo.TransactionEndTry(commit)); err != nil {
						t.Fatal(err)
					}
				}
				if err = tx.BeginTransaction(); err != nil {
					t.Fatal(err)
				}
				if err = tx.ProduceSync(ctx, &kgo.Record{Topic: topic, Partition: 0, Value: []byte(`{"value":"open"}`)}).FirstErr(); err != nil {
					t.Fatal(err)
				}
				src := source(t, []string{topic})
				selected, err := src.PlanResources(ctx, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				if err = tx.EndTransaction(ctx, kgo.TryCommit); err != nil {
					t.Fatal(err)
				}
				got, err := extractBounded(ctx, src, selected, 0)
				if err != nil || !reflect.DeepEqual(got, []string{"committed"}) {
					t.Fatalf("transaction window: %v %v", got, err)
				}
			})
			t.Run("retention-loss", func(t *testing.T) {
				topic := "retention_" + uuid.NewString()
				createTopic(t, producer, topic)
				publish(t, topic, 0, []byte(`{"value":"lost"}`))
				publish(t, topic, 0, []byte(`{"value":"retained"}`))
				src := source(t, []string{topic})
				selected, err := src.PlanResources(ctx, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
				trim := kmsg.NewPtrDeleteRecordsRequest()
				trim.Topics = []kmsg.DeleteRecordsRequestTopic{{Topic: topic, Partitions: []kmsg.DeleteRecordsRequestTopicPartition{{Partition: 0, Offset: 1}}}}
				response, err := trim.RequestWith(ctx, producer)
				if err != nil {
					t.Fatal(err)
				}
				for _, topic := range response.Topics {
					for _, p := range topic.Partitions {
						if p.ErrorCode != 0 {
							t.Fatalf("delete records: %+v", p)
						}
					}
				}
				if _, err = extractBounded(ctx, src, selected, 0); err == nil {
					t.Fatal("retention loss silently skipped")
				}
			})
		})
	}
}
