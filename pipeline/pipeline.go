// Package pipeline implements the builder → writer → integrity extraction loop.
//
// A Source opens one filament.RowWriter per (resource, part) on the inlet and
// appends rows into it. The writer is a batch.Builder that flushes a
// filament.Batch on a row or byte threshold, or at the next row once the flush
// timer has asked. A pool of writer goroutines hands each batch to the Sink: the
// writer computes the read-side CRC and compares it against the Sink's write-side
// CRC, publishing facts (batch buffered/written, integrity verified, chunk
// divergence) via an injected emit callback. The pipeline owns no bus, sink-format,
// or orchestration knowledge; that lives in the engine that drives it.
package pipeline

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
	"github.com/galaxy-io/filament/events"
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
	Tenant        filament.TenantID
	Run           filament.RunID
	Sink          filament.Sink
	Emit          func(events.Fact) // nil → facts discarded
	WritePolicies map[string]filament.WritePolicy
	Options       filament.RunOptions
	FlushInterval time.Duration   // default 1s
	Log           filament.Logger // optional
	NextSeq       func() uint64
}

// Pipeline owns the channel-based extraction loop. Construct with New, launch
// with Start, feed via Records, signal end-of-input with CloseIngest, and block
// for completion with Wait.
type Pipeline struct {
	tenant        filament.TenantID
	run           filament.RunID
	sink          filament.Sink
	emit          func(events.Fact)
	writePolicies map[string]filament.WritePolicy
	opts          batch.Options
	flushIvl      time.Duration
	writers       int // concurrent sink writers
	log           filament.Logger

	batchCh chan filament.Batch

	buildersMu sync.Mutex
	builders   []*batch.Builder

	nextSeq  func() uint64 // monotonic fact sequence for (tenant, run) dedup
	pubMu    sync.Mutex    // serializes publish so concurrent writers emit facts safely
	cancel   context.CancelFunc
	done     chan struct{} // closed on cancel or fatal error; releases blocked builders
	doneCh   sync.Once
	tickStop chan struct{}
	tickDone chan struct{}
	wg       sync.WaitGroup // writer pool; Wait blocks on these
	errVal   atomic.Pointer[error]
}

// New builds a Pipeline from cfg, applying defaults. It does not start goroutines.
func New(cfg Config) *Pipeline {
	rows := cfg.Options.BatchMaxRows
	if rows <= 0 && cfg.Sink != nil {
		rows = cfg.Sink.Spec().Capabilities.PreferredBatchRows
	}
	if rows <= 0 {
		rows = defaultBatchRows
	}
	ivl := cfg.FlushInterval
	if ivl <= 0 {
		ivl = defaultFlushInterval
	}
	writers := cfg.Options.SnapshotParallelism
	if writers <= 0 {
		writers = 1 // a single writer preserves batch order
	}
	emit := cfg.Emit
	if emit == nil {
		emit = func(events.Fact) {}
	}
	nextSeq := cfg.NextSeq
	if nextSeq == nil {
		var seq atomic.Uint64
		nextSeq = func() uint64 { return seq.Add(1) }
	}
	return &Pipeline{
		tenant:        cfg.Tenant,
		run:           cfg.Run,
		sink:          cfg.Sink,
		emit:          emit,
		writePolicies: cfg.WritePolicies,
		opts:          batch.Options{MaxRows: rows, MaxBytes: cfg.Options.BatchMaxBytes},
		flushIvl:      ivl,
		writers:       writers,
		log:           cfg.Log,
		batchCh:       make(chan filament.Batch, 2*writers),
		nextSeq:       nextSeq,
		done:          make(chan struct{}),
		tickStop:      make(chan struct{}),
		tickDone:      make(chan struct{}),
	}
}

// Records returns the inlet a Source opens its row writers on. Safe to call
// before Start.
func (p *Pipeline) Records() filament.RecordSink { return &inlet{p: p} }

// Start launches the flush timer and a pool of writer goroutines. The ctx governs
// them; cancel it to stop the pipeline. With parallelism > 1 the Sink's Apply is
// called concurrently (one batch per writer), so a parallel run requires a Sink
// whose Apply is concurrent-safe.
func (p *Pipeline) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	// Cancellation must also release the inlet, or a Source blocked on a full
	// batch channel deadlocks the run.
	go func() {
		<-ctx.Done()
		p.doneCh.Do(func() { close(p.done) })
	}()
	go p.ticker(ctx)
	p.wg.Add(p.writers)
	for range p.writers {
		go p.writer(ctx)
	}
}

// ticker asks every builder to flush each interval, so a slow source's partial
// chunks still move.
func (p *Pipeline) ticker(ctx context.Context) {
	defer close(p.tickDone)
	t := time.NewTicker(p.flushIvl)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			p.buildersMu.Lock()
			for _, b := range p.builders {
				b.RequestFlush()
			}
			p.buildersMu.Unlock()
		case <-p.tickStop:
			return
		case <-ctx.Done():
			return
		}
	}
}

// CloseIngest signals that the Source has finished pushing; extractErr is what
// Extract returned. Call exactly once, after Extract returns. A clean finish
// flushes every builder's remaining rows; a failed one discards them, since the
// source may have stopped inside a row.
func (p *Pipeline) CloseIngest(extractErr error) {
	if p.cancel != nil { // started
		close(p.tickStop)
		<-p.tickDone
	}
	if extractErr == nil {
		p.buildersMu.Lock()
		builders := p.builders
		p.buildersMu.Unlock()
		for _, b := range builders {
			if err := b.Flush(); err != nil {
				p.setErr(err)
				break
			}
		}
	}
	close(p.batchCh)
}

// Wait blocks until the writer pool exits (which happens after CloseIngest closes
// the batch channel and it drains), releases the pipeline context, and returns the
// first fatal error.
func (p *Pipeline) Wait() error {
	p.wg.Wait()
	p.cancel()
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
	if p.cancel != nil {
		p.cancel()
	}
}

// publish stamps a fact with the run identity and a monotonic sequence, then
// hands it to the emit callback. It holds pubMu so concurrent writers (and the
// builders) sequence and emit facts without racing the emit callback or the seq.
func (p *Pipeline) publish(f events.Fact) {
	p.pubMu.Lock()
	defer p.pubMu.Unlock()
	f.Tenant = p.tenant
	f.Run = p.run
	f.Seq = p.nextSeq()
	f.At = time.Now()
	p.emit(f)
}
