package tracker

import (
	"context"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// totalsFlushInterval bounds how stale a running run's persisted totals may be.
// batch.written facts fold into memory and reach the store at most this often,
// instead of one load and save of the whole run per batch.
const totalsFlushInterval = time.Second

// folds reports whether apply changes state for a fact. The tracker drops any
// other fact before touching the store, so per-batch observability facts
// (buffered, integrity verified) cost nothing.
func folds(data any) bool {
	switch data.(type) {
	case events.RunStartedEvent,
		events.RunCompletedEvent, events.RunFailedEvent, events.RunPartialEvent,
		events.RunCanceledEvent, events.RunPausedEvent,
		events.ResourceStartedEvent, events.ResourceCompletedEvent, events.ResourceFailedEvent,
		events.BatchWrittenEvent, events.HeartbeatEvent, events.CheckpointSavedEvent:
		return true
	}
	return false
}

// runFold is the tracker's in-memory view of one bounded run: the dedup
// high-water mark it has applied, and the batch totals not yet persisted.
type runFold struct {
	hw      uint64
	hwKnown bool

	flushedAt time.Time // zero until the run's first flush
	pending   batchTotals
}

// batchTotals are summed batch.written facts not yet saved to the run.
type batchTotals struct {
	env       events.Envelope // the latest fact's envelope, for the save
	seq       uint64          // stream sequence of the latest fact
	records   int64
	bytes     int64
	resources map[string]*resourceTotals
	order     []string // resources in first-seen order, so saves are deterministic
}

type resourceTotals struct{ records, bytes int64 }

func (b *batchTotals) empty() bool { return b.seq == 0 }

// plus returns b with one more batch, leaving b untouched so a failed flush
// can discard the new batch alone.
func (b batchTotals) plus(env events.Envelope, seq uint64, d events.BatchWrittenEvent) batchTotals {
	out := batchTotals{
		env: env, seq: seq,
		records: b.records + d.Records, bytes: b.bytes + d.Bytes,
		resources: make(map[string]*resourceTotals, len(b.resources)+1),
		order:     append([]string(nil), b.order...),
	}
	for name, t := range b.resources {
		c := *t
		out.resources[name] = &c
	}
	t := out.resources[env.Resource]
	if t == nil {
		t = &resourceTotals{}
		out.resources[env.Resource] = t
		out.order = append(out.order, env.Resource)
	}
	t.records += d.Records
	t.bytes += d.Bytes
	return out
}

func (m *Module) fold(key runKey) *runFold {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.folds == nil {
		m.folds = make(map[runKey]*runFold)
	}
	f := m.folds[key]
	if f == nil {
		f = &runFold{}
		m.folds[key] = f
	}
	return f
}

// highWater returns the highest stream sequence applied for the run, reading
// the store only on the run's first fact in this process.
func (m *Module) highWater(ctx context.Context, tenant filament.TenantID, run filament.RunID) (uint64, error) {
	f := m.fold(runKey{tenant, run})
	if f.hwKnown {
		return f.hw, nil
	}
	hw, _, err := m.ds.DedupHighWater(ctx, string(tenant), run)
	if err != nil {
		return 0, err
	}
	f.hw, f.hwKnown = max(f.hw, hw), true
	return f.hw, nil
}

// advance records seq as applied, in memory first so a failed store write
// cannot get the fact applied twice on redelivery.
// It never creates the run's entry, so a terminal fold that evicted the run
// does not leave one behind.
func (m *Module) advance(ctx context.Context, tenant filament.TenantID, run filament.RunID, seq uint64) (bool, error) {
	m.mu.Lock()
	if f := m.folds[runKey{tenant, run}]; f != nil {
		f.hw = max(f.hw, seq)
	}
	m.mu.Unlock()
	return m.ds.DedupSeen(ctx, string(tenant), run, seq)
}

// foldBatch adds a batch.written fact to the run's pending totals and saves
// them when they are due: on the run's first batch, so progress shows at once,
// then at most every totalsFlushInterval. Cursor folding stays per fact.
//
// Pending totals are progress only. They live in memory until the next flush,
// so a tracker restart can drop them; the run's terminal fact carries the
// engine's totals and corrects the stored ones.
func (m *Module) foldBatch(ctx context.Context, env events.Envelope, seq uint64, d events.BatchWrittenEvent) error {
	f := m.fold(runKey{env.Tenant, env.Run})
	next := f.pending.plus(env, seq, d)
	var dedupErr error
	if f.flushedAt.IsZero() || m.clock().Sub(f.flushedAt) >= totalsFlushInterval {
		if err := m.saveTotals(ctx, f, next); err != nil {
			return err
		}
		// The totals are stored and the in-memory mark covers this fact, so a
		// redelivery skips it. Finish its cursor before reporting a failed
		// dedup write, or the redelivery would drop the cursor too.
		_, dedupErr = m.advance(ctx, env.Tenant, env.Run, seq)
	} else {
		f.pending = next
		f.hw = max(f.hw, seq)
	}

	m.rememberBoundary(env, d.CheckpointPolicy)
	if cp, persist := m.foldCursor(ctx, env, d.Checkpoint); cp != nil && persist {
		// Totals reach the store before the cursor that covers them.
		if err := m.flushTotals(ctx, env.Tenant, env.Run); err != nil {
			return err
		}
		if err := m.saveCheckpoint(ctx, env.Tenant, env.Run, cp); err != nil {
			m.observeCheckpointFailure(ctx)
			if m.log != nil {
				m.log.Error("checkpoint persistence failed", err,
					filament.Field{Key: "event.name", Value: "tracker.checkpoint.persist_failed"},
					filament.Field{Key: "run_id", Value: string(env.Run)})
			}
		}
	}
	return dedupErr
}

// flushTotals saves the run's pending batch totals, if any. Every fact other
// than batch.written calls it first, so each fold sees current totals.
func (m *Module) flushTotals(ctx context.Context, tenant filament.TenantID, run filament.RunID) error {
	m.mu.Lock()
	f := m.folds[runKey{tenant, run}]
	m.mu.Unlock()
	if f == nil || f.pending.empty() {
		return nil
	}
	t := f.pending
	if err := m.saveTotals(ctx, f, t); err != nil {
		return err
	}
	_, err := m.advance(ctx, tenant, run, t.seq)
	return err
}

// saveTotals adds t to the stored run in one load and save. On a failed save
// the pending totals are left as they were. The caller then advances the
// dedup mark past every fact t covers.
func (m *Module) saveTotals(ctx context.Context, f *runFold, t batchTotals) error {
	var pipeline string
	if err := m.mutate(ctx, t.env, func(r *filament.RunState) {
		r.Records += t.records
		r.Bytes += t.bytes
		for _, name := range t.order {
			rt := t.resources[name]
			rs := resourceRef(r, name)
			rs.Records += rt.records
			rs.Bytes += rt.bytes
			if rs.Status == filament.RunRequested {
				rs.Status = filament.RunRunning
			}
		}
		pipeline = r.Request.PipelineID
	}); err != nil {
		return err
	}
	f.pending = batchTotals{}
	f.flushedAt = m.clock()
	for _, name := range t.order {
		rt := t.resources[name]
		m.observeBatch(ctx, events.Envelope{Resource: name}, pipeline, rt.records, rt.bytes)
	}
	return nil
}

func (m *Module) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

// raiseTotals lifts the stored run totals to a terminal fact's. Batch totals
// still pending when a tracker restarted are lost, so the stored ones can
// trail the engine's; a fact from another process carries zero and changes
// nothing.
func raiseTotals(r *filament.RunState, records, bytes int64) {
	r.Records = max(r.Records, records)
	r.Bytes = max(r.Bytes, bytes)
}
