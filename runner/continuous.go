package runner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/schema"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/galaxy-io/filament/telemetry"
)

// DefaultLeaseTTL bounds worker ownership between renewals.
const DefaultLeaseTTL = 2 * time.Minute

// DefaultDrainTimeout bounds graceful worker shutdown.
const DefaultDrainTimeout = 30 * time.Second

// ContinuousConfig configures an admitted serial attempt. RunOne handles only
// bounded runs. Write intent is resolved by the shared stream planner. Schemas default to the configured source.
type ContinuousConfig struct {
	messagePolicy  schema.Policy
	messageSchemas map[string]schema.MessageLayout
	Bus            eventbus.Bus
	Log            filament.Logger
	Metrics        *telemetry.Set
	events         *emitter
	pipelineEvents *continuousPipelineEvents
	Enabled        bool
	Spec           filament.RunSpec
	Store          filament.StreamRuntimeStore
	Source         filament.Source
	Sink           filament.Sink
	// NewSink supplies fresh instances for sinks without reusable sessions.
	NewSink                func() (filament.Sink, error)
	Codecs                 filament.CodecResolver
	Schemas                map[string]rowmodel.Schema
	Boundary               filament.Boundary
	LeaseTTL, DrainTimeout time.Duration
	// MaxEpochs bounds integration runs; zero runs until stopped or canceled.
	MaxEpochs int
}

// errShutdown ends an attempt whose worker was asked to stop during startup or
// a read.
var errShutdown = errors.New("worker shut down")

// continuousStop records why an attempt is ending. The heartbeat sets stopping
// and yielded, shutdown sets stopping and shutdown, and cleanup sets closing.
type continuousStop struct {
	stopping, yielded, shutdown, closing atomic.Bool
}

// watch returns the context for startup and source reads. Shutdown cancels it
// and leaves runCtx alone, so a commit in progress finishes.
func (s *continuousStop) watch(ctx, runCtx context.Context) (readCtx context.Context, release func()) {
	readCtx, stopReading := context.WithCancel(runCtx)
	unwatch := context.AfterFunc(ctx, func() {
		s.shutdown.Store(true)
		s.stopping.Store(true)
		stopReading()
	})
	return readCtx, func() {
		unwatch()
		stopReading()
	}
}

// interrupted maps an error from startup or a read. A cancellation that
// shutdown caused while the run was healthy ends the attempt cleanly. Any other
// error, such as a schema block, stands.
func (s *continuousStop) interrupted(runCtx context.Context, err error) error {
	if s.shutdown.Load() && runCtx.Err() == nil && errors.Is(err, context.Canceled) {
		return errShutdown
	}
	return err
}

