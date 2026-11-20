//go:build integration

package rabbitmq_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/google/uuid"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/amqp"
	"github.com/rabbitmq/rabbitmq-stream-go-client/pkg/stream"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	rabbitsink "github.com/galaxy-io/filament/connectors/rabbitmq/sink"
	"github.com/galaxy-io/filament/rowmodel"
	tc "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestRabbitMQSink(t *testing.T) {
	uri := tc.RabbitMQContainer(t)
	t.Run("publish", func(t *testing.T) {
		name := "filament_" + uuid.NewString()
		env := declare(t, uri, name)
		defer env.Close()
		apply(t, uri, name, batch(t, []row{{id: "a", seq: 1, event: "evt-a"}, {id: "b", seq: 2, event: "evt-b"}}), false)
		got := consume(t, env, name, 2)
		if got[0]["id"] != "a" || got[1]["id"] != "b" {
			t.Fatalf("payloads: %#v", got)
		}
		if got[0]["op"] != "insert" || got[1]["op"] != "update" {
			t.Fatalf("operations: %#v", got)
		}
		if got[0]["event"] != "evt-a" {
			t.Fatalf("event id was not copied: %#v", got[0])
		}
	})
	t.Run("replay", func(t *testing.T) {
		name := "filament_" + uuid.NewString()
		env := declare(t, uri, name)
		defer env.Close()
		rows := batch(t, []row{{id: "a", seq: 1, event: "same"}, {id: "b", seq: 2, event: "same"}})
		apply(t, uri, name, rows, true)
		apply(t, uri, name, rows, false)
		apply(t, uri, name, batch(t, []row{{id: "c", seq: 3, event: "next"}}), false)
		got := consume(t, env, name, 3)
		if got[0]["id"] != "a" || got[1]["id"] != "b" || got[2]["id"] != "c" {
			t.Fatalf("replay stored %#v, want a, b, c once each", got)
		}
	})
	t.Run("missing", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()
		s := rabbitsink.New()
		err := s.Open(ctx, filament.RunSpec{Sink: filament.Ref{Config: map[string]any{"uri": uri, "stream": "missing_" + uuid.NewString()}}})
		if err == nil {
			_ = s.Close(context.Background())
			t.Fatal("missing stream accepted")
		}
	})
	t.Run("fresh watermark", func(t *testing.T) {
		name := "fresh_" + uuid.NewString()
		env := declare(t, uri, name)
		defer env.Close()
		producer, err := env.NewProducer(name, stream.NewProducerOptions().SetProducerName("filament-fresh"))
		if err != nil {
			t.Fatal(err)
		}
		defer producer.Close()
		last, err := producer.GetLastPublishingId()
		if err != nil && !errors.Is(err, stream.OffsetNotFoundError) {
			t.Fatalf("fresh GetLastPublishingId: %d %v", last, err)
		}
		if err == nil && last != 0 {
			t.Fatalf("fresh producer returned publishing id %d", last)
		}
		t.Logf("fresh GetLastPublishingId last=%d err=%v", last, err)
	})
}

type row struct {
	id, event string
	seq       int64
}

func batch(t *testing.T, rows []row) *arrowbatch.Batch {
	t.Helper()
	schema := rowmodel.Schema{Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalString},
		{Name: rowmodel.AuditSequenceField, Logical: rowmodel.LogicalInt64},
		{Name: rowmodel.EventIDField, Logical: rowmodel.LogicalString},
	}}
	rb := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(schema))
	defer rb.Release()
	ops := make([]filament.Operation, len(rows))
	for i, row := range rows {
		rb.Field(0).(*array.StringBuilder).Append(row.id)
		rb.Field(1).(*array.Int64Builder).Append(row.seq)
		rb.Field(2).(*array.StringBuilder).Append(row.event)
		switch i % 3 {
		case 0:
			ops[i] = filament.OpInsert
		case 1:
			ops[i] = filament.OpUpdate
		default:
			ops[i] = filament.OpDelete
		}
	}
	b := arrowbatch.NewBatch(rb.NewRecordBatch(), arrowbatch.NewOperations(ops))
	b.Resource = "events"
	t.Cleanup(b.Release)
	return b
}

func declare(t *testing.T, uri, name string) *stream.Environment {
	t.Helper()
	env, err := stream.NewEnvironment(stream.NewEnvironmentOptions().SetUri(uri))
	if err != nil {
		t.Fatal(err)
	}
	if err := env.DeclareStream(name, nil); err != nil {
		_ = env.Close()
		t.Fatal(err)
	}
	return env
}

func apply(t *testing.T, uri, streamName string, b *arrowbatch.Batch, failConfirm bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	s := rabbitsink.New()
	run := filament.RunSpec{Tenant: "tenant", PipelineID: "pipeline", SinkConnectionID: "sink", Sink: filament.Ref{Config: map[string]any{"uri": uri, "stream": streamName}}}
	if err := s.Open(ctx, run); err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	if err := s.Begin(ctx); err != nil {
		t.Fatal(err)
	}
	if failConfirm {
		rabbitsink.SetConfirmFaultForTest(func() error { return errors.New("lost confirm") })
		t.Cleanup(func() { rabbitsink.SetConfirmFaultForTest(nil) })
	}
	opts := filament.ApplyOptions{Policy: filament.WritePolicy{Capability: s.Spec().Capabilities.WritePolicies[0]}}
	receipt, err := s.Apply(ctx, b, opts)
	if failConfirm {
		if err == nil {
			t.Fatal("confirm fault did not fail Apply")
		}
		return
	}
	if err != nil || receipt.Rows != b.NumRows() || receipt.EncodedCRC == nil {
		t.Fatalf("Apply: %+v %v", receipt, err)
	}
	if err := s.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func consume(t *testing.T, env *stream.Environment, name string, want int) []map[string]string {
	t.Helper()
	var mu sync.Mutex
	var got []map[string]string
	handler := func(_ stream.ConsumerContext, msg *amqp.Message) {
		body := map[string]any{}
		if err := json.Unmarshal(msg.GetData(), &body); err != nil {
			return
		}
		row := map[string]string{}
		if id, ok := body["id"].(string); ok {
			row["id"] = id
		}
		if event, ok := body[rowmodel.EventIDField].(string); ok {
			row["event"] = event
		}
		if props := msg.ApplicationProperties; props != nil {
			if op, ok := props["Filament-Operation"].(string); ok {
				row["op"] = op
			}
			if event, ok := props["Filament-Event-Id"].(string); ok {
				row["event"] = event
			}
		}
		mu.Lock()
		got = append(got, row)
		mu.Unlock()
	}
	consumer, err := env.NewConsumer(name, handler, stream.NewConsumerOptions().SetOffset(stream.OffsetSpecification{}.First()))
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	deadline := time.Now().Add(20 * time.Second)
	var settled time.Time
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(got)
		mu.Unlock()
		if n >= want {
			if settled.IsZero() {
				settled = time.Now().Add(400 * time.Millisecond)
			}
			if !time.Now().Before(settled) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	out := append([]map[string]string(nil), got...)
	if len(out) != want {
		t.Fatalf("consumed %d, want %d: %#v", len(out), want, out)
	}
	return out
}
