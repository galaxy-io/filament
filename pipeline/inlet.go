package pipeline

import (
	"errors"
	"fmt"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform"
)

// ErrPipelineClosed is returned to a builder when it flushes after the pipeline
// has shut down (a fatal write error with no more specific cause).
var ErrPipelineClosed = errors.New("pipeline: closed")

// inlet is the Source's view of the pipeline: a filament.RecordSink that opens
// one arrowbatch.Builder per (resource, part). Each builder's chunks become batches
// on the writer channel; a full channel blocks the source, applying backpressure.
type inlet struct{ p *Pipeline }

type partKey struct {
	resource string
	part     int
}

type registeredSchema struct {
	model rowmodel.Schema
	arrow *arrow.Schema
	plan  *transform.Plan // nil → the resource bypasses the transformer pool
}

var _ arrowbatch.Inlet = (*inlet)(nil)

// Builder opens the row writer for one (resource, part). Safe to call from
// multiple goroutines.
func (in *inlet) Builder(resource string, part int, supplied rowmodel.Schema) (arrowbatch.RowWriter, error) {
	p := in.p
	key := partKey{resource: resource, part: part}
	p.registryMu.Lock()
	defer p.registryMu.Unlock()
	schema := supplied.Clone()
	if schema.Resource == "" {
		schema.Resource = resource
	} else if schema.Resource != resource {
		return nil, fmt.Errorf("pipeline: schema resource %q does not match builder resource %q", schema.Resource, resource)
	}
	if p.audit != nil {
		var err error
		if p.audit.CDCAppend {
			schema = rowmodel.AsCDCAppendHistory(schema)
		}
		schema, err = rowmodel.WithAuditFields(schema, p.audit.CDC)
		if err != nil {
			return nil, fmt.Errorf("pipeline: %w", err)
		}
	}
	want := arrowbatch.Schema(schema)
	registered, ok := p.schemas[resource]
	if ok {
		if !registered.model.Equal(schema) {
			return nil, fmt.Errorf("pipeline: schema changed for resource %q", resource)
		}
		want = registered.arrow
	} else {
		plan, err := p.planFor(resource, supplied)
		if err != nil {
			return nil, err
		}
		registered = registeredSchema{model: schema, arrow: want, plan: plan}
		p.schemas[resource] = registered
	}
	if _, exists := p.builders[key]; exists {
		return nil, fmt.Errorf("pipeline: builder already open for %q part %d", resource, part)
	}
	out := p.batchCh
	if registered.plan != nil {
		out = p.transformCh
	}
	b := arrowbatch.NewBuilder(want, p.alloc, p.opts, &slot{p: p, out: out, resource: resource, part: part})
	p.builders[key] = b
	if p.audit == nil {
		return b, nil
	}
	return &auditWriter{RowWriter: b, run: p.run, audit: *p.audit}, nil
}

// auditWriter appends run lineage after the source has supplied its own fields.
type auditWriter struct {
	arrowbatch.RowWriter
	run   filament.RunID
	audit AuditConfig
}

func (w *auditWriter) EndRow(meta rowmodel.Meta) error {
	w.String(string(w.run))
	w.Timestamp(w.audit.RunStartedAt.UnixMicro())
	if w.audit.CDC {
		w.String(filament.OperationName(meta.Op))
		if meta.LSN == "" {
			w.Null()
			w.Null()
		} else {
			w.String(meta.LSN)
			w.Int64(int64(meta.Seq)) //nolint:gosec // stream sequence is persisted as int64 elsewhere
		}
	}
	return w.RowWriter.EndRow(meta)
}

// slot receives one builder's chunks: it sequences them per (resource, part),
// derives the checkpoint delta from the last row's meta, and queues the batch on
// out, the transform or writer channel chosen when the builder opened. Markers
// take the same channel as rows so a part's completion never overtakes its data.
type slot struct {
	p        *Pipeline
	out      chan *arrowbatch.Batch
	resource string
	part     int
	seq      uint64
}

var _ arrowbatch.Receiver = (*slot)(nil)

func (s *slot) Chunk(b *arrowbatch.Batch) error {
	seq := s.seq
	s.seq++
	rows := int64(b.NumRows())
	nbytes := b.Bytes()
	b.Resource = s.resource
	b.Part = s.part
	b.Seq = seq
	b.Cursor = cursorOf(s.resource, s.part, b.Last, int(rows))
	if err := s.send(b); err != nil {
		return err
	}
	s.p.publish(events.NewFact(events.BatchBuffered, events.Envelope{Resource: s.resource},
		events.BatchBufferedEvent{Records: rows, Bytes: nbytes}))
	return nil
}

// Drained queues the part's completion marker (no rows): a change stream's final
// position, persisted even when the run wrote nothing so the next run resumes from
// here, or a coarse (bitmap/ctid) part's total row count, which the tracker matches
// against acked rows before flagging the part complete.
func (s *slot) Drained(meta filament.RowMeta, total int) error {
	var cursor *filament.CheckpointData
	if meta.LSN != "" {
		cursor = checkpoint.NewStreamDelta(s.resource, meta.LSN, meta.Seq)
	} else {
		cursor = checkpoint.NewCoarseDone(s.resource, s.part, total)
	}
	b := arrowbatch.NewMarker()
	b.Resource, b.Part, b.Cursor = s.resource, s.part, cursor
	if err := s.send(b); err != nil {
		b.Release()
		return err
	}
	return nil
}

// send queues a batch, blocking on backpressure. It returns the pipeline error
// (or ErrPipelineClosed) once the writer has given up, so a Source stops
// extracting instead of spinning against a dead pipeline.
func (s *slot) send(b *arrowbatch.Batch) error { return s.p.send(s.out, b) }

// send queues b on ch, blocking on backpressure, until the pipeline gives up.
func (p *Pipeline) send(ch chan *arrowbatch.Batch, b *arrowbatch.Batch) error {
	select {
	case ch <- b:
		return nil
	case <-p.done:
		if err := p.Err(); err != nil {
			return err
		}
		return ErrPipelineClosed
	}
}

// cursorOf builds the per-shard checkpoint delta from a flushed chunk's last row. A
// change stream (LSN) carries the last row's stream position. A bitmap (Coarse) read
// has no key cursor, so the delta is an ack of this chunk's row count, which the
// tracker sums toward the shard's expected total. A keyset read carries the last
// row's key. A plain ctid read carries neither → nil.
func cursorOf(resource string, part int, last filament.RowMeta, rows int) *filament.CheckpointData {
	if last.LSN != "" {
		return checkpoint.NewStreamDelta(resource, last.LSN, last.Seq)
	}
	if last.Coarse {
		return checkpoint.NewCoarseAck(resource, part, rows)
	}
	if last.Key == nil {
		return nil
	}
	return checkpoint.NewShardDelta(resource, part, last.Key)
}
