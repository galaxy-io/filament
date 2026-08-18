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
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
)

// Module consumes every ingestion fact and persists the derived run state.
type Module struct {
	ds  filament.DataStore
	log filament.Logger
	mx  filament.Metrics

	// Resumable-checkpoint accumulators, keyed by (run, resource). A single durable
	// consumer folds facts serially, but the maps are mutex-guarded in case the host
	// delivers concurrently. cp holds the live merged cursor; since counts written
	// batches toward the per-run persist cadence.
	mu    sync.Mutex
	cp    map[ckKey]filament.Checkpoint
	since map[ckKey]int
	every map[filament.RunID]int // cached per-run CheckpointEvery cadence
	bm    map[ckKey]*bmAccount   // bitmap per-shard ack/want counters
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
	run      filament.RunID
	resource string
}

// New returns an unmounted tracker. Providers are injected by Mount.
func New() *Module {
	return &Module{cp: map[ckKey]filament.Checkpoint{}, since: map[ckKey]int{}, every: map[filament.RunID]int{}, bm: map[ckKey]*bmAccount{}}
}

// compile-time check that we satisfy the Module contract.
var _ module.Module = (*Module)(nil)

// Name identifies this module.
func (m *Module) Name() string { return "tracker" }

// Subscriptions declares one durable consumer over the whole versioned namespace, so the
// tracker sees every fact and survives restarts (resuming where it left off).
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{
		// MaxInFlight 1 serializes folding across replicas: the fold is
		// read-modify-write, so concurrent or out-of-order delivery loses
		// updates (a stale save can clobber a folded terminal).
		{Pattern: events.AllPattern(), Durable: "tracker", Replay: true, MaxInFlight: 1, Handler: m.onFact},
	}
}

// Mount captures the providers this module uses. Cheap, no I/O.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.ds = d.DataStore
	m.log = d.Log
	m.mx = d.Metrics
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
	f, err := events.Decode(msg)
	if err != nil {
		return nil // not a Fact: a foreign publisher on our pattern — skip
	}

	seen, err := m.ds.DedupSeen(ctx, string(f.Tenant), f.Run, msg.Seq())
	if err != nil {
		return err
	}
	if seen {
		return nil // already applied — idempotent no-op
	}

	return m.apply(ctx, f)
}

