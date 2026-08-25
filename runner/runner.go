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
	"errors"
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
		WorkerConfiguration: r.WorkerConfiguration,
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
// run.partial | run.paused | run.canceled).
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
	extractCtx, control, err := prepareRunExecution(ctx, deps, spec, em)
	if err != nil {
		em.failed(err, nil, false)
		return
	}
	defer control.close()
	emit(em, events.RunStarted, "", events.RunStartedEvent{})

	if err := ResolveConfigRefs(extractCtx, deps.Secrets, &spec); err != nil {
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(err, nil, false)
		return
	}

	src, err := deps.Sources.Resolve(spec.Source.Provider)
	if err != nil {
		em.failed(fmt.Errorf("resolve source %q: %w", spec.Source.Provider, err), nil, false)
		return
	}
	if err := src.Configure(extractCtx, filament.NewConfig(spec.Source.Config)); err != nil {
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(fmt.Errorf("configure source %q: %w", spec.Source.Provider, err), nil, false)
		return
	}
	defer func() { _ = src.Teardown(ctx) }()
	plannedResources, err := PlanResources(extractCtx, src, spec.Resources, spec.Selectors)
	if err != nil {
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(fmt.Errorf("plan resources: %w", err), nil, false)
		return
	}
	spec.Resources = plannedResources

	snk, err := deps.Sinks.Resolve(spec.Sink.Provider)
	if err != nil {
		em.failed(fmt.Errorf("resolve sink %q: %w", spec.Sink.Provider, err), nil, false)
		return
	}
	plan, err := filament.ResolveIngestionPlan(extractCtx, src, snk, spec)
	if err != nil {
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(err, nil, false)
		return
	}
	spec.WritePolicies = plan.WritePolicies
	// Ordered reads (incremental cursors, CDC streams, checkpointed resume)
	// cannot shard: parallel writers apply batches out of order, so a keyset
	// cursor could persist behind rows already written.
	if incremental, checkpointed := partitionCheckpointing(spec); plan.RequiresCDC || len(incremental) > 0 || len(checkpointed) > 0 {
		spec.Options.SnapshotParallelism = 1
	}
	if err := snk.Open(extractCtx, spec); err != nil {
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(fmt.Errorf("open sink %q: %w", spec.Sink.Provider, err), nil, false)
		return
	}

	// A schema-aware sink needs typed DDL before any write. When the source can
	// supply per-resource schemas, ensure each one up front so a Schematized sink
	// (e.g. iceberg, postgres) creates/evolves its tables before extraction.
	if err := ensureSchemas(extractCtx, src, snk, spec); err != nil {
		abortSink(ctx, deps, spec, snk)
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(fmt.Errorf("ensure schema: %w", err), nil, false)
		return
	}

	// Announce the resources this run will touch (when known up front).
	for _, res := range spec.Resources {
		emit(em, events.ResourceStarted, res, events.ResourceStartedEvent{})
	}

	extractor, err := resolveExtractor(extractCtx, deps.DataStore, src, spec, plan, deps.Log)
	if err != nil {
		abortSink(ctx, deps, spec, snk)
		if emitControlledIfStopped(extractCtx, err, control, em) {
			return
		}
		em.failed(err, spec.Resources, false)
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
			return extractor(extractCtx, p.Records(), filament.ExtractOpts{
				Resources:   spec.Resources,
				Selectors:   spec.Selectors,
				Parallelism: spec.Options.SnapshotParallelism,
				Observe:     sourceObserver(em),
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
	if waitErr == nil && stoppedByControl(extractCtx, extractErr, control.requested()) {
		finishControlled(ctx, deps, spec, plan, snk, em, resources, control.seal())
		return
	}

	if runErr != nil {
		resumable := isResumableRun(spec, plan)
		em.failed(runErr, resources, resumable)
		if resumable {
			return
		}
		// Abort after the obituary, on its own detached context: cleanup must
		// not eat the terminal publish window, and a slow sink must not strand
		// the row in RunRunning.
		abortSink(ctx, deps, spec, snk)
		return
	}

	if signal := control.seal(); signal != controlNone {
		finishControlled(ctx, deps, spec, plan, snk, em, resources, signal)
		return
	}
	if err := snk.Commit(ctx); err != nil {
		em.failed(fmt.Errorf("commit sink %q: %w", spec.Sink.Provider, err), resources, false)
		abortSink(ctx, deps, spec, snk)
		return
	}
	em.completed(resources)
}

func prepareRunExecution(
	ctx context.Context,
	deps Deps,
	spec filament.RunSpec,
	em *emitter,
) (context.Context, *runControl, error) {
	if err := seedEmitterProgress(ctx, deps.DataStore, spec.Run, em); err != nil {
		return nil, nil, fmt.Errorf("restore run progress: %w", err)
	}
	return newRunControl(ctx, deps.Bus, deps.Log, spec.Tenant, spec.Run)
}

func seedEmitterProgress(ctx context.Context, ds filament.DataStore, run filament.RunID, em *emitter) error {
	if ds == nil {
		return nil
	}
	state, err := ds.LoadRun(ctx, run)
	if errors.Is(err, filament.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if state.Status == filament.RunPartial {
		return nil
	}
	em.seedProgress(state)
	return nil
}

func emitControlledIfStopped(ctx context.Context, err error, control *runControl, em *emitter) bool {
	signal := control.requested()
	return stoppedByControl(ctx, err, signal) && em.controlled(signal)
}

func stoppedByControl(ctx context.Context, err error, signal controlSignal) bool {
	if signal == controlNone || ctx.Err() == nil {
		return false
	}
	return err == nil || errors.Is(err, context.Canceled)
}

func finishControlled(
	ctx context.Context,
	deps Deps,
	spec filament.RunSpec,
	plan filament.IngestionPlan,
	sink filament.Sink,
	em *emitter,
	resources []string,
	signal controlSignal,
) {
	if signal == controlCancel {
		abortSink(ctx, deps, spec, sink)
		logControlBoundary(deps.Log, spec.Run, "cancel", "abort", em, 0)
		em.canceled()
		return
	}
	incremental, checkpointed := partitionCheckpointing(spec)
	checkpointResources := len(incremental) + len(checkpointed)
	checkpointCoverage := filament.CheckpointCoverageFor(spec.Resources, spec.IngestionTypes)
	action := "abort_and_restart"
	committed := plan.RequiresCDC || checkpointCoverage == filament.CheckpointCoverageAll
	if committed {
		if err := sink.Commit(ctx); err != nil {
			em.failed(fmt.Errorf("commit paused sink %q: %w", spec.Sink.Provider, err), resources, false)
			abortSink(ctx, deps, spec, sink)
			return
		}
		action = "commit_for_resume"
	} else {
		// A checkpoint-free read cannot resume from a partial sink, so discard it;
		// resume will safely restart the resource from the beginning.
		abortSink(ctx, deps, spec, sink)
	}
	logControlBoundary(deps.Log, spec.Run, "pause", action, em, checkpointResources)
	em.paused(committed)
}

func logControlBoundary(
	log filament.Logger,
	run filament.RunID,
	control, action string,
	em *emitter,
	checkpointResources int,
) {
	if log == nil {
		return
	}
	records, bytes := em.runTotals()
	log.Info("runner: control boundary reached",
		filament.Field{Key: "run", Value: string(run)},
		filament.Field{Key: "control", Value: control},
		filament.Field{Key: "sink_action", Value: action},
		filament.Field{Key: "checkpoint_resources", Value: checkpointResources},
		filament.Field{Key: "records", Value: records},
		filament.Field{Key: "bytes", Value: bytes},
	)
}

func abortSink(ctx context.Context, deps Deps, spec filament.RunSpec, sink filament.Sink) {
	abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), abortWait)
	defer cancel()
	if err := sink.Abort(abortCtx); err != nil && deps.Log != nil {
		deps.Log.Error("runner: sink abort", err, filament.Field{Key: "run", Value: string(spec.Run)})
	}
}