// RunContinuous executes native epochs. The shared emitter publishes activity;
// runtime persistence, rather than event folding, owns continuous state.
// Providers must honor cancellation. Forced cancellation abandons tentative work.
// Cancelling ctx is a request to shut down: startup or the read in progress
// stops and its epoch is discarded, a commit in progress finishes, and the
// attempt ends clean.
func RunContinuous(ctx context.Context, cfg ContinuousConfig) (result error) {
	if err := validateContinuous(cfg); err != nil {
		return err
	}
	spec := cfg.Spec
	finishEvents := startContinuousEvents(ctx, &cfg)
	defer func() { finishEvents(result) }()
	lease := filament.LeaseToken{Tenant: spec.Tenant, Attempt: *spec.StreamAttempt}
	cleanupInstalled := false
	defer func() {
		if !cleanupInstalled {
			c, done := context.WithTimeout(context.WithoutCancel(ctx), cfg.DrainTimeout)
			defer done()
			result = errors.Join(result, cfg.Store.EndAttempt(c, filament.EndAttemptRequest{Lease: lease, Termination: filament.AttemptClean, Reason: continuousReason(result)}))
		}
	}()
	state, err := loadContinuousAttemptState(ctx, cfg, lease)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancelCause(observeSchemaEvents(context.WithoutCancel(ctx), cfg.events))
	defer cancel(nil)
	var stop continuousStop
	readCtx, release := stop.watch(ctx, runCtx)
	defer release()
	hb, err := startContinuousHeartbeat(ctx, cfg, lease, &stop, cancel)
	if err != nil {
		return err
	}
	var session filament.StreamSession
	var p *pipeline.Pipeline
	var activeEpoch *filament.EpochRef
	opened := false
	defer func() {
		c, done := context.WithTimeout(context.WithoutCancel(ctx), cfg.DrainTimeout)
		defer done()
		stop.closing.Store(true)
		result = closeContinuousPipeline(p, result)
		var closeErr error
		if activeEpoch != nil {
			closeErr = cfg.Sink.Abort(c)
		}
		if session != nil {
			closeErr = errors.Join(closeErr, session.Close(c))
		}
		if opened {
			closeErr = errors.Join(closeErr, cfg.Sink.Close(c))
		}
		closeErr = errors.Join(closeErr, cfg.Source.Teardown(c))
		result = finishContinuousAttempt(c, cfg, lease, result, closeErr, hb.Stop(c), &stop)
	}()
	cleanupInstalled = true
	emitContinuousStarted(cfg)
	opened = true // Close must also clean up a partially successful Open.
	plan, err := prepareContinuousConnectors(readCtx, &cfg)
	if err != nil {
		return stop.interrupted(runCtx, err)
	}
	spec = cfg.Spec
	if err := prepareContinuousSchemas(readCtx, &cfg, state); err != nil {
		return stop.interrupted(runCtx, err)
	}
	session, err = openContinuousStream(readCtx, cfg, filament.StreamOpenOpts{Tenant: spec.Tenant, MessageSchemaPolicy: cfg.messagePolicy, MessageSchemas: schema.CloneMessageLayouts(cfg.messageSchemas), ReportResourceError: continuousResourceErrorReporter(cfg, lease), SourceConnectionID: spec.SourceConnectionID, CheckAuthority: func(c context.Context) error { return cfg.Store.RenewLease(c, lease, cfg.LeaseTTL) }, Resources: spec.Resources, CommittedPositions: state.CommittedPositions.Clone(), Membership: state.Membership, MembershipRevision: state.MembershipRevision, Attempt: lease.Attempt})
	if err != nil {
		return stop.interrupted(runCtx, err)
	}
	p, err = newContinuousPipeline(&cfg, plan.Ordering)
	if err != nil {
		return err
	}
	p.Start(runCtx)
	return runEpochs(runCtx, readCtx, cfg, p, session, cfg.Sink, state, lease, &stop, &activeEpoch)
}

