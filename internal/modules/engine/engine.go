// Package engine runs the core pipeline using the resolved Source and Sink
// providers. It is the bus-driven port of the original runner.Run(): it reacts to
// a run.requested fact, loads the run's request from the DataStore, resolves the
// Source and Sink, drives the extraction pipeline, and publishes lifecycle facts.
// The tracker owns lifecycle state folding; the engine only seeds source
// checkpoints needed before resumable extraction can start.
package engine

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/host"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/module"
	"github.com/galaxy-io/filament/pipeline"
)

// Module is the extraction engine. One run.requested fact drives one extraction.
type Module struct {
	bus     eventbus.Bus
	ds      ingestion.DataStore
	sources ingestion.SourceRegistry
	sinks   ingestion.SinkRegistry
	log     ingestion.Logger
}

// New returns an unmounted engine. Providers are injected by Mount.
func New() *Module { return &Module{} }

var _ module.Module = (*Module)(nil)

func (m *Module) Name() string { return "engine" }

// Subscriptions: a durable consumer over run.requested across every tenant/run.
// The fact is only a trigger; the request payload is loaded from the DataStore.
func (m *Module) Subscriptions() []host.Subscription {
	return []host.Subscription{
		{Pattern: events.SubjectPattern(events.RunRequested), Durable: "engine", Handler: events.Handler(events.RunRequested, m.onRunRequested)},
	}
}

// Mount captures the providers this module uses. Cheap, no I/O.
func (m *Module) Mount(_ context.Context, d module.Deps) error {
	m.bus = d.Bus
	m.ds = d.DataStore
	m.sources = d.Sources
	m.sinks = d.Sinks
	m.log = d.Log
	return nil
}

// onRunRequested loads the requested run and executes it. A failure to load is
// transient (the request state may not be persisted yet) and is naked for
// redelivery; a run that fails for any other reason is reported as a fact and
// acked — blindly re-running a whole extraction would duplicate work.
func (m *Module) onRunRequested(ctx context.Context, ev events.Event[events.RunRequestedEvent]) error {
	state, err := m.ds.LoadRun(ctx, ev.Run)
	if err != nil {
		return fmt.Errorf("engine: load run %q: %w", ev.Run, err)
	}
	// Idempotency: only a freshly-requested run is ours to start. A redelivered
	// trigger for a run already running or finished is acked and ignored — except
	// a completed CDC run, which is a catch-up cycle by construction: re-requesting
	// it continues the change stream from its persisted cursor, so each request
	// drains the source up to a fresh watermark and completes again.
	cdcCycle := state.Request.IngestionType.OrDefault() == ingestion.IngestionCDC &&
		state.Status == ingestion.RunCompleted
	if state.Status != ingestion.RunRequested && state.Status != ingestion.RunPartial && !cdcCycle {
		return nil
	}
	m.runOne(ctx, specFromState(state))
	return nil
}

// specFromState builds the RunSpec the engine executes from the persisted run
// request. Mode defaults to full; checkpoint-based resume lands with incremental
// sources in a later phase.
func specFromState(s ingestion.RunState) ingestion.RunSpec {
	r := s.Request
	return ingestion.RunSpec{
		Tenant:        r.Tenant,
		Run:           s.Run,
		Source:        r.Source,
		Sink:          r.Sink,
		DataStore:     r.DataStore,
		Resources:     r.Resources,
		Selectors:     r.Selectors,
		IngestionType: r.IngestionType.OrDefault(),
		Mode:          ingestion.ModeFull,
		Options:       r.Options,
	}
}

