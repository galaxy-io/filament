// Package tracker folds bus facts into DataStore run/resource/checkpoint state.
package tracker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/module"
)

// Module consumes every ingestion fact and persists the derived run state.
type Module struct {
	ds  ingestion.DataStore
	log ingestion.Logger

	// Resumable-checkpoint accumulators, keyed by (run, resource). A single durable
	// consumer folds facts serially, but the maps are mutex-guarded in case the host
	// delivers concurrently. cp holds the live merged cursor; since counts written
	// batches toward the per-run persist cadence.
	mu    sync.Mutex
	cp    map[ckKey]ingestion.Checkpoint
	since map[ckKey]int
	every map[ingestion.RunID]int // cached per-run CheckpointEvery cadence
	bm    map[ckKey]*bmAccount    // bitmap per-shard ack/want counters
}

// bmAccount counts written rows per bitmap shard against the shard's expected total. A
// shard is marked Done (in the checkpoint) only once acked reaches want — order-independent
// of when the want marker vs the acks arrive, so it is safe under parallel writers.
type bmAccount struct {
	acked   map[int]int
	want    map[int]int
	wantSet map[int]bool
}

func newBmAccount() *bmAccount {
	return &bmAccount{acked: map[int]int{}, want: map[int]int{}, wantSet: map[int]bool{}}
}

type ckKey struct {
	run      ingestion.RunID
	resource string
}

// New returns an unmounted tracker. Providers are injected by Mount.
func New() *Module {
	return &Module{cp: map[ckKey]ingestion.Checkpoint{}, since: map[ckKey]int{}, every: map[ingestion.RunID]int{}, bm: map[ckKey]*bmAccount{}}
}

// compile-time check that we satisfy the Module contract.
var _ module.Module = (*Module)(nil)

func (m *Module) Name() string { return "tracker" }

// Subscriptions: one durable consumer over the whole versioned namespace, so the
// tracker sees every fact and survives restarts (resuming where it left off).
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{
		{Pattern: "ingestion.v1.>", Durable: "tracker", Handler: m.onFact},
	}
}

// Mount captures the providers this module uses. Cheap, no I/O.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.ds = d.DataStore
	m.log = d.Log
	return nil
}

// onFact de-dups then folds the fact into persisted state. Returning nil acks the
// message; returning an error naks it for redelivery.
//
// Dedup keys on the bus stream sequence (msg.Seq), not the event's embedded Seq:
// the stream sequence is broker-assigned and unique per message, so facts from
// different producers for the same run (e.g. the orchestrator's run.requested and
// the engine's run.started) never collide, and a redelivered message keeps its
// sequence so the fold stays idempotent.
func (m *Module) onFact(ctx context.Context, msg eventbus.Message) error {
	ev, err := ingestion.EventOf(msg)
	if err != nil {
		return err
	}

	seen, err := m.ds.DedupSeen(ctx, string(ev.Tenant), ev.Run, msg.Seq())
	if err != nil {
		return err
	}
	if seen {
		return nil // already applied — idempotent no-op
	}

	return m.apply(ctx, ev)
}

