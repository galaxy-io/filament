// Package runner executes one Filament ingestion run. It is the single
// implementation of the run sequence: resolve providers, open the sink, drive
// the pipeline, then commit or abort, publishing lifecycle facts throughout.
//
// Both dispatch backends call RunOne — the engine module runs it in-process,
// the worker binary runs it once per Job — so where a run executes is a
// deployment choice and never a behavioral one. State transitions are not
// written here; they are folded from the published facts by the tracker. The
// sole exception is the resumable cursor seed in resolveExtractor.
package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/pipeline"
)

// abortWait bounds sink cleanup after a failed run. Detached from run
// cancellation so shutdown still cleans staged data, bounded so a dead sink
// cannot stall it indefinitely.
const abortWait = 30 * time.Second

// Deps are the process-local dependencies needed to execute one run. Tracer is
// optional; a nil one skips span reporting.
type Deps struct {
	Bus       eventbus.Bus
	DataStore filament.DataStore
	Secrets   filament.Secrets
	Sources   filament.SourceRegistry
	Sinks     filament.SinkRegistry
	Log       filament.Logger
	Tracer    filament.Tracer
}

// SpecFromState builds the RunSpec to execute from a persisted run state.
func SpecFromState(s filament.RunState) filament.RunSpec {
	r := s.Request
	return filament.RunSpec{
		Tenant: r.Tenant, Run: s.Run,
		PipelineID: r.PipelineID, PipelineVersionID: r.PipelineVersionID,
		CheckpointRoute: r.CheckpointRoute, CursorConfigs: r.CursorConfigs,
		Source: r.Source, Sink: r.Sink, Resources: r.Resources, Selectors: r.Selectors,
		IngestionTypes: r.IngestionTypes, Options: r.Options,
	}
}

// ShouldRun reports whether a persisted run is this attempt's to execute. Only a
// freshly requested or resumable-partial run qualifies, so a redelivered trigger
// for a run already running or finished is a no-op rather than duplicated work.
//
// A completed CDC run is the exception: it is a catch-up cycle by construction,
// so re-requesting it continues the change stream from its persisted cursor,
// draining the source up to a fresh watermark and completing again.
func ShouldRun(state filament.RunState) bool {
	switch state.Status {
	case filament.RunRequested, filament.RunPartial:
		return true
	case filament.RunCompleted:
		return filament.IsCDC(state.Request.IngestionTypes)
	default:
		return false
	}
}

// RunOne executes a single extraction end-to-end: resolve providers, open the
// sink, drive the pipeline with the source, then commit or abort. It publishes
// run.started first and exactly one terminal fact (run.completed | run.failed |
// run.partial).
//
//nolint:funlen // the run lifecycle reads best as one sequence
func RunOne(ctx context.Context, deps Deps, spec filament.RunSpec) {
	var span filament.Span
	if deps.Tracer != nil {
		ctx, span = deps.Tracer.Start(ctx, "filament.run")
		defer span.End()
		span.SetAttr("run", string(spec.Run))
		span.SetAttr("tenant", string(spec.Tenant))
		span.SetAttr("source", spec.Source.Provider)
		span.SetAttr("sink", spec.Sink.Provider)
	}

	em := newEmitter(ctx, deps.Bus, deps.Log, spec.Tenant, spec.Run)
	em.span = span
	emit(em, events.RunStarted, "", events.RunStartedEvent{})

	if err := ResolveConfigRefs(ctx, deps.Secrets, &spec); err != nil {
		em.fail(err)
		return
	}

	src, err := deps.Sources.Resolve(spec.Source.Provider)
	if err != nil {
		em.fail(fmt.Errorf("resolve source %q: %w", spec.Source.Provider, err))
		return
	}
	if err := src.Configure(ctx, filament.NewConfig(spec.Source.Config)); err != nil {
		em.fail(fmt.Errorf("configure source %q: %w", spec.Source.Provider, err))
		return
	}
	defer func() { _ = src.Teardown(ctx) }()
	plannedResources, err := PlanResources(ctx, src, spec.Resources, spec.Selectors)
	if err != nil {
		em.fail(fmt.Errorf("plan resources: %w", err))
		return
	}
	spec.Resources = plannedResources

	snk, err := deps.Sinks.Resolve(spec.Sink.Provider)
	if err != nil {
		em.fail(fmt.Errorf("resolve sink %q: %w", spec.Sink.Provider, err))
		return
	}
	plan, err := filament.ResolveIngestionPlan(ctx, src, snk, spec)
	if err != nil {
		em.fail(err)
		return
	}
	spec.WritePolicies = plan.WritePolicies
	// Ordered reads (incremental cursors, CDC streams) cannot shard.
	if incremental, _ := partitionCheckpointing(spec); plan.RequiresCDC || len(incremental) > 0 {
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
		if aerr := snk.Abort(ctx); aerr != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", aerr, filament.Field{Key: "run", Value: string(spec.Run)})
		}
		em.fail(fmt.Errorf("ensure schema: %w", err))
		return
	}

	// Announce the resources this run will touch (when known up front).
	for _, res := range spec.Resources {
		emit(em, events.ResourceStarted, res, events.ResourceStartedEvent{})
	}

	extractor, err := resolveExtractor(ctx, deps.DataStore, src, spec, plan)
	if err != nil {
		if aerr := snk.Abort(ctx); aerr != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", aerr, filament.Field{Key: "run", Value: string(spec.Run)})
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
		Log:           deps.Log,
	})
	p.Start(ctx)

	// Extract on its own goroutine so the writer can apply backpressure through
	// the inlet. CloseIngest after Extract returns drains the batcher; Wait then
	// blocks until the writer finishes. A panicking source is contained here.
	extractErrCh := make(chan error, 1)
	go func() {
		extractErrCh <- safeCall(func() error {
			return extractor(ctx, p.Records(), filament.ExtractOpts{
				Resources:   spec.Resources,
				Selectors:   spec.Selectors,
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

	// The run is ending: detach fact publishing from run cancellation so the
	// terminal facts survive host shutdown — the obituary must outlive the
	// death, or the row strands in RunRunning forever.
	defer em.finish()()

	if runErr != nil {
		for _, res := range resources {
			emit(em, events.ResourceFailed, res, events.ResourceFailedEvent{Error: runErr.Error()})
		}
		if isResumableRun(spec, plan) {
			em.partial(runErr)
			return
		}
		em.fail(runErr)
		// Abort after the obituary, on its own detached context: cleanup must
		// not eat the terminal publish window, and a slow sink must not strand
		// the row in RunRunning.
		abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), abortWait)
		defer cancel()
		if err := snk.Abort(abortCtx); err != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", err, filament.Field{Key: "run", Value: string(spec.Run)})
		}
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
