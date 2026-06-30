// Package pipeline implements the batcher → writer → integrity extraction loop.
//
// A Source pushes ingestion.Records into the inlet (Records). A pool of batcher
// goroutines — sharded by resource so a resource is always handled by the same
// batcher — accumulates records and, on a row-count threshold or a timer tick,
// flushes a ingestion.Batch. A pool of writer goroutines hands each batch to the Sink:
// the writer computes the read-side CRC and compares it against the Sink's
// write-side CRC, publishing facts (batch buffered/written, integrity verified,
// chunk divergence) via an injected emit callback. Computing the read CRC in the
// (parallel) writers rather than the batcher keeps the per-record batcher work to
// a map-append. The pipeline owns no bus, sink-format, or orchestration knowledge
// that lives in the engine that drives it.
package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
)

// defaultBatchRows is used when Config.Options.BatchMaxRows is unset.
const defaultBatchRows = 256

// defaultFlushInterval bounds how long a partial batch waits before flushing.
const defaultFlushInterval = time.Second

// Config constructs a Pipeline. Tenant/Run stamp the batches and facts; Sink
// receives batches; Emit publishes facts (nil → discarded). The Sink's Open and
// Commit/Abort are the engine's responsibility, not the pipeline's — the pipeline
// only calls Apply.
type Config struct {
	Tenant        ingestion.TenantID
	Run           ingestion.RunID
	Sink          ingestion.Sink
	Emit          func(ingestion.Event) // nil → facts discarded
	WritePolicies map[string]ingestion.WritePolicy
	Options       ingestion.RunOptions
	FlushInterval time.Duration // default 1s
	Log           ingestion.Logger    // optional
	NextSeq       func() uint64
}

// Pipeline owns the channel-based extraction loop. Construct with New, launch
// with Start, feed via Records, signal end-of-input with CloseIngest, and block
// for completion with Wait.
type Pipeline struct {
	tenant        ingestion.TenantID
	run           ingestion.RunID
	sink          ingestion.Sink
	emit          func(ingestion.Event)
	writePolicies map[string]ingestion.WritePolicy
	batchRows     int
	flushIvl      time.Duration
	writers       int // concurrent sink writers
	shards        int // batcher goroutines; records route to a shard by resource hash
	log           ingestion.Logger

	ingestChs []chan ingestion.Record // one inlet channel per batcher shard
	batchCh   chan ingestion.Batch

	nextSeq   func() uint64 // monotonic fact sequence for (tenant, run) dedup
	pubMu     sync.Mutex    // serializes publish so concurrent writers emit facts safely
	cancel    context.CancelFunc
	done      chan struct{}
	doneCh    sync.Once
	batcherWg sync.WaitGroup // batcher shards; batchCh closes once all exit
	wg        sync.WaitGroup // writer pool; Wait blocks on these
	errVal    atomic.Pointer[error]
}

// New builds a Pipeline from cfg, applying defaults. It does not start goroutines.
func New(cfg Config) *Pipeline {
	rows := cfg.Options.BatchMaxRows
	if rows <= 0 {
		rows = defaultBatchRows
	}
	ivl := cfg.FlushInterval
	if ivl <= 0 {
		ivl = defaultFlushInterval
	}
	parallelism := cfg.Options.SnapshotParallelism
	if parallelism <= 0 {
		parallelism = 1 // single writer + single batcher preserves ordering and the serial path
	}
	emit := cfg.Emit
	if emit == nil {
		emit = func(ingestion.Event) {}
	}
	nextSeq := cfg.NextSeq
	if nextSeq == nil {
		var seq atomic.Uint64
		nextSeq = func() uint64 { return seq.Add(1) }
	}
	ingestChs := make([]chan ingestion.Record, parallelism)
	for i := range ingestChs {
		ingestChs[i] = make(chan ingestion.Record, rows*4)
	}
	return &Pipeline{
		tenant:        cfg.Tenant,
		run:           cfg.Run,
		sink:          cfg.Sink,
		emit:          emit,
		writePolicies: cfg.WritePolicies,
		batchRows:     rows,
		flushIvl:      ivl,
		writers:       parallelism,
		shards:        parallelism,
		log:           cfg.Log,
		ingestChs:     ingestChs,
		batchCh:       make(chan ingestion.Batch, 4),
		nextSeq:       nextSeq,
		done:          make(chan struct{}),
	}
}

// Records returns the inlet a Source pushes into. Safe to call before Start.
func (p *Pipeline) Records() ingestion.RecordSink {
	return &inlet{chs: p.ingestChs, done: p.done, err: p.Err}
}

// shardFor maps a resource to a batcher shard by FNV-1a hash, so every record of
// a resource lands in the same batcher (preserving its per-resource chunk seq and
// keyset order) while distinct resources spread across shards.
func shardFor(resource string, n int) int {
	if n <= 1 {
		return 0
	}
	const offset, prime = uint32(2166136261), uint32(16777619)
	h := offset
	for i := range len(resource) {
		h ^= uint32(resource[i])
		h *= prime
	}
	return int(h % uint32(n))
}

// Start launches the batcher shards and a pool of writer goroutines. The ctx
// governs them; cancel it to stop the pipeline. With parallelism > 1 the Sink's
// Apply is called concurrently (one batch per writer), so a parallel run requires
// a Sink whose Apply is concurrent-safe.
func (p *Pipeline) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	p.batcherWg.Add(p.shards)
	for i := range p.shards {
		go p.batcher(ctx, i)
	}
	go func() { p.batcherWg.Wait(); close(p.batchCh) }()
	p.wg.Add(p.writers)
	for range p.writers {
		go p.writer(ctx)
	}
}

// CloseIngest signals that the Source has finished pushing. Call exactly once,
// after Extract returns. Each batcher shard flushes its partial batches and exits.
func (p *Pipeline) CloseIngest() {
	for _, ch := range p.ingestChs {
		close(ch)
	}
}

// Wait blocks until the writer pool exits (which happens after every batcher shard
// has drained and closed batchCh) and returns the first fatal error.
func (p *Pipeline) Wait() error {
	p.wg.Wait()
	return p.Err()
}

// Err returns the pipeline's fatal error, if any.
func (p *Pipeline) Err() error {
	if ep := p.errVal.Load(); ep != nil {
		return *ep
	}
	return nil
}

// setErr records the first fatal error, signals the inlet, and cancels the loop.
func (p *Pipeline) setErr(err error) {
	p.errVal.CompareAndSwap(nil, &err)
	p.doneCh.Do(func() { close(p.done) })
	p.cancel()
}

// publish stamps a fact with the run identity and a monotonic sequence, then
// hands it to the emit callback. It holds pubMu so concurrent writers (and the
// batcher) sequence and emit facts without racing the emit callback or the seq.
func (p *Pipeline) publish(ev ingestion.Event) {
	p.pubMu.Lock()
	defer p.pubMu.Unlock()
	ev.Tenant = p.tenant
	ev.Run = p.run
	ev.Seq = p.nextSeq()
	ev.At = time.Now()
	p.emit(ev)
}
