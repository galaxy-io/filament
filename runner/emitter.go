package runner

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
)

// emitter is the per-run fact publisher. It owns the run's monotonic sequence
// (shared with the pipeline via next) and tallies run- and per-resource totals
// from the batch.written facts it sees, so it can stamp resource.completed and
// run.completed facts with authoritative counts.
type emitter struct {
	ctx    context.Context
	bus    eventbus.Bus
	log    filament.Logger
	span   filament.Span
	tenant filament.TenantID
	run    filament.RunID

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

func newEmitter(ctx context.Context, bus eventbus.Bus, log filament.Logger, tenant filament.TenantID, run filament.RunID) *emitter {
	return &emitter{ctx: ctx, bus: bus, log: log, tenant: tenant, run: run, res: map[string]*tally{}}
}

// next returns the run's next fact sequence. Shared with the pipeline so run
// lifecycle facts and pipeline facts never collide on (tenant, run, seq).
func (e *emitter) next() uint64 { return e.seq.Add(1) }

// terminalPublishWait bounds how long ending facts may publish after the run
// context is cancelled, so shutdown cannot hang on a dead bus.
const terminalPublishWait = 5 * time.Second

// finish detaches the emitter from run cancellation so ending facts — resource
// terminals and the run obituary — publish even while the host is shutting
// down. Call once extraction has stopped (no concurrent pipeline publishes);
// the returned cancel releases the grace timer.
func (e *emitter) finish() context.CancelFunc {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(e.ctx), terminalPublishWait)
	e.ctx = ctx
	return cancel
}

// publish ships an already-stamped fact (the pipeline stamps its own) and tallies
// run- and resource-level totals from batch writes. Publishing uses the run
// context, so mid-run facts stop when the run is cancelled; ending facts survive
// cancellation because RunOne detaches the emitter first (see finish). Safe to
// call concurrently — the pipeline's writer goroutine publishes batch facts
// while the run goroutine publishes lifecycle facts.
func (e *emitter) publish(f events.Fact) {
	if d, ok := f.Data.(events.BatchWrittenEvent); ok {
		e.mu.Lock()
		e.runRecords += d.Records
		e.runBytes += d.Bytes
		t := e.res[f.Resource]
		if t == nil {
			t = &tally{}
			e.res[f.Resource] = t
		}
		t.records += d.Records
		t.bytes += d.Bytes
		e.mu.Unlock()
	}
	if e.log != nil {
		records, bytes, errMsg := factProgress(f)
		fields := []filament.Field{
			{Key: "run", Value: string(f.Run)},
			{Key: "resource", Value: f.Resource},
			{Key: "records", Value: records},
			{Key: "bytes", Value: bytes},
		}
		if errMsg != "" {
			fields = append(fields, filament.Field{Key: "error", Value: errMsg})
		}
		switch f.Data.(type) {
		case events.BatchBufferedEvent, events.BatchWrittenEvent, events.IntegrityVerifiedEvent:
			// Debug, not Info: a large run emits one of these per chunk, which
			// at Info drowns the worker's log.
			e.log.Debug(f.Name, fields...)
		default:
			e.log.Info(f.Name, fields...)
		}
	}
	if err := events.Publish(e.ctx, e.bus, f); err != nil && e.log != nil {
		e.log.Error("runner: publish fact", err, filament.Field{Key: "type", Value: f.Name})
	}
}

// factProgress flattens a typed payload's progress counters and error for logging.
func factProgress(f events.Fact) (records, bytes int64, errMsg string) {
	switch d := f.Data.(type) {
	case events.BatchBufferedEvent:
		return d.Records, d.Bytes, ""
	case events.BatchWrittenEvent:
		return d.Records, d.Bytes, ""
	case events.ResourceCompletedEvent:
		return d.Records, d.Bytes, ""
	case events.RunCompletedEvent:
		return d.Records, d.Bytes, ""
	case events.ResourceFailedEvent:
		return 0, 0, d.Error
	case events.RunFailedEvent:
		return 0, 0, d.Error
	case events.RunPartialEvent:
		return 0, 0, d.Error
	default:
		return 0, 0, ""
	}
}

// emit stamps and publishes a run-originated fact for a run or resource.
// (A free function: Go methods cannot take type parameters.)
func emit[T any](e *emitter, t events.EventType[T], resource string, data T) {
	e.publish(events.NewFact(t, events.Envelope{
		Tenant:   e.tenant,
		Run:      e.run,
		Resource: resource,
		Seq:      e.next(),
		At:       time.Now(),
	}, data))
}

// failed publishes the run's failure terminals — per-resource obituaries, then
// run.partial (resumable) or run.failed — on a context detached from run
// cancellation, so the obituary outlives the death and the row never strands
// in RunRunning. The emitter owns the detach: callers never sequence it.
func (e *emitter) failed(err error, resources []string, resumable bool) {
	defer e.finish()()
	for _, res := range resources {
		emit(e, events.ResourceFailed, res, events.ResourceFailedEvent{Error: err.Error()})
	}
	if resumable {
		e.partial(err)
		return
	}
	e.fail(err)
}

// completed publishes the run's success terminals — per-resource completions
// with their tallies, then run.completed with the run totals — detached from
// run cancellation. Call only after the sink commit has returned, so commit
// time is charged to the run rather than the terminal-publish window.
func (e *emitter) completed(resources []string) {
	defer e.finish()()
	for _, res := range resources {
		records, bytes := e.resourceTally(res)
		emit(e, events.ResourceCompleted, res, events.ResourceCompletedEvent{Records: records, Bytes: bytes})
	}
	records, bytes := e.runTotals()
	emit(e, events.RunCompleted, "", events.RunCompletedEvent{Records: records, Bytes: bytes})
}

func (e *emitter) fail(err error) {
	if e.span != nil {
		e.span.SetError(err)
	}
	if e.log != nil {
		e.log.Error("runner: run failed", err, filament.Field{Key: "run", Value: string(e.run)})
	}
	emit(e, events.RunFailed, "", events.RunFailedEvent{Error: err.Error()})
}

func (e *emitter) partial(err error) {
	if e.span != nil {
		e.span.SetError(err)
	}
	if e.log != nil {
		e.log.Error("runner: run partial", err, filament.Field{Key: "run", Value: string(e.run)})
	}
	emit(e, events.RunPartial, "", events.RunPartialEvent{Error: err.Error()})
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
