package pipeline

import (
	"context"
	"time"

	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament"
)

// batcher accumulates records per resource and flushes a Batch
// when a resource reaches batchRows or the flush timer fires. On ingest close it
// flushes the remainder. Each resource carries its own chunk sequence, and a
// resource is only ever routed to one shard, so that sequence stays monotonic.
func (p *Pipeline) batcher(ctx context.Context, shard int) {
	defer p.batcherWg.Done()

	ingestCh := p.ingestChs[shard]
	// Buffer per (resource, part): a keyset read fans a resource across parts whose
	// records interleave on this one inlet, and a batch must stay part-pure so its
	// cursor delta (the last record's key) belongs to a single shard.
	buffers := make(map[partKey][]ingestion.Record)
	chunkSeq := make(map[partKey]uint64)
	// seen counts every data record per part, so a bitmap part's Drained sentinel can
	// publish the exact row total the tracker must see acked before flagging it complete.
	seen := make(map[partKey]int)

	timer := time.NewTicker(p.flushIvl)
	defer timer.Stop()

	flush := func(key partKey) bool {
		recs := buffers[key]
		if len(recs) == 0 {
			return true
		}
		var nbytes int64
		for i := range recs {
			nbytes += int64(len(recs[i].Data))
		}
		seq := chunkSeq[key]
		chunkSeq[key] = seq + 1

		b := ingestion.Batch{
			Tenant:   p.tenant,
			Run:      p.run,
			Resource: key.resource,
			Part:     key.part,
			Seq:      seq,
			Records:  recs,
			Cursor:   shardCursor(key.resource, key.part, recs),
		}
		select {
		case p.batchCh <- b:
		case <-ctx.Done():
			return false
		}
		p.publish(ingestion.Event{
			Type:     ingestion.EvBatchBuffered,
			Resource: key.resource,
			Fields:   ingestion.EventFields{Records: int64(len(recs)), Bytes: nbytes},
		})
		// New backing array — the flushed slice now belongs to the batch.
		buffers[key] = nil
		return true
	}

	for {
		select {
		case rec, ok := <-ingestCh:
			if !ok {
				for resource := range buffers {
					if !flush(resource) {
						return
					}
				}
				return
			}
			key := partKey{rec.Resource, rec.Part}
			if rec.Drained {
				if !flush(key) {
					return
				}
				if !sendDrained(ctx, p, key, seen[key]) {
					return
				}
				continue
			}
			buffers[key] = append(buffers[key], rec)
			seen[key]++
			if len(buffers[key]) >= p.batchRows {
				if !flush(key) {
					return
				}
			}
		case <-timer.C:
			for resource := range buffers {
				if !flush(resource) {
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

// sendDrained queues a bitmap completion marker (no records) carrying the part's total
// row count, for the writer to publish as the part's want without a sink write. Returns
// false if the pipeline is shutting down.
func sendDrained(ctx context.Context, p *Pipeline, key partKey, total int) bool {
	b := ingestion.Batch{
		Tenant:   p.tenant,
		Run:      p.run,
		Resource: key.resource,
		Part:     key.part,
		Drained:  true,
		Cursor:   checkpoint.NewCoarseDone(key.resource, key.part, total),
	}
	select {
	case p.batchCh <- b:
		return true
	case <-ctx.Done():
		return false
	}
}

type partKey struct {
	resource string
	part     int
}

// shardCursor builds the per-shard checkpoint delta from a flushed batch. A bitmap
// (Coarse) read has no key cursor, so the delta is an ack of this batch's row count, which
// the tracker sums toward the shard's expected total. A keyset read carries the last
// record's key. A plain ctid read carries neither → nil.
func shardCursor(resource string, part int, recs []ingestion.Record) *ingestion.CheckpointData {
	last := recs[len(recs)-1]
	if last.Meta.LSN != "" {
		cp := ingestion.NewCheckpoint(resource)
		cp.Cursor["lsn"] = last.Meta.LSN
		if last.Meta.Seq > 0 {
			cp.Cursor["seq"] = int64(last.Meta.Seq)
		}
		return cp
	}
	if last.Coarse {
		return checkpoint.NewCoarseAck(resource, part, len(recs))
	}
	if last.Key == nil {
		return nil
	}
	return checkpoint.NewShardDelta(resource, part, last.Key)
}
