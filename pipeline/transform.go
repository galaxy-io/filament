package pipeline

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/transform"
)

// planFor compiles the run's transform for one resource against the layout the
// builder emits, before audit fields are appended. A resource the definition
// does not name gets no plan and bypasses the transformer pool. Called with
// registryMu held.
func (p *Pipeline) planFor(resource string, supplied rowmodel.Schema) (*transform.Plan, error) {
	if p.transform == nil {
		return nil, nil
	}
	if _, ok := p.transform.Resources[resource]; !ok {
		return nil, nil
	}
	schema := supplied.Clone()
	schema.Resource = resource
	plan, err := transform.Compile(p.transform, schema)
	if err != nil {
		return nil, fmt.Errorf("pipeline: %w", err)
	}
	return plan, nil
}

// transformer drains the transform channel, applies each batch's plan, and
// queues the result for the writers. Markers pass through untouched, in order.
func (p *Pipeline) transformer(ctx context.Context) {
	defer p.twg.Done()

	for b := range p.transformCh {
		// Mirror the writer: once the pipeline has failed, queued batches are
		// drained and released, never transformed.
		if p.Err() != nil {
			b.Release()
			continue
		}
		p.transformBatch(ctx, b)
	}
}

func (p *Pipeline) transformBatch(ctx context.Context, b *arrowbatch.Batch) {
	// A kernel that panics fails the run, not the worker. Whichever batch is
	// current at that point is still owned here and gets its terminal Release.
	defer func() {
		if r := recover(); r != nil {
			b.Release()
			p.setErr(fmt.Errorf("transform %s seq %d panicked: %v\n%s", b.Resource, b.Seq, r, debug.Stack()))
		}
	}()
	if !b.Drained {
		p.registryMu.Lock()
		plan := p.schemas[b.Resource].plan
		p.registryMu.Unlock()
		started := time.Now()
		rows, err := plan.Apply(ctx, p.alloc, b.Rows())
		if err != nil {
			b.Release()
			p.setErr(fmt.Errorf("transform %s seq %d: %w", b.Resource, b.Seq, err))
			return
		}
		out := b.WithRows(rows)
		b.Release()
		b = out
		p.publish(events.NewFact(events.BatchTransformed, events.Envelope{Resource: b.Resource},
			events.BatchTransformedEvent{Records: int64(b.NumRows()), Bytes: b.Bytes(), Duration: time.Since(started)}))
	}
	if err := p.send(p.batchCh, b); err != nil {
		b.Release()
	}
}