// closeContinuousPipeline joins the pipeline, which is nil when startup ended
// before it was built. Work cut short by shutdown discards its epoch as a
// failed read would, without failing the attempt. The pipeline then reports
// only that interruption: the error it was closed with, or the read's own
// cancellation when it was cut inside a barrier.
func closeContinuousPipeline(p *pipeline.Pipeline, result error) error {
	interrupted := errors.Is(result, errShutdown)
	if p == nil {
		if interrupted {
			return nil
		}
		return result
	}
	p.CloseIngest(result)
	err := p.Wait()
	if !interrupted {
		return errors.Join(result, err)
	}
	if errors.Is(err, errShutdown) || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func loadContinuousAttemptState(ctx context.Context, cfg ContinuousConfig, lease filament.LeaseToken) (filament.StreamState, error) {
	spec := cfg.Spec
	request := filament.StreamStateRequest{Tenant: spec.Tenant, Stream: filament.StreamRef{ID: lease.Attempt.StreamID, Generation: lease.Attempt.Generation}}
	state, err := cfg.Store.LoadStreamState(ctx, request)
	if err != nil {
		return filament.StreamState{}, err
	}
	if state.Attempt == nil || state.Attempt.Lease != lease || state.Desired != filament.StreamEnabled || state.PipelineVersionID != spec.PipelineVersionID || state.Run != spec.Run {
		return filament.StreamState{}, filament.ErrFenced
	}
	if err := cfg.Store.RenewLease(ctx, lease, cfg.LeaseTTL); err != nil {
		return filament.StreamState{}, err
	}
	return state, nil
}

// startContinuousHeartbeat renews ownership and observes pause, stop and yield
// independently of source reads and sink writes. Only the heartbeat task owns
// drainStart. A yield drains like a stop but records that this attempt yielded for an upgrade.
// The drain deadline bounds the epoch loop. Cleanup runs under its own deadline.
func startContinuousHeartbeat(ctx context.Context, cfg ContinuousConfig, lease filament.LeaseToken, stop *continuousStop, cancel context.CancelCauseFunc) (*streamkit.Heartbeat, error) {
	request := filament.StreamStateRequest{Tenant: lease.Tenant, Stream: filament.StreamRef{ID: lease.Attempt.StreamID, Generation: lease.Attempt.Generation}}
	var drainStart time.Time
	return streamkit.StartHeartbeat(context.WithoutCancel(ctx), min(cfg.LeaseTTL/3, time.Second), func(parent context.Context) error {
		c, done := context.WithTimeout(parent, cfg.LeaseTTL/3)
		defer done()
		err := cfg.Store.RenewLease(c, lease, cfg.LeaseTTL)
		if err == nil {
			var current filament.StreamState
			current, err = cfg.Store.LoadStreamState(c, request)
			yield := err == nil && current.Attempt != nil && current.Attempt.Lease == lease && current.Attempt.YieldRequested
			if yield {
				stop.yielded.Store(true)
			}
			if err == nil && (yield || stop.shutdown.Load() || current.Desired != filament.StreamEnabled) {
				stop.stopping.Store(true)
				if drainStart.IsZero() {
					drainStart = time.Now()
				}
				if !stop.closing.Load() && time.Since(drainStart) >= cfg.DrainTimeout {
					err = context.DeadlineExceeded
				}
			}
		}
		if err != nil {
			cancel(err)
		}
		return err
	})
}

func validateContinuous(cfg ContinuousConfig) error {
	if cfg.Sink == nil || cfg.Source == nil {
		return errors.New("continuous runner requires source and sink")
	}
	if !cfg.Sink.Spec().Capabilities.ReusableSession && cfg.NewSink == nil {
		return errors.New("continuous runner requires a factory for single-write sinks")
	}
	if !cfg.Enabled {
		return filament.ErrContinuousDisabled
	}
	spec := cfg.Spec
	if spec.Options.Execution.Normalize() != filament.ExecutionContinuous {
		return errors.New("continuous runner requires continuous execution")
	}
	if err := spec.ValidateStreamAttempt(); err != nil {
		return err
	}
	if _, _, err := filament.PlanContinuousRun(cfg.Source, cfg.Sink, spec); err != nil {
		return err
	}
	switch {
	case len(spec.Resources) == 0,
		cfg.Store == nil,
		cfg.Codecs == nil,
		cfg.MaxEpochs < 0,
		cfg.LeaseTTL < 30*time.Millisecond || cfg.LeaseTTL > 24*time.Hour,
		cfg.DrainTimeout <= 0:
		return errors.New("continuous runner: store, codecs, lease and drain limits required")
	case cfg.Boundary.MaxWait <= 0,
		cfg.Boundary.MaxRecords <= 0,
		cfg.Boundary.MaxBytes < 0,
		cfg.Boundary.MaxAge < 0:
		return fmt.Errorf("continuous runner: epoch boundary needs positive MaxWait and MaxRecords and non-negative MaxBytes and MaxAge, got %+v", cfg.Boundary)
	}
	for _, r := range spec.Resources {
		if _, ok := cfg.Schemas[r]; cfg.Schemas != nil && !ok {
			return fmt.Errorf("continuous runner: missing schema for %s", r)
		}
	}
	return nil
}

// publishCertified releases a certified epoch's held write facts, the bytes
// its sink wrote at commit, and a checkpoint fact per resource.
func publishCertified(cfg ContinuousConfig, dst filament.Sink, receipts []filament.EpochReceipt) {
	if cfg.events == nil {
		return
	}
	cfg.pipelineEvents.certified()
	publishCommitBytes(cfg.events, dst)
	for _, receipt := range receipts {
		emit(cfg.events, events.CheckpointSaved, receipt.Resource, events.CheckpointSavedEvent{})
	}
}

func runEpochs(runCtx, readCtx context.Context, cfg ContinuousConfig, p *pipeline.Pipeline, session filament.StreamSession, dst filament.Sink, state filament.StreamState, lease filament.LeaseToken, stop *continuousStop, activeEpoch **filament.EpochRef) error {
	spec := cfg.Spec
	var err error
	records := p.Records().(filament.StreamRecordSink)
	epoch := int64(1)
	if state.LastEpoch != nil {
		epoch = state.LastEpoch.Epoch + 1
	}
	for count := 0; cfg.MaxEpochs == 0 || count < cfg.MaxEpochs; count++ {
		if stop.stopping.Load() {
			return nil
		}
		if err := context.Cause(runCtx); err != nil {
			return err
		}
		ref := filament.EpochRef{Attempt: lease.Attempt, Epoch: epoch, MembershipRevision: state.MembershipRevision}
		if err := p.BindEpoch(ref); err != nil {
			return err
		}
		if err := dst.Begin(runCtx); err != nil {
			return err
		}
		*activeEpoch = &ref
		var coverage filament.Coverage
		for len(coverage.Positions) == 0 {
			coverage, err = session.Read(readCtx, records, cfg.Boundary)
			if err != nil {
				return stop.interrupted(runCtx, err)
			}
			if len(coverage.Claims) > 0 {
				return filament.ErrIncompleteCoverage
			}
			if stop.stopping.Load() && len(coverage.Positions) == 0 {
				return nil
			}
		}
		proof, err := p.SealEpoch()
		if err != nil {
			return err
		}
		commit := messageEpochCommit(cfg, ref, coverage, proof)
		// Check source coverage before making destination effects durable.
		// SealEpoch has verified every Apply receipt. Commit confirms durability
		// for those exact writes; source epoch identity stays inside the runner.
		for resource, total := range proof.Resources() {
			commit.Certificate.Receipts = append(commit.Certificate.Receipts, filament.EpochReceipt{Resource: resource, Rows: total.Rows, Bytes: total.Bytes})
		}
		if err := commit.ValidateCompletion(cfg.Codecs); err != nil {
			return err
		}
		err = dst.Commit(runCtx)
		if err != nil {
			return err
		}
		*activeEpoch = nil
		if _, err = cfg.Store.CommitEpoch(runCtx, commit); err != nil {
			historical, lookupErr := cfg.Store.GetEpoch(runCtx, filament.EpochLookup{Tenant: spec.Tenant, Key: ref.Key()})
			if lookupErr != nil {
				return errors.Join(err, lookupErr)
			}
			expected, e1 := commit.Certificate.CanonicalBytes(cfg.Codecs)
			actual, e2 := historical.Certificate.CanonicalBytes(cfg.Codecs)
			if e1 != nil || e2 != nil || !bytes.Equal(expected, actual) {
				return errors.Join(filament.ErrEpochConflict, e1, e2)
			}
		}
		// Publish only certified progress, never tentative Apply results. A lost
		// commit response verified above has the same durable outcome.
		publishCertified(cfg, dst, commit.Certificate.Receipts)
		// Historical lookup alone does not authorize acknowledgement.
		if err := cfg.Store.RenewLease(runCtx, lease, cfg.LeaseTTL); err != nil {
			return err
		}
		if err := session.Acknowledge(runCtx, coverage); err != nil {
			return err
		}
		// Certifying a baseline admits its members and moves the membership
		// revision, which the next epoch must carry.
		if len(coverage.Activated) > 0 {
			state, err = cfg.Store.LoadStreamState(runCtx, filament.StreamStateRequest{Tenant: spec.Tenant, Stream: filament.StreamRef{ID: lease.Attempt.StreamID, Generation: lease.Attempt.Generation}})
			if err != nil {
				return err
			}
		}
		epoch++
	}
	return nil
}

func ensureContinuousSchemas(runCtx context.Context, cfg *ContinuousConfig) error {
	if cfg.messagePolicy != "" {
		return nil
	} // Message schemas are complete only after the epoch probe.
	if schemaSink, ok := cfg.Sink.(filament.Schematized); ok {
		audit := continuousAudit(cfg.Spec)
		for _, resource := range cfg.Spec.Resources {
			shaped, err := auditSchema(cfg.Schemas[resource], audit.CDC, audit.CDCAppend)
			if err != nil {
				return err
			}
			destination, model := destinationSchema(resource, shaped, cfg.Spec.WritePolicies)
			if err := schemaSink.EnsureSchema(runCtx, destination, model); err != nil {
				return err
			}
		}
	}
	return nil
}

func continuousReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, schema.ErrBlocked) {
		return schema.ErrBlocked.Error() + ": " + err.Error()
	}
	return err.Error()
}