// apply folds one fact into persisted run/resource/checkpoint state. Facts not
// listed here (heartbeats, page-fetched, batch-buffered, integrity-verified,
// rate-limited, schedule-fired) carry no durable state change and are ignored.
func (m *Module) apply(ctx context.Context, ev ingestion.Event) error {
	switch ev.Type {
	case ingestion.EvRunStarted:
		return m.mutate(ctx, ev, func(r *ingestion.RunState) {
			r.Status = ingestion.RunRunning
			if r.StartedAt.IsZero() {
				r.StartedAt = ev.At
			}
		})

	case ingestion.EvRunCompleted:
		if err := m.mutate(ctx, ev, func(r *ingestion.RunState) {
			r.Status = ingestion.RunCompleted
			finishedAt(r, ev.At)
			// The terminal fact carries the engine's authoritative totals.
			r.Records = ev.Fields.Records
			r.Bytes = ev.Fields.Bytes
		}); err != nil {
			return err
		}
		m.flushRun(ctx, ev.Run)
		return nil

	case ingestion.EvRunFailed:
		if err := m.mutate(ctx, ev, func(r *ingestion.RunState) {
			r.Status = ingestion.RunFailed
			finishedAt(r, ev.At)
			r.Error = ev.Fields.Error
		}); err != nil {
			return err
		}
		m.flushRun(ctx, ev.Run)
		return nil

	case ingestion.EvRunPartial:
		if err := m.mutate(ctx, ev, func(r *ingestion.RunState) {
			r.Status = ingestion.RunPartial
			finishedAt(r, ev.At)
			r.Error = ev.Fields.Error
		}); err != nil {
			return err
		}
		m.flushRun(ctx, ev.Run)
		return nil

	case ingestion.EvResourceStarted:
		return m.mutate(ctx, ev, func(r *ingestion.RunState) {
			rs := resourceRef(r, ev.Resource)
			rs.Enabled = true
			if rs.Status == ingestion.RunRequested {
				rs.Status = ingestion.RunRunning
			}
		})

	case ingestion.EvResourceCompleted:
		if err := m.mutate(ctx, ev, func(r *ingestion.RunState) {
			rs := resourceRef(r, ev.Resource)
			rs.Status = ingestion.RunCompleted
			rs.Records = ev.Fields.Records
			rs.Bytes = ev.Fields.Bytes
		}); err != nil {
			return err
		}
		m.flushResource(ctx, ev.Run, ev.Resource)
		return nil

	case ingestion.EvResourceFailed:
		return m.mutate(ctx, ev, func(r *ingestion.RunState) {
			rs := resourceRef(r, ev.Resource)
			rs.Status = ingestion.RunFailed
			rs.Error = ev.Fields.Error
		})

	case ingestion.EvBatchWritten:
		// Incremental progress: accumulate per-resource and run totals as chunks
		// land, so observers see counts climb before the run finishes.
		if err := m.mutate(ctx, ev, func(r *ingestion.RunState) {
			r.Records += ev.Fields.Records
			r.Bytes += ev.Fields.Bytes
			rs := resourceRef(r, ev.Resource)
			rs.Records += ev.Fields.Records
			rs.Bytes += ev.Fields.Bytes
			if rs.Status == ingestion.RunRequested {
				rs.Status = ingestion.RunRunning
			}
		}); err != nil {
			return err
		}
		if cp, persist := m.foldCursor(ctx, ev); cp != nil && persist {
			if err := m.ds.SaveCheckpoint(ctx, ev.Run, cp); err != nil && m.log != nil {
				m.log.Error("tracker: save checkpoint", err, ingestion.Field{Key: "run", Value: string(ev.Run)})
			}
		}
		return nil

	case ingestion.EvCheckpointSaved, ingestion.EvWatermarkAdvanced:
		if ev.Fields.Checkpoint == nil {
			return nil
		}
		if err := m.ds.SaveCheckpoint(ctx, ev.Run, ev.Fields.Checkpoint); err != nil {
			return err
		}
		return m.mutate(ctx, ev, func(r *ingestion.RunState) {
			resourceRef(r, ev.Resource).Checkpoint = ev.Fields.Checkpoint
		})

	default:
		return nil // facts this module doesn't fold are acked and ignored
	}
}

// foldCursor merges a batch.written keyset delta into the resource's accumulated
// checkpoint and reports whether the run's persist cadence is due. Returns (nil,false)
// for a non-keyset batch or before the shard layout (the plan) has been seeded.
func (m *Module) foldCursor(ctx context.Context, ev ingestion.Event) (ingestion.Checkpoint, bool) {
	if ev.Fields.Checkpoint == nil {
		return nil, false
	}
	if part, ack, want, hasWant, ok := checkpoint.CoarseDelta(ev.Fields.Checkpoint); ok {
		return m.foldBitmap(ctx, ev, part, ack, want, hasWant)
	}
	if _, _, ok := checkpoint.ParseStream(ev.Fields.Checkpoint); ok {
		return m.foldStream(ctx, ev)
	}
	key := ckKey{ev.Run, ev.Resource}

	m.mu.Lock()
	base, ok := m.cp[key]
	if !ok {
		base = m.loadCheckpoint(ctx, ev.Run, ev.Resource) // recover layout after a restart
		m.cp[key] = base
	}
	merged := checkpoint.MergeShardDelta(base, ev.Fields.Checkpoint)
	if merged == nil {
		m.mu.Unlock()
		return nil, false
	}
	m.cp[key] = merged
	m.since[key]++
	persist := m.since[key] >= m.cadence(ctx, ev.Run)
	if persist {
		m.since[key] = 0
	}
	m.mu.Unlock()
	return merged, persist
}

// foldStream folds a change-stream position delta: the newer position (guarded by the
// per-run seq, since concurrent writers can publish batch facts out of order) replaces
// the resource's cursor wholesale — a stream cursor has no shard layout to merge into.
func (m *Module) foldStream(ctx context.Context, ev ingestion.Event) (ingestion.Checkpoint, bool) {
	key := ckKey{ev.Run, ev.Resource}

	m.mu.Lock()
	base, ok := m.cp[key]
	if !ok {
		base = m.loadCheckpoint(ctx, ev.Run, ev.Resource) // recover position after a restart
		m.cp[key] = base
	}
	merged := checkpoint.MergeStream(base, ev.Fields.Checkpoint)
	if merged == nil {
		m.mu.Unlock()
		return nil, false
	}
	m.cp[key] = merged
	m.since[key]++
	persist := m.since[key] >= m.cadence(ctx, ev.Run)
	if persist {
		m.since[key] = 0
	}
	m.mu.Unlock()
	return merged, persist
}

