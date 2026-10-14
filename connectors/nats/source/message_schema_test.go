package source

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/schema"
)

type messageSchemaSink struct {
	filament.Sink
	columns [][]string
}

func (*messageSchemaSink) Spec() filament.SinkSpec { return filament.SinkSpec{} }
func (s *messageSchemaSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	names := make([]string, b.Rows().Schema().NumFields())
	for i, f := range b.Rows().Schema().Fields() {
		names[i] = f.Name
	}
	s.columns = append(s.columns, names)
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func TestLiveLayoutSharedAcrossPhysicalStreams(t *testing.T) {
	url := os.Getenv("FILAMENT_TEST_NATS_URL")
	if url == "" {
		t.Skip("set FILAMENT_TEST_NATS_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nc, err := nats.Connect(url)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	name := "layout_" + uuid.NewString()[:8]
	for _, suffix := range []string{"a", "b"} {
		streamName := name + suffix
		if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: streamName, Subjects: []string{name + "." + suffix}}); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = js.DeleteStream(context.Background(), streamName) }()
	}
	resource := name + ".>"
	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"url": url})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(ctx)
	attempt := filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}
	opened, err := src.OpenStream(ctx, filament.StreamOpenOpts{SourceConnectionID: "source", Resources: []string{resource}, Attempt: attempt, CheckAuthority: func(context.Context) error { return nil }, MessageSchemaPolicy: schema.NullableAdd})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close(ctx)
	sink := &messageSchemaSink{}
	p, err := pipeline.NewStream(pipeline.Config{Sink: sink, MessageSchemaPolicy: schema.NullableAdd, WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullAppend)}}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(ctx)
	defer func() {
		p.CloseIngest(p.Err())
		if err := p.Wait(); err != nil {
			t.Error(err)
		}
	}()
	for i, tc := range []struct{ subject, payload string }{{"a", `{"id":1}`}, {"b", `{"id":2,"email":"x"}`}, {"a", `{"id":3}`}} {
		if _, err := js.Publish(ctx, name+"."+tc.subject, []byte(tc.payload)); err != nil {
			t.Fatal(err)
		}
		if err := p.BindEpoch(filament.EpochRef{Attempt: attempt, Epoch: int64(i + 1), MembershipRevision: 1}); err != nil {
			t.Fatal(err)
		}
		coverage, err := opened.Read(ctx, p.Records().(filament.StreamRecordSink), filament.Boundary{MaxRecords: 1, MaxWait: 50 * time.Millisecond})
		if err != nil {
			t.Fatal(err)
		}
		proof, err := p.SealEpoch()
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if i > 0 {
			want = 2
		}
		if len(proof.MessageSchemas()[resource].Fields) != want {
			t.Fatal("layout not shared across backing streams")
		}
		if err := opened.Acknowledge(ctx, coverage); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.columns) != 3 || len(sink.columns[0]) != 7 || len(sink.columns[1]) != 8 || len(sink.columns[2]) != 8 {
		t.Fatalf("stale shared builder: %v", sink.columns)
	}
}
