package pipeline

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/telemetry"
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
	// The source has already narrowed supplied to the selection.
	schema := rowmodel.InputFor(supplied, nil, p.audit != nil && p.audit.CDC)
	schema.Resource = resource
	plan, err := transform.Compile(p.transform, schema,
		transform.WithContracts(p.contracts), transform.WithInvoker(p.invoker(resource)))
	if err != nil {
		return nil, fmt.Errorf("pipeline: %w", err)
	}
	return plan, nil
}

// transformer drains the transform channel, applies each batch's plan, and
// queues the result for the writers. Markers and controls pass through
// untouched, in order.
func (p *Pipeline) transformer(ctx context.Context) {
	defer p.twg.Done()

	for item := range p.transformCh {
		// Mirror the writer: once the pipeline has failed, queued batches are
		// drained and released, never transformed.
		if p.Err() != nil || item.gate.quarantined() {
			p.reject(item.batch, p.dropCause(item.gate))
			item.batch.Release()
			continue
		}
		p.transformBatch(ctx, item)
	}
}

func (p *Pipeline) transformBatch(ctx context.Context, item queuedBatch) {
	b := item.batch
	// A kernel that panics fails the run, not the worker. Whichever batch is
	// current at that point is still owned here and gets its terminal Release.
	defer func() {
		if r := recover(); r != nil {
			f := Failure{Resource: b.Resource, Scope: ScopeSession, Err: transformError(fmt.Errorf("transform %s seq %d panicked: %v\n%s", b.Resource, b.Seq, r, debug.Stack()))}
			p.reject(b, f.Err)
			b.Release()
			p.report(f)
		}
	}()
	if !b.Drained && b.Control == nil {
		p.registryMu.Lock()
		plan := p.schemas[b.Resource].plan
		p.registryMu.Unlock()
		if plan != nil {
			started := time.Now()
			rows, err := plan.Apply(ctx, p.alloc, b.Rows())
			if err != nil {
				f := Failure{Resource: b.Resource, Scope: ScopeResource, Err: transformError(fmt.Errorf("transform %s seq %d: %w", b.Resource, b.Seq, err))}
				p.reject(b, f.Err)
				b.Release()
				p.report(f)
				return
			}
			out := b.WithRows(rows)
			b.Release()
			b = out
			item.batch = b
			p.publish(events.NewFact(events.BatchTransformed, events.Envelope{Resource: b.Resource},
				events.BatchTransformedEvent{Records: int64(b.NumRows()), Bytes: b.Bytes(), Duration: time.Since(started)}))
		}
	}
	if err := p.send(p.batchCh, item); err != nil {
		p.reject(b, err)
		b.Release()
	}
}

// transformError marks a failure of the transform stage.
func transformError(err error) error {
	return telemetry.WithErrorType(telemetry.ErrorTypeTransform, err)
}
