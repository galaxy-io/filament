package pipeline

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/transform"
)

const (
	// defaultBatchMaxBytes is the Arrow bytes one alchemy call carries before its pack flushes.
	defaultBatchMaxBytes = 16 << 20
	// defaultMaxInFlight is how many packs run at once.
	defaultMaxInFlight = 4
)

// pack is consecutive batches of one resource transformed by a single plan application.
type pack struct {
	items  []queuedBatch
	schema *arrow.Schema
	bytes  int64
	opened time.Time
}

// packJob is one unit handed to the writers in submission order; out is valid once done closes.
type packJob struct {
	items []queuedBatch
	plan  *transform.Plan
	done  chan struct{}
	out   []queuedBatch
}

// packer replaces the transformer pool when a plan calls alchemy: it packs batches per resource, runs packs
// concurrently, and emits them in order, so nothing after a failed pack is ever written.
func (p *Pipeline) packer(ctx context.Context) {
	defer p.twg.Done()
	jobs := make(chan *packJob, p.maxInFlight)
	sem := make(chan struct{}, p.maxInFlight)
	emitted := make(chan struct{})
	go p.emitPacks(jobs, emitted)

	packs := map[string]*pack{}
	submit := func(items []queuedBatch, plan *transform.Plan) {
		j := &packJob{items: items, plan: plan, done: make(chan struct{})}
		select {
		case jobs <- j:
		case <-p.done:
			releaseItems(items)
			return
		}
		sem <- struct{}{}
		go func() {
			defer func() { <-sem }()
			p.runPack(ctx, j)
		}()
	}
	flush := func(resource string) {
		pk, ok := packs[resource]
		if !ok {
			return
		}
		delete(packs, resource)
		submit(pk.items, p.planOf(resource))
	}
	flushAll := func(olderThan time.Time) {
		for resource, pk := range packs {
			if olderThan.IsZero() || pk.opened.Before(olderThan) {
				flush(resource)
			}
		}
	}

	tick := time.NewTicker(p.flushIvl)
	defer tick.Stop()
	for {
		select {
		case item, ok := <-p.transformCh:
			if !ok {
				flushAll(time.Time{})
				close(jobs)
				<-emitted
				return
			}
			if p.Err() != nil {
				item.batch.Release()
				continue
			}
			b := item.batch
			plan := p.planOf(b.Resource)
			switch {
			case b.Control != nil:
				flushAll(time.Time{})
				submit([]queuedBatch{item}, nil)
			case b.Drained || plan == nil || !plan.Invokes():
				flush(b.Resource)
				submit([]queuedBatch{item}, plan)
			default:
				pk := packs[b.Resource]
				if pk != nil && (!pk.schema.Equal(b.Rows().Schema()) || pk.bytes+b.Bytes() > p.batchMaxBytes) {
					flush(b.Resource)
					pk = nil
				}
				if pk == nil {
					pk = &pack{schema: b.Rows().Schema(), opened: time.Now()}
					packs[b.Resource] = pk
				}
				pk.items = append(pk.items, item)
				pk.bytes += b.Bytes()
			}
		case now := <-tick.C:
			flushAll(now.Add(-p.flushIvl))
		}
	}
}

func (p *Pipeline) planOf(resource string) *transform.Plan {
	p.registryMu.Lock()
	defer p.registryMu.Unlock()
	return p.schemas[resource].plan
}

// emitPacks hands finished jobs to the writers in submission order, releasing everything once the run has failed.
func (p *Pipeline) emitPacks(jobs <-chan *packJob, emitted chan<- struct{}) {
	defer close(emitted)
	for j := range jobs {
		<-j.done
		for _, item := range j.out {
			if p.Err() != nil {
				item.batch.Release()
				continue
			}
			if err := p.send(p.batchCh, item); err != nil {
				item.batch.Release()
			}
		}
	}
}

// runPack applies the plan over the pack's rows and splits the result back into its batches.
func (p *Pipeline) runPack(ctx context.Context, j *packJob) {
	defer close(j.done)
	owned := j.items
	var in, rows arrow.RecordBatch
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		releaseItems(owned)
		for _, rec := range []arrow.RecordBatch{in, rows} {
			if rec != nil {
				rec.Release()
			}
		}
		j.out = nil
		p.setErr(fmt.Errorf("transform %s panicked: %v\n%s", j.items[0].batch.Resource, r, debug.Stack()))
	}()
	if p.Err() != nil {
		releaseItems(owned)
		return
	}
	first := j.items[0].batch
	if j.plan == nil || first.Drained || first.Control != nil {
		j.out = j.items
		return
	}
	started := time.Now()
	var err error
	if in, err = p.concatRecords(batchRows(j.items)); err != nil {
		releaseItems(owned)
		p.setErr(fmt.Errorf("transform %s seq %d: pack: %w", first.Resource, first.Seq, err))
		return
	}
	rows, err = j.plan.Apply(ctx, p.alloc, in)
	in.Release()
	in = nil
	if err != nil {
		releaseItems(owned)
		p.report(Failure{Resource: first.Resource, Scope: ScopeResource, Err: fmt.Errorf("transform %s seq %d-%d: %w", first.Resource, first.Seq, j.items[len(j.items)-1].batch.Seq, err)})
		return
	}
	elapsed := time.Since(started)
	var offset int64
	out := make([]queuedBatch, len(j.items))
	for i, item := range j.items {
		n := int64(item.batch.NumRows())
		out[i] = item
		out[i].batch = item.batch.WithRows(rows.NewSlice(offset, offset+n))
		offset += n
	}
	releaseItems(j.items)
	owned = out
	rows.Release()
	rows = nil
	total := max(offset, 1)
	for _, item := range out {
		n := int64(item.batch.NumRows())
		p.publish(events.NewFact(events.BatchTransformed, events.Envelope{Resource: item.batch.Resource},
			events.BatchTransformedEvent{Records: n, Bytes: item.batch.Bytes(), Duration: elapsed * time.Duration(n) / time.Duration(total)}))
	}
	j.out = out
}

func batchRows(items []queuedBatch) []arrow.RecordBatch {
	recs := make([]arrow.RecordBatch, len(items))
	for i, item := range items {
		recs[i] = item.batch.Rows()
	}
	return recs
}

// concatRecords joins records column by column into a new record; a single record is retained, not copied.
func (p *Pipeline) concatRecords(recs []arrow.RecordBatch) (arrow.RecordBatch, error) {
	first := recs[0]
	if len(recs) == 1 {
		first.Retain()
		return first, nil
	}
	mem := p.alloc
	if mem == nil {
		mem = memory.DefaultAllocator
	}
	cols := make([]arrow.Array, first.NumCols())
	var rows int64
	for _, rec := range recs {
		rows += rec.NumRows()
	}
	for c := range cols {
		parts := make([]arrow.Array, len(recs))
		for i, rec := range recs {
			parts[i] = rec.Column(c)
		}
		col, err := array.Concatenate(parts, mem)
		if err != nil {
			for _, done := range cols[:c] {
				done.Release()
			}
			return nil, err
		}
		cols[c] = col
	}
	rec := array.NewRecordBatch(first.Schema(), cols, rows)
	for _, col := range cols {
		col.Release()
	}
	return rec, nil
}

func releaseItems(items []queuedBatch) {
	for _, item := range items {
		item.batch.Release()
	}
}
