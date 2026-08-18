package pipeline

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// writer drains batches, computes the read-side CRC over the batch, writes it to
// the Sink, and compares that read CRC against the Sink's write-side CRC.
// This double check is _mostly_ free in terms of allocs/processing and is an added layer
// of defense against bitflips during processing.
func (p *Pipeline) writer(ctx context.Context) {
	defer p.wg.Done()

	for b := range p.batchCh {
		// A bitmap completion marker carries no rows: publish its want delta (the part's
		// expected ack total) and skip the sink write entirely.
		if b.Drained {
			p.publish(events.NewFact(events.BatchWritten, events.Envelope{Resource: b.Resource},
				events.BatchWrittenEvent{Checkpoint: b.Cursor}))
			continue
		}

		readCRC, _ := filament.CRC32C(b.Records)
		receipt, err := p.writeBatch(ctx, b)
		if err != nil {
			p.setErr(fmt.Errorf("write %s seq %d: %w", b.Resource, b.Seq, err))
			return
		}

		if receipt.WriteCRC == readCRC {
			p.publish(events.NewFact(events.BatchWritten, events.Envelope{Resource: b.Resource},
				events.BatchWrittenEvent{
					Records: int64(len(b.Records)), Bytes: receipt.Bytes,
					URI: receipt.URI, CRC: receipt.WriteCRC,
					Checkpoint: receiptCheckpoint(receipt, b),
				}))
			p.publish(events.NewFact(events.IntegrityVerified, events.Envelope{Resource: b.Resource},
				events.IntegrityVerifiedEvent{CRC: readCRC}))
			continue
		}

		if p.log != nil {
			p.log.Warn("chunk divergence",
				filament.Field{Key: "resource", Value: b.Resource},
				filament.Field{Key: "seq", Value: b.Seq},
				filament.Field{Key: "read_crc", Value: readCRC},
				filament.Field{Key: "write_crc", Value: receipt.WriteCRC},
			)
		}
		p.publish(events.NewFact(events.ChunkDivergence, events.Envelope{Resource: b.Resource},
			events.ChunkDivergenceEvent{CRC: readCRC}))
		p.setErr(fmt.Errorf("write %s seq %d: CRC divergence: read %08x, write %08x", b.Resource, b.Seq, readCRC, receipt.WriteCRC))
		return
	}
}

func (p *Pipeline) writeBatch(ctx context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	policy, err := p.policyFor(b.Resource)
	if err != nil {
		return filament.WriteReceipt{}, err
	}
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

func receiptCheckpoint(receipt filament.WriteReceipt, b filament.Batch) *filament.CheckpointData {
	if receipt.Checkpoint != nil {
		return receipt.Checkpoint
	}
	return b.Cursor
}