// foldBitmap accumulates a bitmap shard's write acks against its expected total and, when
// the part is fully written, flips its Done flag in the resource checkpoint and persists.
// Returns (nil, false) for a plain ack or before completion. Order-independent of whether
// the want marker or the acks land first, so it is correct under parallel writers.
func (m *Module) foldBitmap(ctx context.Context, ev ingestion.Event, part, ack, want int, hasWant bool) (ingestion.Checkpoint, bool) {
	key := ckKey{ev.Run, ev.Resource}

	m.mu.Lock()
	defer m.mu.Unlock()

	base, ok := m.cp[key]
	if !ok {
		base = m.loadCheckpoint(ctx, ev.Run, ev.Resource) // recover layout after a restart
		m.cp[key] = base
	}
	acct, ok := m.bm[key]
	if !ok {
		acct = newBmAccount()
		m.bm[key] = acct
	}
	acct.acked[part] += ack
	if hasWant {
		acct.want[part] = want
		acct.wantSet[part] = true
	}
	if !acct.wantSet[part] || acct.acked[part] < acct.want[part] {
		return nil, false // shard not fully written yet
	}

	ks, ok := checkpoint.ParseKeyset(base)
	if !ok || part < 0 || part >= len(ks.Shards) || ks.Shards[part].Done {
		return nil, false // layout not seeded yet, or already complete
	}
	ks.Shards[part].Done = true
	merged := ks.ToCheckpoint(base.Resource())
	m.cp[key] = merged
	return merged, true // a completed shard is durable progress — persist immediately
}

// flushResource persists the resource's latest accumulated cursor immediately,
// regardless of cadence (called on resource.completed and run termination).
func (m *Module) flushResource(ctx context.Context, run ingestion.RunID, resource string) {
	key := ckKey{run, resource}
	m.mu.Lock()
	cp := m.cp[key]
	m.since[key] = 0
	m.mu.Unlock()
	if cp != nil {
		if err := m.ds.SaveCheckpoint(ctx, run, cp); err != nil && m.log != nil {
			m.log.Error("tracker: flush checkpoint", err, ingestion.Field{Key: "run", Value: string(run)})
		}
	}
}

// flushRun persists every accumulated cursor for the run (called on terminal facts so
// a resumable failure leaves the freshest possible cursors on disk).
func (m *Module) flushRun(ctx context.Context, run ingestion.RunID) {
	m.mu.Lock()
	pending := make(map[string]ingestion.Checkpoint)
	for key, cp := range m.cp {
		if key.run == run && cp != nil {
			pending[key.resource] = cp
		}
		if key.run == run {
			m.since[key] = 0
		}
	}
	m.mu.Unlock()
	for _, cp := range pending {
		if err := m.ds.SaveCheckpoint(ctx, run, cp); err != nil && m.log != nil {
			m.log.Error("tracker: flush checkpoint", err, ingestion.Field{Key: "run", Value: string(run)})
		}
	}
}

// cadence is the run's CheckpointEvery (cached; default when unset).
func (m *Module) cadence(ctx context.Context, run ingestion.RunID) int {
	if n, ok := m.every[run]; ok {
		return n
	}
	n := ingestion.DefaultCheckpointEvery
	if r, err := m.ds.LoadRun(ctx, run); err == nil && r.Request.Options.CheckpointEvery > 0 {
		n = r.Request.Options.CheckpointEvery
	}
	m.every[run] = n
	return n
}

// loadCheckpoint returns the persisted cursor for (run, resource) or nil.
func (m *Module) loadCheckpoint(ctx context.Context, run ingestion.RunID, resource string) ingestion.Checkpoint {
	cp, err := m.ds.LoadCheckpoint(ctx, run, resource)
	if err != nil {
		return nil
	}
	return cp
}

// mutate loads the run, applies fn, and saves it back — the read-modify-write
// every fold shares. The run is created on first sight (a fact may arrive before
// any prior state exists). Loading the full state first preserves fields this
// fact doesn't touch (notably the original Request). The tracker's single pump
// goroutine serializes these, so no row is lost to a concurrent fold.
func (m *Module) mutate(ctx context.Context, ev ingestion.Event, fn func(*ingestion.RunState)) error {
	r, err := m.ds.LoadRun(ctx, ev.Run)
	if errors.Is(err, ingestion.ErrNotFound) {
		r = ingestion.RunState{Run: ev.Run, Tenant: ev.Tenant}
	} else if err != nil {
		return err
	}
	fn(&r)
	return m.ds.SaveRun(ctx, r)
}

// resourceRef returns a pointer to the named resource's state on the run,
// appending a fresh one if absent. Saving the run cascades the slice back to the
// store, so mutations through this pointer persist.
func resourceRef(r *ingestion.RunState, name string) *ingestion.ResourceState {
	for i := range r.Resources {
		if r.Resources[i].Resource == name {
			return &r.Resources[i]
		}
	}
	r.Resources = append(r.Resources, ingestion.ResourceState{
		Run:      r.Run,
		Tenant:   r.Tenant,
		Resource: name,
		Enabled:  true,
		Status:   ingestion.RunRunning,
	})
	return &r.Resources[len(r.Resources)-1]
}

// finishedAt stamps the run's finish time once.
func finishedAt(r *ingestion.RunState, at time.Time) {
	if r.FinishedAt == nil {
		t := at
		r.FinishedAt = &t
	}
}
