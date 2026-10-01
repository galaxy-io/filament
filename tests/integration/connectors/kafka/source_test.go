//go:build integration

package kafka_test

import (
	"context"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	kafkasource "github.com/galaxy-io/filament/connectors/kafka/source"
	"github.com/galaxy-io/filament/pipeline"
	tc "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestKafkaSource(t *testing.T) {
	brokers := tc.KafkaContainer(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...), kgo.RecordPartitioner(kgo.ManualPartitioner()))
	if err != nil {
		t.Fatal(err)
	}
	defer producer.Close()
	topic := "source_" + uuid.NewString()
	createTopic(t, producer, topic)
	for _, value := range [][]byte{[]byte(`{"value":"first"}`), []byte(`{"value":"second"}`), nil} {
		if err := producer.ProduceSync(ctx, &kgo.Record{Topic: topic, Partition: 1, Key: []byte("key"), Value: value}).FirstErr(); err != nil {
			t.Fatal(err)
		}
	}
	attempt := filament.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: uuid.NewString(), Generation: 1, Token: 1}
	saved := testSource(t, brokers, topic, attempt)
	testSourceGuardrails(t, producer, brokers, topic, attempt, saved)
}

func createTopic(t *testing.T, c *kgo.Client, topic string) {
	t.Helper()
	req := kmsg.NewPtrCreateTopicsRequest()
	req.Topics = []kmsg.CreateTopicsRequestTopic{{Topic: topic, NumPartitions: 2, ReplicationFactor: 1}}
	res, err := req.RequestWith(t.Context(), c)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res.Topics {
		if r.ErrorCode != 0 {
			t.Fatalf("create topic: %+v", r)
		}
	}
}

type collectSink struct {
	filament.Sink
	rows        int
	payloadNull []bool
}

func (*collectSink) Spec() filament.SinkSpec { return filament.SinkSpec{} }
func (s *collectSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.rows += b.NumRows()
	idx := b.Rows().Schema().FieldIndices("tombstone")[0]
	for i := range b.NumRows() {
		s.payloadNull = append(s.payloadNull, b.Rows().Column(idx).(*array.Boolean).Value(i))
	}
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func testSource(t *testing.T, brokers []string, topic string, attempt filament.AttemptRef) filament.DomainPositions {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	src := kafkasource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"brokers": brokers})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(context.Background())
	opts := filament.StreamOpenOpts{SourceConnectionID: "connection", Resources: []string{topic}, Attempt: attempt, CheckAuthority: func(context.Context) error { return nil }}
	session, err := src.OpenStream(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close(context.Background())
	sink := &collectSink{}
	p, err := pipeline.NewStream(pipeline.Config{Sink: sink, WritePolicies: map[string]filament.WritePolicy{topic: {Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}}}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(ctx)
	defer func() {
		p.CloseIngest(nil)
		if err := p.Wait(); err != nil {
			t.Error(err)
		}
	}()
	out := p.Records().(filament.StreamRecordSink)
	boundary := filament.Boundary{MaxWait: time.Second, MaxRecords: 1}
	committed := filament.DomainPositions{}
	for sink.rows < 3 {
		coverage, err := session.Read(ctx, out, boundary)
		if err != nil {
			t.Fatal(err)
		}
		if len(coverage.Positions) == 0 {
			continue
		}
		for d, pos := range coverage.Positions {
			committed[d] = pos
		}
		if err = session.Acknowledge(ctx, coverage); err != nil {
			t.Fatal(err)
		}
	}
	if len(committed) != 2 || len(sink.payloadNull) != 3 || sink.payloadNull[0] || !sink.payloadNull[2] {
		t.Fatalf("source envelope/checkpoints: %+v %+v", committed, sink.payloadNull)
	}
	if err = session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	opts.CommittedPositions = committed
	resumed, err := src.OpenStream(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close(context.Background())
	coverage, err := resumed.Read(ctx, out, filament.Boundary{MaxWait: 100 * time.Millisecond})
	if err != nil || len(coverage.Positions) != 0 {
		t.Fatalf("resume replayed acknowledged data: %+v %v", coverage, err)
	}
	return committed
}

func testSourceGuardrails(t *testing.T, c *kgo.Client, brokers []string, topic string, attempt filament.AttemptRef, saved filament.DomainPositions) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	src := kafkasource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"brokers": brokers})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(context.Background())
	opts := filament.StreamOpenOpts{SourceConnectionID: "connection", Resources: []string{topic}, Attempt: attempt, CommittedPositions: saved, CheckAuthority: func(context.Context) error { return nil }}
	live, err := src.OpenStream(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close(context.Background())
	grow := kmsg.NewPtrCreatePartitionsRequest()
	grow.Topics = []kmsg.CreatePartitionsRequestTopic{{Topic: topic, Count: 3}}
	grown, err := grow.RequestWith(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range grown.Topics {
		if r.ErrorCode != 0 {
			t.Fatalf("grow: %+v", r)
		}
	}
	// Metadata validation occurs before any attempt to write a record.
	if _, err = live.Read(ctx, nil, filament.Boundary{MaxWait: time.Second}); err == nil {
		t.Fatal("live membership change accepted")
	}
	if err = live.Close(ctx); err != nil {
		t.Fatal(err)
	}
	trim := kmsg.NewPtrDeleteRecordsRequest()
	trim.Topics = []kmsg.DeleteRecordsRequestTopic{{Topic: topic, Partitions: []kmsg.DeleteRecordsRequestTopicPartition{{Partition: 1, Offset: 2}}}}
	trimmed, err := trim.RequestWith(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range trimmed.Topics {
		for _, p := range r.Partitions {
			if p.ErrorCode != 0 {
				t.Fatalf("trim: %+v", p)
			}
		}
	}
	stale := saved.Clone()
	for d, p := range stale {
		p.Value = []byte("0")
		stale[d] = p
	}
	opts.CommittedPositions = stale
	if session, err := src.OpenStream(ctx, opts); err == nil {
		_ = session.Close(ctx)
		t.Fatal("retention loss silently reset")
	}
	del := kmsg.NewPtrDeleteTopicsRequest()
	del.TopicNames = []string{topic}
	del.Topics = []kmsg.DeleteTopicsRequestTopic{{Topic: &topic}}
	deleted, err := del.RequestWith(ctx, c)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range deleted.Topics {
		if r.ErrorCode != 0 {
			t.Fatalf("delete: %+v", r)
		}
	}
	createTopic(t, c, topic)
	opts.CommittedPositions = saved
	if session, err := src.OpenStream(ctx, opts); err == nil {
		_ = session.Close(ctx)
		t.Fatal("recreated topic reused old checkpoint")
	}
}
