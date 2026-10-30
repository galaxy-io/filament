//go:build integration || e2e

// Package testutil contains helpers shared by the tagged test suites. It is
// internal to the tests module so none of these types become product APIs.
package testutil

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
)

// Record is one data row or completion marker captured by CollectSink.
type Record struct {
	Resource string
	ID       string
	Op       filament.Operation
	Key      []string
	Coarse   bool
	LSN      string
	Seq      uint64
	Drained  bool
}

// CollectSink retains extracted records and Arrow batches for assertions or
// replay into a real sink.
type CollectSink struct {
	mu      sync.Mutex
	Records []Record
	batches []*arrowbatch.Batch
	wrap    func(filament.RowWriter) filament.RowWriter
}

func (s *CollectSink) Builder(resource string, part int, schema filament.RecordSchema) (filament.RowWriter, error) {
	as := arrowbatch.Schema(schema)
	c := &collectChunks{sink: s, resource: resource, part: part, pk: schema.PrimaryKey}
	var writer filament.RowWriter = arrowbatch.NewBuilder(as, nil, arrowbatch.Options{MaxRows: 1}, c)
	if s.wrap != nil {
		writer = s.wrap(writer)
	}
	return writer, nil
}

// SetWriterWrapper installs a wrapper before the first writer is built.
func (s *CollectSink) SetWriterWrapper(wrap func(filament.RowWriter) filament.RowWriter) {
	s.wrap = wrap
}

// BatchesFor returns retained batches for one resource in flush order.
func (s *CollectSink) BatchesFor(resource string) []*arrowbatch.Batch {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*arrowbatch.Batch
	for _, batch := range s.batches {
		if batch.Resource == resource {
			out = append(out, batch)
		}
	}
	return out
}

// Release releases all retained Arrow batches.
func (s *CollectSink) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, batch := range s.batches {
		batch.Release()
	}
	s.batches = nil
}

type collectChunks struct {
	sink     *CollectSink
	resource string
	part     int
	pk       []string
}

func (c *collectChunks) Chunk(batch *arrowbatch.Batch) error {
	batch.Resource, batch.Part = c.resource, c.part
	c.sink.mu.Lock()
	defer c.sink.mu.Unlock()
	c.sink.batches = append(c.sink.batches, batch)
	rows, ops := batch.Rows(), batch.Operations()
	for i := range int(rows.NumRows()) {
		record := Record{Resource: c.resource, Key: batch.Last.Key, Coarse: batch.Last.Coarse, LSN: batch.Last.LSN, Seq: batch.Last.Seq}
		if ops.Len() != 0 {
			record.Op = ops.At(i)
		}
		for n, name := range c.pk {
			if indexes := rows.Schema().FieldIndices(name); len(indexes) == 1 {
				if n > 0 {
					record.ID += "\x1f"
				}
				record.ID += rows.Column(indexes[0]).ValueStr(i)
			}
		}
		c.sink.Records = append(c.sink.Records, record)
	}
	return nil
}

func (c *collectChunks) Drained(meta filament.RowMeta, _ int) error {
	c.sink.mu.Lock()
	defer c.sink.mu.Unlock()
	c.sink.Records = append(c.sink.Records, Record{
		Resource: c.resource,
		Drained:  true,
		LSN:      meta.LSN,
		Seq:      meta.Seq,
		Coarse:   true,
	})
	return nil
}

// DataRecords filters completion markers from captured records.
func DataRecords(records []Record) []Record {
	out := make([]Record, 0, len(records))
	for _, record := range records {
		if !record.Drained {
			out = append(out, record)
		}
	}
	return out
}

// StreamCheckpoint returns the last stream completion marker for a resource.
func StreamCheckpoint(t testing.TB, records []Record, resource string) filament.Checkpoint {
	t.Helper()
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Resource == resource && records[i].Drained && records[i].LSN != "" {
			return checkpoint.NewStreamDelta(resource, records[i].LSN, records[i].Seq)
		}
	}
	t.Fatalf("no stream marker for %q in %#v", resource, records)
	return nil
}

// ApplyAll replays retained batches into a sink under one policy.
func ApplyAll(ctx context.Context, sink filament.Sink, batches []*arrowbatch.Batch, policy filament.WritePolicy) error {
	for _, batch := range batches {
		if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
			return err
		}
	}
	return nil
}

type flakySink struct {
	filament.Sink
	failAt int
	writes int
}

// NewFlakySink wraps a sink and fails the requested Apply call.
func NewFlakySink(sink filament.Sink, failAt int) filament.Sink {
	return &flakySink{Sink: sink, failAt: failAt}
}

func (s *flakySink) Apply(ctx context.Context, batch *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return filament.WriteReceipt{}, fmt.Errorf("injected write failure at batch %d", s.writes)
	}
	return s.Sink.Apply(ctx, batch, opts)
}

// WaitStatus blocks until a run reaches the requested status or ctx expires.
func WaitStatus(t testing.TB, ctx context.Context, store filament.DataStore, tenant filament.TenantID, id filament.RunID, want filament.RunStatus) filament.RunState {
	return WaitForStatuses(t, ctx, store, tenant, id, want)
}

// WaitForStatuses blocks until a run reaches any requested status or ctx expires.
func WaitForStatuses(t testing.TB, ctx context.Context, store filament.DataStore, tenant filament.TenantID, id filament.RunID, statuses ...filament.RunStatus) filament.RunState {
	t.Helper()
	want := make(map[filament.RunStatus]bool, len(statuses))
	for _, status := range statuses {
		want[status] = true
	}
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, tenant, id)
		if err == nil && want[state.Status] {
			return state
		}
		select {
		case <-ctx.Done():
			if err == nil {
				t.Fatalf("waiting for run %s status in %v: last status %v error %q", id, statuses, state.Status, state.Error)
			}
			t.Fatalf("waiting for run %s status in %v: %v", id, statuses, err)
		case <-ticker.C:
		}
	}
}