// apply folds one fact into persisted run/resource/checkpoint state. Facts not
// listed here (page-fetched, batch-buffered, integrity-verified, rate-limited,
// schedule-fired) carry no durable state change and are ignored.
func (m *Module) apply(ctx context.Context, f events.Fact) error {
	env := f.Envelope
	switch d := f.Data.(type) {
	case events.RunStartedEvent:
		return m.mutate(ctx, env, func(r *filament.RunState) {
			r.Status = filament.RunRunning
			// The store keeps the first stamp, so a redelivered fact cannot move it.
			r.StartedAt = env.At
		})

	case events.RunCompletedEvent:
		return m.terminal(ctx, env, "completed", func(r *filament.RunState) {
			r.Status = filament.RunCompleted
			// The terminal fact carries the engine's authoritative totals.
			r.Records = d.Records
			r.Bytes = d.Bytes
		})

	case events.RunFailedEvent:
		return m.terminal(ctx, env, "failed", func(r *filament.RunState) {
			r.Status = filament.RunFailed
			r.Error = d.Error
		})

	case events.RunPartialEvent:
		return m.terminal(ctx, env, "partial", func(r *filament.RunState) {
			r.Status = filament.RunPartial
			r.Error = d.Error
		})

	case events.ResourceStartedEvent:
		return m.mutate(ctx, env, func(r *filament.RunState) {
			rs := resourceRef(r, env.Resource)
			rs.Enabled = true
			if rs.Status == filament.RunRequested {
				rs.Status = filament.RunRunning
			}
		})

	case events.ResourceCompletedEvent:
		if err := m.mutate(ctx, env, func(r *filament.RunState) {
			rs := resourceRef(r, env.Resource)
			rs.Status = filament.RunCompleted
			rs.Records = d.Records
			rs.Bytes = d.Bytes
		}); err != nil {
			return err
		}
		m.flushResource(ctx, env.Run, env.Resource)
		return nil

	case events.ResourceFailedEvent:
		return m.mutate(ctx, env, func(r *filament.RunState) {
			rs := resourceRef(r, env.Resource)
			rs.Status = filament.RunFailed
			rs.Error = d.Error
		})

	case events.BatchWrittenEvent:
		// Incremental progress: accumulate per-resource and run totals as chunks
		// land, so observers see counts climb before the run finishes.
		var pipeline string
		if err := m.mutate(ctx, env, func(r *filament.RunState) {
			r.Records += d.Records
			r.Bytes += d.Bytes
			rs := resourceRef(r, env.Resource)
			rs.Records += d.Records
			rs.Bytes += d.Bytes
			if rs.Status == filament.RunRequested {
				rs.Status = filament.RunRunning
			}
			pipeline = r.Request.PipelineID
		}); err != nil {
			return err
		}
		m.observeBatch(env, pipeline, d.Records, d.Bytes)
		if cp, persist := m.foldCursor(ctx, env, d.Checkpoint); cp != nil && persist {
			if err := m.saveCheckpoint(ctx, env.Run, cp); err != nil {
				m.observeCheckpointFailure()
				if m.log != nil {
					m.log.Error("tracker: save checkpoint", err, filament.Field{Key: "run", Value: string(env.Run)})
				}
			}
		}
		return nil

	case events.HeartbeatEvent:
		// Usage counters are cumulative (CPU) or high-water (memory), so max
		// keeps the fold idempotent under redelivery and reordering.
		return m.mutate(ctx, env, func(r *filament.RunState) {
			r.CPUSeconds = max(r.CPUSeconds, d.CPUSeconds)
			r.MemoryPeakBytes = max(r.MemoryPeakBytes, d.MemoryPeakBytes, d.MemoryBytes)
		})

	case events.CheckpointSavedEvent:
		return m.applyCheckpoint(ctx, env, d.Checkpoint)

	case events.WatermarkAdvancedEvent:
		// Observational only: a source has seen a newer cursor, but the rows
		// carrying it may not be durable yet. batch.written remains the sole
		// input to checkpoint persistence.
		return nil

	default:
		return nil // facts this module doesn't fold are acked and ignored
	}
}

// terminal folds a run's terminal fact: apply the status mutation, stamp the
// finish time, record the run metrics, and flush the run's cursors.
func (m *Module) terminal(ctx context.Context, env events.Envelope, status string, fn func(*filament.RunState)) error {
	var labels runLabels
	if err := m.mutate(ctx, env, func(r *filament.RunState) {
		fn(r)
		finishedAt(r, env.At)
		labels = labelsFor(r)
	}); err != nil {
		return err
	}
	m.observeRun(env, status, labels)
	m.flushRun(ctx, env.Run, status == "completed")
	m.evictRun(env.Run)
	return nil
}

// evictRun drops the run's accumulator entries once it is terminal, so the
// maps don't grow with every run the process ever tracked.
func (m *Module) evictRun(run filament.RunID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key := range m.cp {
		if key.run == run {
			delete(m.cp, key)
		}
	}
	for key := range m.since {
		if key.run == run {
			delete(m.since, key)
		}
	}
	for key := range m.bm {
		if key.run == run {
			delete(m.bm, key)
		}
	}
	delete(m.every, run)
}

// applyCheckpoint persists a cursor fact's checkpoint and pins it on the resource.
func (m *Module) applyCheckpoint(ctx context.Context, env events.Envelope, cp *filament.CheckpointData) error {
	if cp == nil {
		return nil
	}
	if err := m.saveCheckpoint(ctx, env.Run, cp); err != nil {
		return err
	}
	return m.mutate(ctx, env, func(r *filament.RunState) {
		resourceRef(r, env.Resource).Checkpoint = cp
	})
}

