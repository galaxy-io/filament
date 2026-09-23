package source

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/stream"
	"github.com/galaxy-io/filament/pipeline"
)

type testSink struct{ filament.Sink }

func (*testSink) Spec() filament.SinkSpec { return filament.SinkSpec{} }
func (*testSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

type failingControl struct {
	filament.StreamRecordSink
	err error
}

func (s failingControl) Control(context.Context, filament.Control) error { return s.err }

func TestSessionIdleHeartbeatAndRecreation(t *testing.T) { testSession(t, false) }
func TestSessionPartialReadFailure(t *testing.T)         { testSession(t, true) }
func testSession(t *testing.T, failControl bool) {
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
	name := "session_" + uuid.NewString()[:8]
	streamCfg := jetstream.StreamConfig{Name: name, Subjects: []string{name}}
	if _, err := js.CreateStream(ctx, streamCfg); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = js.DeleteStream(context.Background(), name) }()
	cc := jetstream.ConsumerConfig{Durable: "consumer", AckPolicy: jetstream.AckExplicitPolicy, DeliverPolicy: jetstream.DeliverAllPolicy, MaxAckPending: 1, AckWait: time.Second}
	if _, err := js.CreateConsumer(ctx, name, cc); err != nil {
		t.Fatal(err)
	}
	src := New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"url": url, "stream": name, "consumer": "consumer"})); err != nil {
		t.Fatal(err)
	}
	defer src.Teardown(ctx)
	var authorityErr error
	opened, err := src.OpenStream(ctx, filament.StreamOpenOpts{SourceConnectionID: "source-connection", CheckAuthority: func(context.Context) error { return authorityErr }, Resources: []string{name}, Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close(ctx)
	p, err := pipeline.NewStream(pipeline.Config{Sink: &testSink{}, WritePolicies: map[string]filament.WritePolicy{name: {Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}}}, filament.OrderingNone)
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
	records := p.Records().(filament.StreamRecordSink)
	boundary := filament.Boundary{MaxRecords: 1, MaxWait: 50 * time.Millisecond}
	coverage, err := opened.Read(ctx, records, boundary)
	if err != nil || len(coverage.Positions) != 0 {
		t.Fatalf("idle: %+v %v", coverage, err)
	}
	if _, err := js.Publish(ctx, name, []byte("event")); err != nil {
		t.Fatal(err)
	}
	if failControl {
		cause := errors.New("boundary failed after row emission")
		_, err := opened.Read(ctx, failingControl{StreamRecordSink: records, err: cause}, boundary)
		if !errors.Is(err, cause) {
			t.Fatalf("partial read: %v", err)
		}
		if _, err := opened.Read(ctx, records, boundary); !errors.Is(err, cause) {
			t.Fatalf("read reused failed writer: %v", err)
		}
		if err := opened.Acknowledge(ctx, filament.Coverage{}); !errors.Is(err, cause) {
			t.Fatalf("ack accepted failed read: %v", err)
		}
		return
	}
	coverage, err = opened.Read(ctx, records, boundary)
	if err != nil || len(coverage.Positions) != 1 {
		t.Fatalf("read: %+v %v", coverage, err)
	}
	if _, err := opened.Read(ctx, records, boundary); !errors.Is(err, stream.ErrAcknowledgementPending) {
		t.Fatalf("read with pending coverage: %v", err)
	}
	wrong := coverage.Clone()
	for _, p := range wrong.Positions {
		p.Value[0] = '9'
	}
	if err := opened.Acknowledge(ctx, wrong); !errors.Is(err, filament.ErrIncompleteCoverage) {
		t.Fatalf("mismatched acknowledgement: %v", err)
	}
	// Hold the message longer than AckWait. A second pull must not redeliver it
	// while the session's independent InProgress task maintains the delivery.
	select {
	case <-time.After(1400 * time.Millisecond):
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	raw := opened.(*session)
	if _, err := raw.consumer.Next(jetstream.FetchMaxWait(100 * time.Millisecond)); !errors.Is(err, nats.ErrTimeout) {
		t.Fatalf("heartbeat allowed redelivery: %v", err)
	}
	authorityErr = filament.ErrFenced
	if err := opened.Acknowledge(ctx, coverage); !errors.Is(err, filament.ErrFenced) {
		t.Fatalf("ack bypassed authority: %v", err)
	}
	authorityErr = nil
	if err := opened.Acknowledge(ctx, coverage); err != nil {
		t.Fatal(err)
	}
	if err := js.DeleteStream(ctx, name); err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateStream(ctx, streamCfg); err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateConsumer(ctx, name, cc); err != nil {
		t.Fatal(err)
	}
	if _, err := opened.Read(ctx, records, boundary); !errors.Is(err, filament.ErrPositionIncomparable) {
		t.Fatalf("stream recreation accepted: %v", err)
	}
}
