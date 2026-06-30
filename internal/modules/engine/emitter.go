package engine

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament"
)

// emitter is the per-run fact publisher. It owns the run's monotonic sequence
// (shared with the pipeline via next) and tallies run- and per-resource totals
// from the batch.written facts it sees, so it can stamp resource.completed and
// run.completed facts with authoritative counts.
type emitter struct {
	ctx    context.Context
	bus    eventbus.Bus
	log    ingestion.Logger
	tenant ingestion.TenantID
	run    ingestion.RunID

	seq atomic.Uint64

	mu         sync.Mutex
	runRecords int64
	runBytes   int64
	res        map[string]*tally
}

type tally struct {
	records int64
	bytes   int64
}

func newEmitter(ctx context.Context, bus eventbus.Bus, log ingestion.Logger, tenant ingestion.TenantID, run ingestion.RunID) *emitter {
	return &emitter{ctx: ctx, bus: bus, log: log, tenant: tenant, run: run, res: map[string]*tally{}}
}

// next returns the run's next fact sequence. Shared with the pipeline so engine
// lifecycle facts and pipeline facts never collide on (tenant, run, seq).
func (e *emitter) next() uint64 { return e.seq.Add(1) }

// publish ships an already-stamped fact (the pipeline stamps its own) and tallies
// run- and resource-level totals from batch writes. Publishing uses the run
// context; a fact emitted after that context is cancelled (e.g. host shutdown) is
// dropped. Safe to call concurrently — the pipeline's writer goroutine publishes
// batch facts while the engine goroutine publishes lifecycle facts.
func (e *emitter) publish(ev ingestion.Event) {
	if ev.Type == ingestion.EvBatchWritten {
		e.mu.Lock()
		e.runRecords += ev.Fields.Records
		e.runBytes += ev.Fields.Bytes
		t := e.res[ev.Resource]
		if t == nil {
			t = &tally{}
			e.res[ev.Resource] = t
		}
		t.records += ev.Fields.Records
		t.bytes += ev.Fields.Bytes
		e.mu.Unlock()
	}
	if err := ingestion.PublishEvent(e.ctx, e.bus, ev); err != nil && e.log != nil {
		e.log.Error("engine: publish fact", err, ingestion.Field{Key: "type", Value: ev.Type.String()})
	}
}

// emitFact stamps and publishes an engine-originated fact for a run or resource.
func (e *emitter) emitFact(t ingestion.EventType, resource string, f ingestion.EventFields) {
	e.publish(ingestion.Event{
		Type:     t,
		Tenant:   e.tenant,
		Run:      e.run,
		Resource: resource,
		Seq:      e.next(),
		At:       time.Now(),
		Fields:   f,
	})
}

// fact emits a run-level lifecycle fact.
func (e *emitter) fact(t ingestion.EventType, f ingestion.EventFields) { e.emitFact(t, "", f) }

// fail publishes the terminal run.failed fact carrying the error message.
func (e *emitter) fail(err error) {
	if e.log != nil {
		e.log.Error("engine: run failed", err, ingestion.Field{Key: "run", Value: string(e.run)})
	}
	e.fact(ingestion.EvRunFailed, ingestion.EventFields{Error: err.Error()})
}

func (e *emitter) partial(err error) {
	if e.log != nil {
		e.log.Error("engine: run partial", err, ingestion.Field{Key: "run", Value: string(e.run)})
	}
	e.fact(ingestion.EvRunPartial, ingestion.EventFields{Error: err.Error()})
}

// resourceTally returns the accumulated counts for one resource.
func (e *emitter) resourceTally(name string) (records, bytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if t := e.res[name]; t != nil {
		return t.records, t.bytes
	}
	return 0, 0
}

// runTotals returns the run's accumulated counts.
func (e *emitter) runTotals() (records, bytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runRecords, e.runBytes
}

// seenResources returns, sorted, the resources that produced at least one batch.
func (e *emitter) seenResources() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]string, 0, len(e.res))
	for name := range e.res {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// resolveResources picks the resource set to report lifecycle facts for: the
// explicitly requested set when known, else the set that actually produced data.
func resolveResources(requested, seen []string) []string {
	if len(requested) > 0 {
		return requested
	}
	return seen
}