func prepareContinuousConnectors(ctx context.Context, cfg *ContinuousConfig) (filament.IngestionPlan, error) {
	if err := cfg.Source.Configure(ctx, filament.NewConfig(cfg.Spec.Source.Config)); err != nil {
		return filament.IngestionPlan{}, err
	}
	if err := SelectColumns(cfg.Source, cfg.Spec.Columns); err != nil {
		return filament.IngestionPlan{}, err
	}
	if err := SelectPartitions(cfg.Source, cfg.Spec.Partitions); err != nil {
		return filament.IngestionPlan{}, err
	}
	plan, err := filament.ResolveIngestionPlan(ctx, cfg.Source, cfg.Sink, cfg.Spec)
	if err != nil {
		return filament.IngestionPlan{}, err
	}
	cfg.Spec.WritePolicies = plan.WritePolicies
	sinkSpec, schemas, err := prepareSinkResources(ctx, cfg.Source, cfg.Sink, &cfg.Spec, cfg.Schemas)
	if err != nil {
		return filament.IngestionPlan{}, err
	}
	cfg.Schemas = schemas
	cfg.Sink = newBoundarySink(cfg.Sink, cfg.NewSink)
	if err := cfg.Sink.Open(ctx, sinkSpec); err != nil {
		return filament.IngestionPlan{}, err
	}
	return plan, nil
}

