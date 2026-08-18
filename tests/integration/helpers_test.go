//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
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
	batches []filament.Batch
	wrap    func(filament.RowWriter) filament.RowWriter // optional per-writer wrapper
}

func (s *collectSink) Builder(resource string, part int, schema filament.RecordSchema) (filament.RowWriter, error) {
	as := batch.Schema(schema)
	c := &collectChunks{sink: s, resource: resource, part: part, pk: schema.PrimaryKey}
	var w filament.RowWriter = batch.New(as, batch.Options{MaxRows: 1}, c) // one batch per row: nothing waits in a builder after Extract
	if s.wrap != nil {
		w = s.wrap(w)
	}
	return w, nil
}

// batchesFor returns the collected batches of one resource, in flush order.
func (s *collectSink) batchesFor(resource string) []filament.Batch {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []filament.Batch
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

func (c *collectChunks) Chunk(ch batch.Chunk) error {
	ch.Rows.Retain()
	c.sink.mu.Lock()
	defer c.sink.mu.Unlock()
	c.sink.batches = append(c.sink.batches, filament.Batch{Resource: c.resource, Part: c.part, Rows: ch.Rows, Ops: ch.Ops})
	for i := range int(ch.Rows.NumRows()) {
		r := rec{Resource: c.resource, Key: ch.Last.Key, Coarse: ch.Last.Coarse, LSN: ch.Last.LSN, Seq: ch.Last.Seq}
		if ch.Ops != nil {
			r.Op = ch.Ops[i]
		}
		for n, name := range c.pk { // key columns' text, joined with 0x1f
			if idx := ch.Rows.Schema().FieldIndices(name); len(idx) == 1 {
				if n > 0 {
					r.ID += "\x1f"
				}
				r.ID += ch.Rows.Column(idx[0]).ValueStr(i)
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
func applyAll(ctx context.Context, dst filament.Sink, batches []filament.Batch, policy filament.WritePolicy) error {
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

func (s *flakySink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.writes++
	if s.failAt > 0 && s.writes == s.failAt {
		return filament.WriteReceipt{}, fmt.Errorf("injected write failure at batch %d", s.writes)
	}
	return s.Sink.Apply(ctx, b, opts)
}

func waitStatus(t *testing.T, ctx context.Context, store filament.DataStore, id filament.RunID, want filament.RunStatus) filament.RunState {
	t.Helper()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := store.LoadRun(ctx, id)
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