// runOne executes a single extraction end-to-end: resolve providers, open the
// sink, drive the pipeline with the source, then commit or abort. It publishes
// run.started first and exactly one terminal fact (run.completed | run.failed).
func (m *Module) runOne(ctx context.Context, spec ingestion.RunSpec) {
	em := newEmitter(ctx, m.bus, m.log, spec.Tenant, spec.Run)
	emit(em, events.RunStarted, "", events.RunStartedEvent{})

	src, err := m.sources.Resolve(spec.Source.Provider)
	if err != nil {
		em.fail(fmt.Errorf("resolve source %q: %w", spec.Source.Provider, err))
		return
	}
	if err := src.Configure(ctx, ingestion.NewConfig(spec.Source.Config)); err != nil {
		em.fail(fmt.Errorf("configure source %q: %w", spec.Source.Provider, err))
		return
	}
	defer func() { _ = src.Teardown(ctx) }()
	if planner, ok := src.(ingestion.ResourcePlanner); ok {
		resources, err := planner.PlanResources(ctx, spec.Resources, spec.Selectors)
		if err != nil {
			em.fail(fmt.Errorf("plan resources: %w", err))
			return
		}
		spec.Resources = resources
	}

	snk, err := m.sinks.Resolve(spec.Sink.Provider)
	if err != nil {
		em.fail(fmt.Errorf("resolve sink %q: %w", spec.Sink.Provider, err))
		return
	}
	plan, err := resolveIngestionPlan(ctx, src, snk, spec)
	if err != nil {
		em.fail(err)
		return
	}
	spec.IngestionType = plan.Type
	spec.Mode = plan.SourcePolicy.Mode
	if plan.SourcePolicy.Ordered {
		spec.Options.SnapshotParallelism = 1
	}
	if err := snk.Open(ctx, spec); err != nil {
		em.fail(fmt.Errorf("open sink %q: %w", spec.Sink.Provider, err))
		return
	}

	// A schema-aware sink needs typed DDL before any write. When the source can
	// supply per-resource schemas, ensure each one up front so a Schematized sink
	// (e.g. iceberg, postgres) creates/evolves its tables before extraction.
	if err := ensureSchemas(ctx, src, snk, spec); err != nil {
		if aerr := snk.Abort(ctx); aerr != nil && m.log != nil {
			m.log.Error("engine: sink abort", aerr, ingestion.Field{Key: "run", Value: string(spec.Run)})
		}
		em.fail(fmt.Errorf("ensure schema: %w", err))
		return
	}

	// Announce the resources this run will touch (when known up front).
	for _, res := range spec.Resources {
		emit(em, events.ResourceStarted, res, events.ResourceStartedEvent{})
	}

	extractor, err := m.resolveExtractor(ctx, src, spec, plan)
	if err != nil {
		if aerr := snk.Abort(ctx); aerr != nil && m.log != nil {
			m.log.Error("engine: sink abort", aerr, ingestion.Field{Key: "run", Value: string(spec.Run)})
		}
		em.fail(err)
		return
	}

	p := pipeline.New(pipeline.Config{
		Tenant:        spec.Tenant,
		Run:           spec.Run,
		Sink:          snk,
		Emit:          em.publish,
		NextSeq:       em.next,
		WritePolicies: plan.WritePolicies,
		Options:       spec.Options,
		Log:           m.log,
	})
	p.Start(ctx)

	// Extract on its own goroutine so the writer can apply backpressure through
	// the inlet. CloseIngest after Extract returns drains the batcher; Wait then
	// blocks until the writer finishes. A panicking source is contained here.
	extractErrCh := make(chan error, 1)
	go func() {
		extractErrCh <- safeCall(func() error {
			return extractor(ctx, p.Records(), ingestion.ExtractOpts{
				Resources:   spec.Resources,
				Selectors:   spec.Selectors,
				Mode:        spec.Mode,
				Parallelism: spec.Options.SnapshotParallelism,
			})
		})
		p.CloseIngest()
	}()

	waitErr := p.Wait()
	extractErr := <-extractErrCh

	// A write failure (waitErr) is the root cause and takes precedence: when the
	// pipeline fails, the inlet returns it to Extract too, so extractErr is just
	// the echo.
	runErr := waitErr
	if runErr == nil {
		runErr = extractErr
	}

	resources := resolveResources(spec.Resources, em.seenResources())

	if runErr != nil {
		if isResumableRun(spec, plan) {
			for _, res := range resources {
				emit(em, events.ResourceFailed, res, events.ResourceFailedEvent{Error: runErr.Error()})
			}
			em.partial(runErr)
			return
		}
		if err := snk.Abort(ctx); err != nil && m.log != nil {
			m.log.Error("engine: sink abort", err, ingestion.Field{Key: "run", Value: string(spec.Run)})
		}
		for _, res := range resources {
			emit(em, events.ResourceFailed, res, events.ResourceFailedEvent{Error: runErr.Error()})
		}
		em.fail(runErr)
		return
	}

	if err := snk.Commit(ctx); err != nil {
		em.fail(fmt.Errorf("commit sink %q: %w", spec.Sink.Provider, err))
		return
	}
	for _, res := range resources {
		records, bytes := em.resourceTally(res)
		emit(em, events.ResourceCompleted, res, events.ResourceCompletedEvent{Records: records, Bytes: bytes})
	}
	records, bytes := em.runTotals()
	emit(em, events.RunCompleted, "", events.RunCompletedEvent{Records: records, Bytes: bytes})
}
