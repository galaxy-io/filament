package pipeline

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func boundary(kind rowmodel.ControlKind) rowmodel.Control {
	c := rowmodel.Control{Kind: kind, Domain: rowmodel.DomainKey{Incarnation: "source", Domain: "log"}, Position: rowmodel.Position{Codec: "opaque", Value: []byte("position")}}
	if kind == rowmodel.TxnBegin || kind == rowmodel.TxnEnd {
		c.TxnID = "txn"
	}
	return c
}

func streamForTest(t *testing.T, sink filament.Sink, order filament.Ordering) *Pipeline {
	t.Helper()
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	p, err := NewStream(Config{Sink: sink, Allocator: alloc, Options: filament.RunOptions{BatchMaxRows: 100, SnapshotParallelism: 8}, FlushInterval: time.Hour, WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullReplace)}}, order)
	if err != nil {
		t.Fatal(err)
	}
	if p.writers != 1 {
		t.Fatal("stream must have one writer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	p.Start(ctx)
	t.Cleanup(func() {
		p.CloseIngest(p.Err())
		_ = p.Wait()
		cancel()
		alloc.AssertSize(t, 0)
	})
	return p
}

func streamRow(t *testing.T, w arrowbatch.RowWriter, id int64) {
	t.Helper()
	w.Int64(id)
	w.String("x")
	if err := w.EndRow(rowmodel.Meta{LSN: "bounded-must-not-checkpoint", Coarse: true}); err != nil {
		t.Fatal(err)
	}
}

func streamBuilder(t *testing.T, p *Pipeline, resource string) arrowbatch.RowWriter {
	t.Helper()
	w, err := p.Records().Builder(resource, 0, schema)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestStreamRepeatedBarriersFlushIdleRows(t *testing.T) {
	sink := &fakeSink{}
	p := streamForTest(t, sink, filament.OrderingNone)
	w := streamBuilder(t, p, "users")
	for i := range 2 {
		streamRow(t, w, int64(i))
		control := boundary(rowmodel.ProgressBoundary)
		r, err := p.Barrier(context.Background(), control)
		if err != nil {
			t.Fatal(err)
		}
		control.Position.Value[0] = 'X'
		if r.Sequence != uint64(i+1) || string(r.Boundary.Position.Value) != "position" {
			t.Fatalf("wrong receipt: %+v", r)
		}
		batches := sink.batches()
		if len(batches) != i+1 || batches[i].rows != 1 || batches[i].cursor != nil {
			t.Fatalf("wrong batches: %+v", batches)
		}
	}
	if err := w.Drain(rowmodel.Meta{Coarse: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches()) != 2 {
		t.Fatal("control or drain reached sink")
	}
}

type blockingStreamSink struct {
	fakeSink
	entered chan struct{}
	release chan struct{}
}

func (s *blockingStreamSink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	select {
	case s.entered <- struct{}{}:
	default:
	}
	select {
	case <-s.release:
		return s.fakeSink.Apply(ctx, b, opts)
	case <-ctx.Done():
		return filament.WriteReceipt{}, ctx.Err()
	}
}

func TestStreamBarrierWaitsForApply(t *testing.T) {
	sink := &blockingStreamSink{entered: make(chan struct{}, 1), release: make(chan struct{})}
	p := streamForTest(t, sink, filament.OrderingNone)
	result := make(chan error, 1)
	go func() {
		// The same goroutine owns row production and the control.
		err := push(p.Records(), []row{rec("users", 1)})
		if err == nil {
			_, err = p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary))
		}
		result <- err
	}()
	select {
	case <-sink.entered:
	case <-time.After(time.Second):
		t.Fatal("Apply not reached")
	}
	select {
	case err := <-result:
		t.Fatalf("barrier completed while Apply blocked: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(sink.release)
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("barrier did not complete")
	}
}

func TestStreamBarrierCancellation(t *testing.T) {
	sink := &blockingStreamSink{entered: make(chan struct{}, 1), release: make(chan struct{})}
	p := streamForTest(t, sink, filament.OrderingNone)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		err := push(p.Records(), []row{rec("users", 1)})
		if err == nil {
			_, err = p.Barrier(ctx, boundary(rowmodel.ProgressBoundary))
		}
		result <- err
	}()
	select {
	case <-sink.entered:
	case <-time.After(time.Second):
		t.Fatal("Apply not reached")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation blocked")
	}
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err == nil {
		t.Fatal("barrier after cancellation succeeded")
	}
}

func TestStreamBarrierWriteFailures(t *testing.T) {
	for name, sink := range map[string]*fakeSink{"apply": {failOn: 1}, "integrity": {corrupt: true}, "encoded": {encodedIntegrity: true, omitEncoded: true}} {
		t.Run(name, func(t *testing.T) {
			p := streamForTest(t, sink, filament.OrderingNone)
			streamRow(t, streamBuilder(t, p, "users"), 1)
			if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err == nil {
				t.Fatal("failed write accepted")
			}
			if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err == nil {
				t.Fatal("later barrier accepted")
			}
		})
	}
}

func TestStreamEmptyTransaction(t *testing.T) {
	sink := &fakeSink{}
	p := streamForTest(t, sink, filament.OrderingTransaction)
	in := p.Records().(filament.StreamRecordSink)
	for range 2 {
		if err := in.Control(context.Background(), boundary(rowmodel.TxnBegin)); err != nil {
			t.Fatal(err)
		}
		if err := in.Control(context.Background(), boundary(rowmodel.TxnEnd)); err != nil {
			t.Fatal(err)
		}
	}
	if len(sink.batches()) != 0 {
		t.Fatal("empty transaction wrote data")
	}
}

func TestStreamRejectsProgressInsideTransaction(t *testing.T) {
	p := streamForTest(t, &fakeSink{}, filament.OrderingTransaction)
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.TxnBegin)); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err == nil {
		t.Fatal("progress inside transaction accepted")
	}
}