// foldCursor merges a batch.written keyset delta into the resource's accumulated
// checkpoint and reports whether the run's persist cadence is due. Returns (nil,false)
// for a non-keyset batch or before the shard layout (the plan) has been seeded.
func (m *Module) foldCursor(ctx context.Context, env events.Envelope, cp *filament.CheckpointData) (filament.Checkpoint, bool) {
	if cp == nil {
		return nil, false
	}
	if part, ack, want, hasWant, ok := checkpoint.CoarseDelta(cp); ok {
		return m.foldBitmap(ctx, env, part, ack, want, hasWant)
	}
	if _, _, ok := checkpoint.ParseStream(cp); ok {
		return m.foldStream(ctx, env, cp)
	}
	key := ckKey{env.Run, env.Resource}

	m.mu.Lock()
	base, ok := m.cp[key]
	if !ok {
		base = m.loadCheckpoint(ctx, env.Run, env.Resource) // recover layout after a restart
		m.cp[key] = base
	}
	merged := checkpoint.MergeShardDelta(base, cp)
	if merged == nil {
		m.mu.Unlock()
		return nil, false
	}
	m.cp[key] = merged
	m.since[key]++
	persist := m.since[key] >= m.cadence(ctx, env.Run)
	if persist {
		m.since[key] = 0
	}
	m.mu.Unlock()
	return merged, persist
}

// foldStream folds a change-stream position delta: the newer position (guarded by the
// per-run seq, since concurrent writers can publish batch facts out of order) replaces
// the resource's cursor wholesale — a stream cursor has no shard layout to merge into.
func (m *Module) foldStream(ctx context.Context, env events.Envelope, cp *filament.CheckpointData) (filament.Checkpoint, bool) {
	key := ckKey{env.Run, env.Resource}

	m.mu.Lock()
	base, ok := m.cp[key]
	if !ok {
		base = m.loadCheckpoint(ctx, env.Run, env.Resource) // recover position after a restart
		m.cp[key] = base
	}
	merged := checkpoint.MergeStream(base, cp)
	if merged == nil {
		m.mu.Unlock()
		return nil, false
	}
	m.cp[key] = merged
	m.since[key]++
	persist := m.since[key] >= m.cadence(ctx, env.Run)
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
func (m *Module) foldBitmap(ctx context.Context, env events.Envelope, part, ack, want int, hasWant bool) (filament.Checkpoint, bool) {
	key := ckKey{env.Run, env.Resource}

	m.mu.Lock()
	defer m.mu.Unlock()

	base, ok := m.cp[key]
	if !ok {
		base = m.loadCheckpoint(ctx, env.Run, env.Resource) // recover layout after a restart
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
func (m *Module) flushResource(ctx context.Context, run filament.RunID, resource string) {
	key := ckKey{run, resource}
	m.mu.Lock()
	cp := m.cp[key]
	if cp == nil {
		cp = m.loadCheckpoint(ctx, run, resource)
	}
	if promoted, ok := checkpoint.PromoteIncrementalBackfill(cp); ok {
		cp = promoted
		m.cp[key] = promoted
	}
	m.since[key] = 0
	m.mu.Unlock()
	if cp != nil {
		if err := m.commitCheckpoint(ctx, run, cp); err != nil {
			m.observeCheckpointFailure()
			if m.log != nil {
				m.log.Error("tracker: flush checkpoint", err, filament.Field{Key: "run", Value: string(run)})
			}
		}
	}
}

// flushRun persists every accumulated cursor for the run. Incremental and CDC
// cursors become durable only after a successful sink commit; other cursor modes
// retain their existing attempt-local resume behavior.
func (m *Module) flushRun(ctx context.Context, run filament.RunID, committed bool) {
	m.mu.Lock()
	pending := make(map[string]filament.Checkpoint)
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
		if err := m.persistCheckpoint(ctx, run, cp, committed); err != nil {
			m.observeCheckpointFailure()
			if m.log != nil {
				m.log.Error("tracker: flush checkpoint", err, filament.Field{Key: "run", Value: string(run)})
			}
		}
	}
}

// cadence is the run's CheckpointEvery (cached; default when unset).
func (m *Module) cadence(ctx context.Context, run filament.RunID) int {
	if n, ok := m.every[run]; ok {
		return n
	}
	n := filament.DefaultCheckpointEvery
	if r, err := m.ds.LoadRun(ctx, run); err == nil && r.Request.Options.CheckpointEvery > 0 {
		n = r.Request.Options.CheckpointEvery
	}
	m.every[run] = n
	return n
}

// saveCheckpoint persists attempt-local progress. Cross-run incremental and CDC
// progress remains tentative until the sink commits and commitCheckpoint promotes it.
func (m *Module) saveCheckpoint(ctx context.Context, run filament.RunID, cp filament.Checkpoint) error {
	return m.persistCheckpoint(ctx, run, cp, false)
}

func (m *Module) commitCheckpoint(ctx context.Context, run filament.RunID, cp filament.Checkpoint) error {
	return m.persistCheckpoint(ctx, run, cp, true)
}

func (m *Module) persistCheckpoint(ctx context.Context, run filament.RunID, cp filament.Checkpoint, committed bool) error {
	state, err := m.ds.LoadRun(ctx, run)
	if err != nil {
		return err
	}
	mode := filament.SourcePolicyForIngestion(filament.TypeFor(state.Request.IngestionTypes, cp.Resource())).Mode
	if mode == filament.ModeIncremental || mode == filament.ModeCDC {
		if !committed {
			return nil
		}
		if key, ok := state.Request.ResourceCheckpointKey(cp.Resource()); ok {
			return m.ds.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: run, Checkpoint: cp})
		}
	}
	return m.ds.SaveCheckpoint(ctx, run, cp)
}