func continuousResourceErrorReporter(cfg ContinuousConfig, lease filament.LeaseToken) func(context.Context, string, error) error {
	var reportResourceError func(context.Context, string, error) error
	if reporter, ok := cfg.Store.(filament.StreamResourceErrorStore); ok {
		reportResourceError = func(ctx context.Context, resource string, issue error) error {
			message := ""
			if issue != nil {
				message = issue.Error()
			}
			if err := reporter.SetStreamResourceError(ctx, lease, resource, message); err != nil {
				return err
			}
			if issue != nil && cfg.events != nil {
				emit(cfg.events, events.ResourceFailed, resource, events.ResourceFailedEvent{Error: message})
			}
			return nil
		}
	}
	return reportResourceError
}

// continuousAudit gives a stream the same lineage as a bounded run, so its
// rows carry the run, and for CDC the operation, position and sequence.
func continuousAudit(spec filament.RunSpec) pipeline.AuditConfig {
	return pipeline.AuditConfig{RunStartedAt: startedAtFor(spec), CDC: filament.IsCDC(spec.IngestionTypes), CDCAppend: filament.IsCDCAppend(spec.IngestionTypes)}
}

func newContinuousPipeline(cfg *ContinuousConfig, ordering filament.Ordering) (*pipeline.Pipeline, error) {
	spec := cfg.Spec
	audit := continuousAudit(spec)
	pipelineConfig := pipeline.Config{MessageSchemaPolicy: cfg.messagePolicy, MessageSchemas: cfg.messageSchemas, Tenant: spec.Tenant, Run: spec.Run, Sink: cfg.Sink, WritePolicies: spec.WritePolicies, Options: spec.Options, Log: cfg.Log, Audit: &audit}
	if schemaSink, ok := cfg.Sink.(filament.Schematized); ok {
		// The pipeline hands over the shaped layout, so compare against the
		// planned schema shaped the same way.
		pipelineConfig.PrepareSchema = func(ctx context.Context, resource string, schema rowmodel.Schema) error {
			if planned, ok := cfg.Schemas[resource]; cfg.messagePolicy == "" && ok {
				shaped, err := auditSchema(planned, audit.CDC, audit.CDCAppend)
				if err != nil {
					return err
				}
				if shaped.Equal(schema) {
					return nil
				}
			}
			destination, actual := destinationSchema(resource, schema, spec.WritePolicies)
			return schemaSink.EnsureSchema(ctx, destination, actual)
		}
	}
	if cfg.events != nil {
		cfg.pipelineEvents = &continuousPipelineEvents{emitter: cfg.events}
		pipelineConfig.Emit = cfg.pipelineEvents.observe
	}
	return pipeline.NewStream(pipelineConfig, ordering)
}