func TestStreamFlushRejectsPartialRow(t *testing.T) {
	p := streamForTest(t, &fakeSink{}, filament.OrderingNone)
	w := streamBuilder(t, p, "users")
	streamRow(t, w, 1)
	w.Int64(2)
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err == nil {
		t.Fatal("partial row accepted")
	}
}

func TestStreamStrictResourceSwitch(t *testing.T) {
	sink := &fakeSink{}
	p := streamForTest(t, sink, filament.OrderingGlobalStrict)
	a, b := streamBuilder(t, p, "a"), streamBuilder(t, p, "b")
	streamRow(t, a, 1)
	streamRow(t, b, 2)
	streamRow(t, a, 3)
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err != nil {
		t.Fatal(err)
	}
	var resources []string
	for _, batch := range sink.batches() {
		resources = append(resources, batch.resource)
	}
	if !reflect.DeepEqual(resources, []string{"a", "b", "a"}) {
		t.Fatalf("reordered: %v", resources)
	}
}

func TestStreamStrictSwitchRejectsPartialRow(t *testing.T) {
	p := streamForTest(t, &fakeSink{}, filament.OrderingGlobalStrict)
	a, b := streamBuilder(t, p, "a"), streamBuilder(t, p, "b")
	a.Int64(1)
	b.Int64(2)
	if err := b.EndRow(rowmodel.Meta{}); err == nil {
		t.Fatal("mid-row switch accepted")
	}
}

func TestStreamRejectsPartitionScheduling(t *testing.T) {
	for _, order := range []filament.Ordering{"", filament.OrderingPartition, "unknown"} {
		if _, err := NewStream(Config{}, order); err == nil {
			t.Fatalf("accepted %q", order)
		}
	}
	p := New(Config{})
	if _, ok := p.Records().(filament.StreamRecordSink); ok {
		t.Fatal("bounded inlet exposes streaming controls")
	}
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err == nil {
		t.Fatal("bounded barrier accepted")
	}
}

func TestStreamCancellationUnblocksOwnerFlush(t *testing.T) {
	sink := &blockingStreamSink{entered: make(chan struct{}, 1), release: make(chan struct{})}
	p := streamForTest(t, sink, filament.OrderingNone)
	// Four partial builders exceed the queue capacity while the first Apply blocks.
	for _, resource := range []string{"a", "b", "c", "d"} {
		streamRow(t, streamBuilder(t, p, resource), 1)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { _, err := p.Barrier(ctx, boundary(rowmodel.ProgressBoundary)); result <- err }()
	select {
	case <-sink.entered:
	case <-time.After(time.Second):
		t.Fatal("Apply not reached")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("owner flush did not cancel")
	}
}

func TestStreamClosedWriterDoesNotBlockSwitch(t *testing.T) {
	sink := &fakeSink{}
	p := streamForTest(t, sink, filament.OrderingGlobalStrict)
	a, b := streamBuilder(t, p, "a"), streamBuilder(t, p, "b")
	streamRow(t, a, 1)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	streamRow(t, b, 2)
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.ProgressBoundary)); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches()) != 2 {
		t.Fatal("closed writer lost rows")
	}
}

func TestStreamCloseRejectsOpenTransaction(t *testing.T) {
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	p, err := NewStream(Config{Sink: &fakeSink{}, Allocator: alloc}, filament.OrderingTransaction)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	if _, err = p.Barrier(context.Background(), boundary(rowmodel.TxnBegin)); err != nil {
		p.CloseIngest(err)
		_ = p.Wait()
		t.Fatal(err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err == nil {
		t.Fatal("clean closure accepted an open transaction")
	}
	alloc.AssertSize(t, 0)
	if _, err := p.Barrier(context.Background(), boundary(rowmodel.TxnEnd)); err == nil {
		t.Fatal("barrier accepted after closure")
	}
}
