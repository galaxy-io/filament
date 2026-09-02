//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

// rec is one row (or completion marker) as the tests inspect it: the resource,
// its primary key as text (columns joined with 0x1f), and its operation and
// resume metadata.
type rec struct {
	Resource string
	ID       string
	Op       filament.Operation
	Key      []string
	Coarse   bool
	LSN      string
	Seq      uint64
	Drained  bool
}

// collectSink is a filament.RecordSink that keeps every row as a rec and every
// flushed batch (retained) for replay into a sink.
type collectSink struct {
	mu      sync.Mutex
	recs    []rec
	batches []*arrowbatch.Batch
	wrap    func(filament.RowWriter) filament.RowWriter // optional per-writer wrapper
}

func (s *collectSink) Builder(resource string, part int, schema filament.RecordSchema) (filament.RowWriter, error) {
	as := arrowbatch.Schema(schema)
	c := &collectChunks{sink: s, resource: resource, part: part, pk: schema.PrimaryKey}
	var w filament.RowWriter = arrowbatch.NewBuilder(as, nil, arrowbatch.Options{MaxRows: 1}, c) // one batch per row: nothing waits in a builder after Extract
	if s.wrap != nil {
		w = s.wrap(w)
	}
	return w, nil
}

// batchesFor returns the collected batches of one resource, in flush order.
func (s *collectSink) batchesFor(resource string) []*arrowbatch.Batch {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*arrowbatch.Batch
	for _, b := range s.batches {
		if b.Resource == resource {
			out = append(out, b)
		}
	}
	return out
}

type collectChunks struct {
	sink     *collectSink
	resource string
	part     int
	pk       []string
}

func (c *collectChunks) Chunk(ch *arrowbatch.Batch) error {
	ch.Resource, ch.Part = c.resource, c.part
	c.sink.mu.Lock()
	defer c.sink.mu.Unlock()
	c.sink.batches = append(c.sink.batches, ch)
	rows, ops := ch.Rows(), ch.Operations()
	for i := range int(rows.NumRows()) {
		r := rec{Resource: c.resource, Key: ch.Last.Key, Coarse: ch.Last.Coarse, LSN: ch.Last.LSN, Seq: ch.Last.Seq}
		if ops.Len() != 0 {
			r.Op = ops.At(i)
		}
		for n, name := range c.pk { // key columns' text, joined with 0x1f
			if idx := rows.Schema().FieldIndices(name); len(idx) == 1 {
				if n > 0 {
					r.ID += "\x1f"
				}
				r.ID += rows.Column(idx[0]).ValueStr(i)
			}
		}
		c.sink.recs = append(c.sink.recs, r)
	}
	return nil
}

func (c *collectChunks) Drained(meta filament.RowMeta, _ int) error {
	c.sink.mu.Lock()
	defer c.sink.mu.Unlock()
	c.sink.recs = append(c.sink.recs, rec{Resource: c.resource, Drained: true, LSN: meta.LSN, Seq: meta.Seq, Coarse: true})
	return nil
}

// applyAll replays a resource's collected batches into a sink under one policy.
func applyAll(ctx context.Context, dst filament.Sink, batches []*arrowbatch.Batch, policy filament.WritePolicy) error {
	for _, b := range batches {
		if _, err := dst.Apply(ctx, b, filament.ApplyOptions{Policy: policy}); err != nil {
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

func (s *flakySink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return filament.WriteReceipt{}, fmt.Errorf("injected write failure at batch %d", s.writes)
	}
	return s.Sink.Apply(ctx, b, opts)
}

func waitStatus(t *testing.T, ctx context.Context, store filament.DataStore, tenant filament.TenantID, id filament.RunID, want filament.RunStatus) filament.RunState {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, tenant, id)
		if err == nil && state.Status == want {
			return state
		}
		select {
		case <-ctx.Done():
			if err == nil {
				t.Fatalf("waiting for run %s status %v: last status %v error %q", id, want, state.Status, state.Error)
			}
			t.Fatalf("waiting for run %s status %v: %v", id, want, err)
		case <-ticker.C:
		}
	}
}