func emitContinuousStarted(cfg ContinuousConfig) {
	spec := cfg.Spec
	if cfg.events != nil {
		emit(cfg.events, events.RunStarted, "", events.RunStartedEvent{})
		for _, resource := range spec.Resources {
			emit(cfg.events, events.ResourceStarted, resource, events.ResourceStartedEvent{})
		}
	}
}

func startContinuousEvents(ctx context.Context, cfg *ContinuousConfig) func(error) {
	if cfg.Bus == nil {
		return func(error) {}
	}
	cfg.events = newEmitter(ctx, cfg.Bus, cfg.Log, cfg.Spec.Tenant, cfg.Spec.Run)
	cfg.events.measure(cfg.Metrics, cfg.Spec.PipelineID)
	return func(result error) {
		defer cfg.events.finish()()
		cfg.events.schemaBlocked(result)
		emit(cfg.events, events.StreamAttemptEnded, "", events.StreamAttemptEndedEvent{Error: continuousReason(result)})
	}
}

// finishContinuousAttempt records how the attempt ended. A heartbeat that
// failed already stopped the run and says nothing about closure. One that is
// still running leaves closure unproven.
func finishContinuousAttempt(ctx context.Context, cfg ContinuousConfig, lease filament.LeaseToken, result, cleanupErr, heartbeatErr error, stop *continuousStop) error {
	if errors.Is(heartbeatErr, streamkit.ErrHeartbeatRunning) {
		cleanupErr = errors.Join(cleanupErr, heartbeatErr)
		heartbeatErr = nil
	}
	termination := filament.AttemptClean
	if cleanupErr != nil {
		termination = filament.AttemptUnproven
	}
	reason := continuousReason(result)
	switch {
	case reason != "":
	case stop.yielded.Load():
		reason = filament.AttemptReasonUpgrade
	case stop.shutdown.Load():
		reason = filament.AttemptReasonShutdown
	}
	endErr := cfg.Store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: lease, Termination: termination, Reason: reason})
	err := errors.Join(result, cleanupErr, endErr)
	if err == nil {
		return nil
	}
	return errors.Join(err, heartbeatErr)
}
