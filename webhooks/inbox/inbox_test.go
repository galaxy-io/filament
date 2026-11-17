package inbox

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type testStream struct {
	jetstream.Stream
	info *jetstream.StreamInfo
	next jetstream.Consumer
}

type testJetStream struct {
	jetstream.JetStream
	stream  *testStream
	updates int
}

func (j *testJetStream) Stream(context.Context, string) (jetstream.Stream, error) {
	return j.stream, nil
}

func (j *testJetStream) UpdateStream(_ context.Context, cfg jetstream.StreamConfig) (jetstream.Stream, error) {
	j.updates++
	j.stream.info.Config = cfg
	return j.stream, nil
}

func (s *testStream) Info(context.Context, ...jetstream.StreamInfoOpt) (*jetstream.StreamInfo, error) {
	return s.info, nil
}

func (s *testStream) GetMsg(context.Context, uint64, ...jetstream.GetMsgOpt) (*jetstream.RawStreamMsg, error) {
	return &jetstream.RawStreamMsg{Time: time.Now()}, nil
}

func (s *testStream) OrderedConsumer(context.Context, jetstream.OrderedConsumerConfig) (jetstream.Consumer, error) {
	return s.next, nil
}

type timeoutConsumer struct{ jetstream.Consumer }

func (*timeoutConsumer) Next(...jetstream.FetchOpt) (jetstream.Msg, error) {
	return nil, nats.ErrTimeout
}

func TestFirstAfterDoesNotTreatTimeoutAsCutoff(t *testing.T) {
	stream := &testStream{next: &timeoutConsumer{}}
	inbox := &Inbox{stream: stream}
	_, err := inbox.firstAfter(context.Background(), time.Now().Add(-time.Hour), 10)
	if !errors.Is(err, nats.ErrTimeout) {
		t.Fatalf("expected lookup timeout, got %v", err)
	}
}

func TestReaderRejectsReplacedStream(t *testing.T) {
	original := time.Now().Add(-time.Hour)
	stream := &testStream{info: &jetstream.StreamInfo{Created: time.Now()}}
	inbox := &Inbox{stream: stream, incarnation: original}
	if _, err := inbox.Reader(context.Background(), "tenant", "source", 5, original); err == nil {
		t.Fatal("expected a replaced stream to be rejected")
	}
}

func TestOpenExpandsCapacityWithoutReplacingStream(t *testing.T) {
	created := time.Now().Add(-time.Hour)
	stream := &testStream{info: &jetstream.StreamInfo{
		Created: created,
		Config: jetstream.StreamConfig{
			Name: DefaultStream, Subjects: []string{subjectRoot + ".>"},
			Retention: jetstream.LimitsPolicy, Storage: jetstream.FileStorage,
			Discard: jetstream.DiscardNew, DiscardNewPerSubject: true,
			MaxMsgsPerSubject: 1, MaxBytes: 1024, MaxMsgSize: 2048, Replicas: 1,
		},
	}}
	js := &testJetStream{stream: stream}
	opened, err := Open(context.Background(), js, Config{MaxBytes: 4096, MaxMsgSize: 4096, Replicas: 1})
	if err != nil {
		t.Fatal(err)
	}
	if js.updates != 1 || stream.info.Config.MaxBytes != 4096 || stream.info.Config.MaxMsgSize != 4096 || !opened.incarnation.Equal(created) {
		t.Fatalf("capacity expansion changed stream identity or was not applied: updates=%d, config=%+v", js.updates, stream.info.Config)
	}
	if _, err := Open(context.Background(), js, Config{MaxBytes: 2048, MaxMsgSize: 3072, Replicas: 1}); err != nil {
		t.Fatalf("lower configured capacity should not shrink the stream: %v", err)
	}
	if js.updates != 1 {
		t.Fatalf("unexpected additional stream update: %d", js.updates)
	}
}