// loadCheckpoint returns the persisted cursor for (run, resource) or nil.
func (m *Module) loadCheckpoint(ctx context.Context, run filament.RunID, resource string) filament.Checkpoint {
	if state, err := m.ds.LoadRun(ctx, run); err == nil {
		mode := filament.SourcePolicyForIngestion(filament.TypeFor(state.Request.IngestionTypes, resource)).Mode
		if mode == filament.ModeIncremental || mode == filament.ModeCDC {
			if key, ok := state.Request.ResourceCheckpointKey(resource); ok {
				stored, err := m.ds.LoadResourceCheckpoint(ctx, key)
				if err == nil {
					return stored.Checkpoint
				}
				return nil
			}
		}
	}
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
func (m *Module) mutate(ctx context.Context, env events.Envelope, fn func(*filament.RunState)) error {
	r, err := m.ds.LoadRun(ctx, env.Run)
	if errors.Is(err, filament.ErrNotFound) {
		r = filament.RunState{Run: env.Run, Tenant: env.Tenant}
	} else if err != nil {
		return err
	}
	fn(&r)
	return m.ds.SaveRun(ctx, r)
}

// resourceRef returns a pointer to the named resource's state on the run,
// appending a fresh one if absent. Saving the run cascades the slice back to the
// store, so mutations through this pointer persist.
func resourceRef(r *filament.RunState, name string) *filament.ResourceState {
	for i := range r.Resources {
		if r.Resources[i].Resource == name {
			return &r.Resources[i]
		}
	}
	r.Resources = append(r.Resources, filament.ResourceState{
		Run:      r.Run,
		Tenant:   r.Tenant,
		Resource: name,
		Enabled:  true,
		Status:   filament.RunRunning,
	})
	return &r.Resources[len(r.Resources)-1]
}

// finishedAt stamps the run's finish time once.
func finishedAt(r *filament.RunState, at time.Time) {
	if r.EndedAt == nil {
		t := at
		r.EndedAt = &t
	}
}
