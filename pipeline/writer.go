package pipeline

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/events"
)

// writer drains batches, computes the read-side CRC over the batch, writes it to
// the Sink, and compares that read CRC against the Sink's write-side CRC.
// This double check is _mostly_ free in terms of allocs/processing and is an added layer
// of defense against bitflips during processing.
func (p *Pipeline) writer(ctx context.Context) {
	defer p.wg.Done()

	for b := range p.batchCh {
		// Once any writer fails, the source is stopped but already queued batches
		// still belong to the pipeline. Drain and release them without calling the
		// sink so every successful ownership transfer has a terminal Release.
		if p.Err() != nil {
			b.Release()
			continue
		}
		p.processBatch(ctx, b)
	}
}

func (p *Pipeline) processBatch(ctx context.Context, b *arrowbatch.Batch) (ok bool) {
	defer b.Release()
	policy, err := p.policyFor(b.Resource)
	if err != nil {
		p.setErr(err)
		return false
	}
	// A bitmap completion marker carries no rows: publish its want delta (the part's
	// expected ack total) and skip the sink write entirely.
	if b.Drained {
		p.publish(events.NewFact(events.BatchWritten, events.Envelope{Resource: b.Resource},
			events.BatchWrittenEvent{Checkpoint: b.Cursor, CheckpointPolicy: policy.Checkpoint}))
		return true
	}

	readCRC := b.IntegrityCRC()
	receipt, err := p.writeBatch(ctx, b, policy)
	if err != nil {
		p.setErr(fmt.Errorf("write %s seq %d: %w", b.Resource, b.Seq, err))
		return false
	}
	if p.encodedIntegrity && receipt.EncodedCRC == nil {
		p.setErr(fmt.Errorf("write %s seq %d: sink %q requires encoded integrity evidence but returned none", b.Resource, b.Seq, p.sink.Name()))
		return false
	}

	if receipt.WriteCRC == readCRC {
		p.publish(events.NewFact(events.BatchWritten, events.Envelope{Resource: b.Resource},
			events.BatchWrittenEvent{
				Records: int64(b.NumRows()), Bytes: receipt.Bytes,
				URI: receipt.URI, CRC: receipt.WriteCRC,
				Checkpoint:       receiptCheckpoint(receipt, b),
				CheckpointPolicy: policy.Checkpoint,
			}))
		p.publish(events.NewFact(events.IntegrityVerified, events.Envelope{Resource: b.Resource},
			events.IntegrityVerifiedEvent{CRC: readCRC}))
		if receipt.EncodedCRC != nil {
			p.publish(events.NewFact(events.EncodedIntegrityVerified, events.Envelope{Resource: b.Resource},
				events.EncodedIntegrityVerifiedEvent{CRC: *receipt.EncodedCRC}))
		}
		return true
	}

	if p.log != nil {
		p.log.Warn("chunk divergence",
			filament.Field{Key: "event.name", Value: "pipeline.chunk.divergence"},
			filament.Field{Key: "resource", Value: b.Resource},
			filament.Field{Key: "seq", Value: b.Seq},
			filament.Field{Key: "read_crc", Value: readCRC},
			filament.Field{Key: "write_crc", Value: receipt.WriteCRC},
		)
	}
	p.publish(events.NewFact(events.ChunkDivergence, events.Envelope{Resource: b.Resource},
		events.ChunkDivergenceEvent{CRC: readCRC}))
	p.setErr(fmt.Errorf("write %s seq %d: CRC divergence: read %08x, write %08x", b.Resource, b.Seq, readCRC, receipt.WriteCRC))
	return false
}

func (p *Pipeline) writeBatch(ctx context.Context, b *arrowbatch.Batch, policy filament.WritePolicy) (filament.WriteReceipt, error) {
	return p.sink.Apply(ctx, b, filament.ApplyOptions{Policy: policy})
}

func (p *Pipeline) policyFor(resource string) (filament.WritePolicy, error) {
	if p.writePolicies != nil {
		if policy, ok := p.writePolicies[resource]; ok {
			return policy, nil
		}
		if policy, ok := p.writePolicies[""]; ok {
			policy.Resource = resource
			return policy, nil
		}
	}
	return filament.WritePolicy{}, fmt.Errorf("missing write policy for resource %q", resource)
}

func receiptCheckpoint(receipt filament.WriteReceipt, b *arrowbatch.Batch) *filament.CheckpointData {
	if receipt.Checkpoint != nil {
		return receipt.Checkpoint
	}
	return b.Cursor
}
