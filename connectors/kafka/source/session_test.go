package source

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/streamkit"
)

type fakeConsumer struct {
	records []*kgo.Record
	polls   int
}

func (f *fakeConsumer) PollRecords(context.Context, int) kgo.Fetches {
	f.polls++
	if len(f.records) == 0 {
		return nil
	}
	r := f.records[0]
	f.records = f.records[1:]
	return kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: r.Topic, Partitions: []kgo.FetchPartition{{Partition: r.Partition, Records: []*kgo.Record{r}}}}}}}
}
func (*fakeConsumer) Close() {}

type collectingSink struct {
	filament.Sink
	rows    int
	inspect func(*arrowbatch.Batch)
}

func (*collectingSink) Spec() filament.SinkSpec { return filament.SinkSpec{} }
func (s *collectingSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.rows += b.NumRows()
	if s.inspect != nil {
		s.inspect(b)
	}
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func testSession(t *testing.T, initial bool) (*session, *fakeConsumer, filament.StreamRecordSink, *collectingSink) {
	t.Helper()
	d := filament.DomainKey{Domain: "topic/0", Incarnation: "cluster/topic-id"}
	f := &fakeConsumer{records: []*kgo.Record{{Topic: "events", Partition: 0, Offset: 4, Key: []byte{}, Value: []byte("message"), Timestamp: time.Now()}}}
	s := &session{consumer: f, partitions: map[partitionKey]*partitionState{{"events", 0}: {domain: d, next: 0, initial: initial}}, writers: map[string]arrowbatch.RowWriter{}, projectors: nil, codecs: New()}
	s.projectors = map[string]*streamkit.Projector{}
	var err error
	s.lifecycle, err = stream.NewSourceLifecycle(filament.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}, func(context.Context) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	sink := &collectingSink{}
	p, err := pipeline.NewStream(pipeline.Config{Sink: sink, WritePolicies: map[string]filament.WritePolicy{"events": {Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}}}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	t.Cleanup(func() {
		p.CloseIngest(nil)
		if err := p.Wait(); err != nil {
			t.Error(err)
		}
	})
	return s, f, p.Records().(filament.StreamRecordSink), sink
}

func TestDecodedJSONAndTombstone(t *testing.T) {
	s, f, out, sink := testSession(t, false)
	f.records = []*kgo.Record{
		{Topic: "events", Partition: 0, Offset: 0, Value: []byte(`{"customer_id":"customer-1","amount_cents":9007199254740993}`)},
		{Topic: "events", Partition: 0, Offset: 1, Value: nil},
		{Topic: "events", Partition: 0, Offset: 2, Value: []byte(`{"customer_id":"customer-1","unexpected":true}`)},
	}
	sink.inspect = func(b *arrowbatch.Batch) {
		schema := b.Rows().Schema()
		if len(schema.FieldIndices("_filament_payload")) != 0 {
			t.Error("decoded source exposed raw envelope payload")
		}
		for _, name := range []string{"customer_id", "amount_cents", "tombstone"} {
			if len(schema.FieldIndices(name)) != 1 {
				t.Errorf("missing decoded column %s", name)
				return
			}
		}
		customer := b.Rows().Column(schema.FieldIndices("customer_id")[0])
		amount := b.Rows().Column(schema.FieldIndices("amount_cents")[0])
		tombstone := b.Rows().Column(schema.FieldIndices("tombstone")[0]).(*array.Boolean)
		if sink.rows == 1 {
			if customer.(*array.String).Value(0) != "customer-1" || amount.(*array.String).Value(0) != "9007199254740993" || tombstone.Value(0) {
				t.Error("JSON values were not preserved")
			}
		} else if !customer.IsNull(0) || !amount.IsNull(0) || !tombstone.Value(0) {
			t.Error("tombstone did not retain schema with null payload columns")
		}
	}
	for range 2 {
		coverage, err := s.Read(t.Context(), out, filament.Boundary{MaxWait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Acknowledge(t.Context(), coverage); err != nil {
			t.Fatal(err)
		}
	}
	coverage, err := s.Read(t.Context(), out, filament.Boundary{MaxWait: time.Second})
	if err == nil || len(coverage.Positions) != 0 || sink.rows != 2 || s.partitions[partitionKey{"events", 0}].next != 2 {
		t.Fatalf("schema drift advanced progress: rows=%d coverage=%+v err=%v", sink.rows, coverage, err)
	}
	if err := s.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestInitialFrontierAndCertifiedProgress(t *testing.T) {
	s, f, out, sink := testSession(t, true)
	b := filament.Boundary{MaxWait: time.Second, MaxRecords: 1}
	c, err := s.Read(t.Context(), out, b)
	if err != nil || len(c.Positions) != 1 || f.polls != 0 {
		t.Fatalf("initial: %+v %v", c, err)
	}
	if _, err = s.Read(t.Context(), out, b); !errors.Is(err, stream.ErrAcknowledgementPending) {
		t.Fatal(err)
	}
	if err = s.Acknowledge(t.Context(), filament.Coverage{}); err == nil {
		t.Fatal("accepted wrong coverage")
	}
	if err = s.Acknowledge(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	c, err = s.Read(t.Context(), out, b)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range c.Positions {
		n, _ := offset(p)
		if n != 5 {
			t.Fatalf("next offset %d", n)
		}
	}
	if sink.rows != 1 {
		t.Fatalf("rows %d", sink.rows)
	}
	if s.partitions[partitionKey{"events", 0}].next != 0 {
		t.Fatal("advanced before certification")
	}
	if err = s.Acknowledge(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	if s.partitions[partitionKey{"events", 0}].next != 5 {
		t.Fatal("certified offset not applied")
	}
	if err = s.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(t.Context(), out, b); err == nil {
		t.Fatal("read after close")
	}
}

type badControl struct{ filament.StreamRecordSink }

func (badControl) Control(context.Context, filament.Control) error {
	return errors.New("control failed")
}

func TestFailedControlCannotBeAcknowledged(t *testing.T) {
	s, _, out, _ := testSession(t, false)
	_, err := s.Read(t.Context(), badControl{out}, filament.Boundary{MaxWait: time.Second})
	if err == nil {
		t.Fatal("control failure ignored")
	}
	if _, err = s.Read(t.Context(), out, filament.Boundary{MaxWait: time.Second}); err == nil {
		t.Fatal("failed read reused")
	}
	if err = s.Acknowledge(t.Context(), filament.Coverage{}); err == nil {
		t.Fatal("failed read acknowledged")
	}
}

func TestOffsetCodec(t *testing.T) {
	c := OffsetCodec{}
	for _, value := range []string{"", "-1", "+1", "1.1", "9223372036854775808"} {
		p := position(0)
		p.Value = []byte(value)
		if err := c.Validate(p); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	p := position(0)
	p.Value = []byte("0002")
	canon, err := c.Canonicalize(p)
	if err != nil || string(canon.Value) != "2" {
		t.Fatalf("%+v %v", canon, err)
	}
	cmp, err := c.Compare(position(0), position(1))
	if err != nil || cmp != filament.PositionBefore {
		t.Fatal(cmp, err)
	}
}

func TestDomainsRequireIncarnation(t *testing.T) {
	cluster, topic := "cluster", "events"
	m := &kmsg.MetadataResponse{ClusterID: &cluster, Topics: []kmsg.MetadataResponseTopic{{Topic: &topic, TopicID: [16]byte{1}, Partitions: []kmsg.MetadataResponseTopicPartition{{Partition: 0}}}}}
	first, err := domains(m, "source")
	if err != nil {
		t.Fatal(err)
	}
	m.Topics[0].TopicID = [16]byte{2}
	second, err := domains(m, "source")
	if err != nil {
		t.Fatal(err)
	}
	k := partitionKey{topic, 0}
	if first[k].Domain != second[k].Domain || first[k].Incarnation == second[k].Incarnation {
		t.Fatal("topic recreation reused identity")
	}
	m.Topics[0].TopicID = [16]byte{}
	if _, err = domains(m, "source"); err == nil {
		t.Fatal("missing topic ID accepted")
	}
}

func TestPlanningStableAcrossEndpoints(t *testing.T) {
	s := New()
	req := filament.ReplicationStreamPlanningRequest{SourceConnectionID: "source", ReplicationStreamID: "stream", Resources: []string{"b", "a"}, Config: filament.NewConfig(map[string]any{"brokers": []string{"one:9092"}})}
	first, err := s.PlanReplicationStream(req)
	if err != nil {
		t.Fatal(err)
	}
	req.Config = filament.NewConfig(map[string]any{"brokers": []string{"two:9092"}})
	second, err := s.PlanReplicationStream(req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ConsumerName != second.ConsumerName || first.Resources[0] != "a" {
		t.Fatal("unstable planning")
	}
}

func TestAuthorityLossPreventsAcknowledgment(t *testing.T) {
	s, _, out, _ := testSession(t, true)
	c, err := s.Read(t.Context(), out, filament.Boundary{MaxWait: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	denied := errors.New("ownership lost")
	// Replace only the authority callback by constructing a lifecycle with the
	// same pending coverage, simulating a coordinator lease changing after Read.
	s.lifecycle, err = stream.NewSourceLifecycle(filament.AttemptRef{RunID: "r", ExecutionID: "e", StreamID: "s", Generation: 1, Token: 1}, func(context.Context) error { return denied })
	if err != nil {
		t.Fatal(err)
	}
	if err = s.lifecycle.MarkRead(c); err != nil {
		t.Fatal(err)
	}
	if err = s.Acknowledge(t.Context(), c); !errors.Is(err, denied) {
		t.Fatal(err)
	}
	if !s.partitions[partitionKey{"events", 0}].initial {
		t.Fatal("acknowledged without authority")
	}
}

func TestLeadingTombstonesWaitForSchema(t *testing.T) {
	s, f, out, sink := testSession(t, false)
	f.records = []*kgo.Record{
		{Topic: "events", Offset: 0},
		{Topic: "events", Offset: 1},
	}
	boundary := filament.Boundary{MaxWait: time.Second, MaxRecords: 1}
	for range 3 { // Includes an idle poll while the payload has not arrived.
		coverage, err := s.Read(t.Context(), out, boundary)
		if err != nil || len(coverage.Positions) != 0 || sink.rows != 0 || s.writers["events"] != nil || s.columns["events"] != nil {
			t.Fatalf("tombstone initialized schema or advanced progress: %+v %v", coverage, err)
		}
	}
	f.records = []*kgo.Record{
		{Topic: "events", Offset: 2, Value: []byte(`{"value":"first"}`)},
		{Topic: "events", Offset: 3},
	}
	sink.inspect = func(b *arrowbatch.Batch) {
		schema := b.Rows().Schema()
		indices := schema.FieldIndices("value")
		if len(indices) != 1 {
			t.Error("leading tombstone selected raw schema")
			return
		}
		values := b.Rows().Column(indices[0]).(*array.String)
		offsets := b.Rows().Column(schema.FieldIndices("offset")[0]).(*array.Int64)
		tombstones := b.Rows().Column(schema.FieldIndices("tombstone")[0]).(*array.Boolean)
		for i := 0; i < b.NumRows(); i++ {
			wantOffset := int64(sink.rows - b.NumRows() + i)
			if offsets.Value(i) != wantOffset || tombstones.Value(i) != (wantOffset != 2) || values.IsNull(i) != (wantOffset != 2) {
				t.Errorf("wrong row at offset %d", wantOffset)
			}
			if wantOffset == 2 && values.Value(i) != "first" {
				t.Error("lost JSON payload")
			}
		}
	}
	for next := int64(1); next <= 4; next++ {
		coverage, err := s.Read(t.Context(), out, boundary)
		if err != nil || len(coverage.Positions) != 1 {
			t.Fatalf("read: %+v %v", coverage, err)
		}
		p := s.partitions[partitionKey{"events", 0}]
		got, err := offset(coverage.Positions[p.domain])
		if err != nil || got != next || p.next != next-1 || sink.rows != int(next) {
			t.Fatalf("out of order progress: offset=%d committed=%d rows=%d err=%v", got, p.next, sink.rows, err)
		}
		if err := s.Acknowledge(t.Context(), coverage); err != nil {
			t.Fatal(err)
		}
	}
	if s.pendingCount != 0 || len(s.pending) != 0 || len(s.ready) != 0 {
		t.Fatal("retained emitted records")
	}
}

func TestTombstoneLookaheadLimitDoesNotAdvanceProgress(t *testing.T) {
	s, f, out, sink := testSession(t, false)
	s.pendingCount = maxPendingTombstones
	f.records[0].Value = nil
	coverage, err := s.Read(t.Context(), out, filament.Boundary{MaxWait: time.Second})
	if err == nil || len(coverage.Positions) != 0 || sink.rows != 0 || s.partitions[partitionKey{"events", 0}].next != 0 {
		t.Fatalf("lookahead limit advanced progress: %+v %v", coverage, err)
	}
	if err := s.Acknowledge(t.Context(), coverage); err == nil {
		t.Fatal("acknowledged failed lookahead")
	}
}
