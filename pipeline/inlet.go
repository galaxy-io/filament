package pipeline

import (
	"errors"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/events"
)

// ErrPipelineClosed is returned to a builder when it flushes after the pipeline
// has shut down (a fatal write error with no more specific cause).
var ErrPipelineClosed = errors.New("pipeline: closed")

// inlet is the Source's view of the pipeline: a filament.RecordSink that opens
// one batch.Builder per (resource, part). Each builder's chunks become batches
// on the writer channel; a full channel blocks the source, applying backpressure.
type inlet struct{ p *Pipeline }

var _ filament.RecordSink = (*inlet)(nil)

// Builder opens the row writer for one (resource, part). Safe to call from
// multiple goroutines.
func (in *inlet) Builder(resource string, part int, schema filament.RecordSchema) (filament.RowWriter, error) {
	p := in.p
	b := batch.New(batch.Schema(schema), p.opts, &slot{p: p, resource: resource, part: part})
	p.buildersMu.Lock()
	p.builders = append(p.builders, b)
	p.buildersMu.Unlock()
	return b, nil
}

// slot receives one builder's chunks: it sequences them per (resource, part),
// derives the checkpoint delta from the last row's meta, and queues the batch.
type slot struct {
	p        *Pipeline
	resource string
	part     int
	seq      uint64
}

var _ batch.Receiver = (*slot)(nil)

func (s *slot) Chunk(c batch.Chunk) error {
	seq := s.seq
	s.seq++
	rows := int64(c.Rows.NumRows())
	nbytes := batch.Bytes(c.Rows) // before send: the writer releases the rows after Apply
	b := filament.Batch{
		Tenant:   s.p.tenant,
		Run:      s.p.run,
		Resource: s.resource,
		Part:     s.part,
		Seq:      seq,
		Rows:     c.Rows,
		Ops:      c.Ops,
		Cursor:   cursorOf(s.resource, s.part, c.Last, int(rows)),
	}
	if err := s.send(b); err != nil {
		c.Rows.Release()
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
	return s.send(filament.Batch{
		Tenant:   s.p.tenant,
		Run:      s.p.run,
		Resource: s.resource,
		Part:     s.part,
		Drained:  true,
		Cursor:   cursor,
	})
}

// send queues a batch, blocking on backpressure. It returns the pipeline error
// (or ErrPipelineClosed) once the writer has given up, so a Source stops
// extracting instead of spinning against a dead pipeline.
func (s *slot) send(b filament.Batch) error {
	select {
	case s.p.batchCh <- b:
		return nil
	case <-s.p.done:
		if err := s.p.Err(); err != nil {
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
