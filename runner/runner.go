// Package runner executes one Filament ingestion run.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/pipeline"
)

// Deps are the process-local dependencies needed to execute one run.
type Deps struct {
	Bus       eventbus.Bus
	DataStore filament.DataStore
	Secrets   filament.Secrets
	Sources   filament.SourceRegistry
	Sinks     filament.SinkRegistry
	Log       filament.Logger
}

// SpecFromState builds the RunSpec to execute from a persisted run state.
func SpecFromState(s filament.RunState) filament.RunSpec {
	r := s.Request
	return filament.RunSpec{
		Tenant: r.Tenant, Run: s.Run,
		PipelineID: r.PipelineID, PipelineVersionID: r.PipelineVersionID,
		CheckpointRoute: r.CheckpointRoute, CursorConfigs: r.CursorConfigs,
		Source: r.Source, Sink: r.Sink, Resources: r.Resources, Selectors: r.Selectors,
		IngestionType: r.IngestionType.OrDefault(), Mode: filament.ModeFull, Options: r.Options,
	}
}

// RunOne executes a single extraction end-to-end: resolve providers, open the
// sink, drive the pipeline with the source, then commit or abort. It publishes
// run.started first and exactly one terminal fact (run.completed | run.failed |
// run.partial).
//
//nolint:funlen // the run lifecycle reads best as one sequence
func RunOne(ctx context.Context, deps Deps, spec filament.RunSpec) {
	em := newEmitter(ctx, deps.Bus, deps.Log, spec.Tenant, spec.Run)
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

	if err := ensureSchemas(ctx, src, snk, spec); err != nil {
		if aerr := snk.Abort(ctx); aerr != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", aerr, filament.Field{Key: "run", Value: string(spec.Run)})
		}
		em.fail(fmt.Errorf("ensure schema: %w", err))
		return
	}

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

	extractErrCh := make(chan error, 1)
	go func() {
		extractErrCh <- safeCall(func() error {
			return extractor(ctx, p.Records(), filament.ExtractOpts{
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

	runErr := waitErr
	if runErr == nil {
		runErr = extractErr
	}

	resources := resolveResources(spec.Resources, em.seenResources())

	if runErr != nil {
		if isResumableRun(plan) {
			for _, res := range resources {
				emit(em, events.ResourceFailed, res, events.ResourceFailedEvent{Error: runErr.Error()})
			}
			em.partial(runErr)
			return
		}
		if err := snk.Abort(ctx); err != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", err, filament.Field{Key: "run", Value: string(spec.Run)})
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

// ResolveConfigRefs resolves opaque config and field references into the
// process-local RunSpec copy immediately before connector configuration.
func ResolveConfigRefs(ctx context.Context, secrets filament.Secrets, spec *filament.RunSpec) error {
	if err := resolveRefConfig(ctx, secrets, &spec.Source, spec.Tenant, "source"); err != nil {
		return err
	}
	if err := resolveRefConfig(ctx, secrets, &spec.Sink, spec.Tenant, "sink"); err != nil {
		return err
	}
	if err := resolveSecretRefs(ctx, secrets, &spec.Source, spec.Tenant, "source"); err != nil {
		return err
	}
	if err := resolveSecretRefs(ctx, secrets, &spec.Sink, spec.Tenant, "sink"); err != nil {
		return err
	}
	return nil
}

// resolveSecretRefs reads each of the ref's declared secrets and injects the
// plaintext value into the provider config under the mapped field. Every ref is
// tenant-scoped first, so a spec cannot read another tenant's connection secrets.
func resolveSecretRefs(ctx context.Context, secrets filament.Secrets, ref *filament.Ref, tenant filament.TenantID, role string) error {
	if len(ref.SecretRefs) == 0 {
		return nil
	}
	if secrets == nil {
		return fmt.Errorf("%s %q has secret refs but no secrets store is configured", role, ref.Provider)
	}
	if ref.Config == nil {
		ref.Config = make(map[string]any, len(ref.SecretRefs))
	}
	for field, name := range ref.SecretRefs {
		if err := filament.ValidateConnectionSecretRef(name, tenant); err != nil {
			return fmt.Errorf("resolve %s secret for field %q: %w", role, field, err)
		}
		secret, err := secrets.Read(ctx, name)
		if err != nil {
			return fmt.Errorf("resolve %s secret %q for field %q: %w", role, name, field, err)
		}
		setConfigPath(ref.Config, field, string(secret.Value))
	}
	return nil
}

// setConfigPath assigns value at a dotted field path, rebuilding objects that
// were removed when their only submitted values were secrets.
func setConfigPath(cfg map[string]any, path string, value any) {
	parts := strings.Split(path, ".")
	current := cfg
	for _, part := range parts[:len(parts)-1] {
		nested, ok := current[part].(map[string]any)
		if !ok {
			nested = make(map[string]any)
			current[part] = nested
		}
		current = nested
	}
	current[parts[len(parts)-1]] = value
}

func resolveRefConfig(ctx context.Context, secrets filament.Secrets, ref *filament.Ref, tenant filament.TenantID, role string) error {
	if ref.ConfigRef == "" || len(ref.Config) > 0 {
		return nil
	}
	if secrets == nil {
		return fmt.Errorf("%s %q has config ref %q but no secrets store is configured", role, ref.Provider, ref.ConfigRef)
	}
	if err := filament.ValidateConnectionSecretRef(ref.ConfigRef, tenant); err != nil {
		return fmt.Errorf("read %s config ref: %w", role, err)
	}
	secret, err := secrets.Read(ctx, ref.ConfigRef)
	if err != nil {
		return fmt.Errorf("read %s config ref %q: %w", role, ref.ConfigRef, err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(secret.Value, &cfg); err != nil {
		return fmt.Errorf("decode %s config ref %q as JSON object: %w", role, ref.ConfigRef, err)
	}
	ref.Config = cfg
	return nil
}

type extractorFunc func(context.Context, filament.RecordSink, filament.ExtractOpts) error

func resolveExtractor(ctx context.Context, ds filament.DataStore, src filament.Source, spec filament.RunSpec, plan filament.IngestionPlan) (extractorFunc, error) {
	if plan.Type == filament.IngestionCDC {
		changes, ok := src.(filament.ChangeSource)
		if !ok {
			return nil, fmt.Errorf("source %q does not support CDC extraction", spec.Source.Provider)
		}
		checkpoints, err := loadChangeCheckpoints(ctx, ds, spec)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			return changes.ExtractChanges(ctx, sink, filament.ChangeExtractOpts{
				Resources:   opts.Resources,
				Checkpoints: checkpoints,
				Limit:       opts.Limit,
			})
		}, nil
	}
	if isResumableRun(plan) {
		resumable, ok := src.(filament.Resumable)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Provider)
		}
		incremental := plan.SourcePolicy.Mode == filament.ModeIncremental
		prev := make(map[string]filament.Checkpoint, len(spec.Resources))
		for _, resource := range spec.Resources {
			var cp filament.Checkpoint
			var err error
			if incremental {
				key, valid := spec.ResourceCheckpointKey(resource)
				if !valid {
					return nil, fmt.Errorf("incremental resource %q requires a versioned pipeline route", resource)
				}
				var state filament.ResourceCheckpointState
				state, err = ds.LoadResourceCheckpoint(ctx, key)
				cp = state.Checkpoint
			} else {
				cp, err = ds.LoadCheckpoint(ctx, spec.Run, resource)
			}
			if err == nil {
				prev[resource] = cp
			} else if err != nil && !errors.Is(err, filament.ErrNotFound) {
				return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
			}
		}
		var resumePlan map[string]filament.Checkpoint
		var err error
		if incremental {
			planner, ok := src.(filament.IncrementalPlanner)
			if !ok {
				return nil, fmt.Errorf("source %q does not support incremental planning", spec.Source.Provider)
			}
			resumePlan, err = planner.PlanIncremental(ctx, spec.Resources, prev, spec.CursorConfigs)
		} else {
			planner, ok := src.(filament.ResumePlanner)
			if !ok {
				return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Provider)
			}
			resumePlan, err = planner.PlanResume(ctx, spec.Resources, prev)
		}
		if err != nil {
			return nil, fmt.Errorf("plan resume: %w", err)
		}
		for _, cp := range resumePlan {
			if cp == nil {
				continue
			}
			var saveErr error
			if incremental {
				key, _ := spec.ResourceCheckpointKey(cp.Resource())
				saveErr = ds.SaveResourceCheckpoint(ctx, filament.ResourceCheckpointState{Key: key, Run: spec.Run, Checkpoint: cp})
			} else {
				saveErr = ds.SaveCheckpoint(ctx, spec.Run, cp)
			}
			if saveErr != nil {
				return nil, fmt.Errorf("seed checkpoint %q: %w", cp.Resource(), saveErr)
			}
		}
		return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
			return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
		}, nil
	}
	return func(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
		return src.Extract(ctx, sink, opts)
	}, nil
}

func loadChangeCheckpoints(ctx context.Context, ds filament.DataStore, spec filament.RunSpec) (map[string]filament.Checkpoint, error) {
	out := make(map[string]filament.Checkpoint, len(spec.Resources))
	for _, resource := range spec.Resources {
		var cp filament.Checkpoint
		var err error
		if key, ok := spec.ResourceCheckpointKey(resource); ok {
			var state filament.ResourceCheckpointState
			state, err = ds.LoadResourceCheckpoint(ctx, key)
			cp = state.Checkpoint
		} else {
			cp, err = ds.LoadCheckpoint(ctx, spec.Run, resource)
		}
		if err == nil {
			out[resource] = cp
			continue
		}
		if !errors.Is(err, filament.ErrNotFound) {
			return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func isResumableRun(plan filament.IngestionPlan) bool {
	return plan.SourcePolicy.Checkpointing != filament.CheckpointNone && !plan.RequiresCDC
}

func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("source panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}

func resolveIngestionPlan(ctx context.Context, src filament.Source, snk filament.Sink, spec filament.RunSpec) (filament.IngestionPlan, error) {
	ingestionType := spec.IngestionType.OrDefault()
	sourcePolicy := filament.SourcePolicyForIngestion(ingestionType)
	writePolicy := filament.WritePolicyForIngestion(ingestionType)

	if err := validateSourcePolicy(src.Spec(), ingestionType, sourcePolicy); err != nil {
		return filament.IngestionPlan{}, err
	}
	if err := validateSinkPolicy(snk, writePolicy); err != nil {
		return filament.IngestionPlan{}, err
	}

	policies := map[string]filament.WritePolicy{}
	if len(spec.Resources) == 0 {
		policies[""] = writePolicy
	} else {
		for _, resource := range spec.Resources {
			policy := writePolicy
			policy.Resource = resource
			if policy.Capability.RequiresPK {
				keys, err := primaryKeyForResource(ctx, src, resource)
				if err != nil {
					return filament.IngestionPlan{}, err
				}
				if len(keys) == 0 {
					return filament.IngestionPlan{}, fmt.Errorf("%s requested for resource %q but no primary key was discovered", ingestionType, resource)
				}
				policy.Keys = keys
			}
			policies[resource] = policy
		}
	}

	return filament.IngestionPlan{
		Type:          ingestionType,
		SourcePolicy:  sourcePolicy,
		WritePolicies: policies,
		RequiresCDC:   ingestionType == filament.IngestionCDC,
		RequiresPK:    writePolicy.Capability.RequiresPK,
	}, nil
}

func validateSourcePolicy(spec filament.ConnectorSpec, ingestionType filament.IngestionType, policy filament.SourcePolicy) error {
	for _, candidate := range spec.SourcePolicies {
		if candidate.Mode == policy.Mode && acceptsOperations(candidate.EmitsOps, policy.EmitsOps) && (!policy.Ordered || candidate.Ordered) {
			return nil
		}
	}
	if len(spec.SourcePolicies) > 0 {
		return fmt.Errorf("source %q does not support ingestion type %q", spec.Name, ingestionType)
	}
	for _, mode := range spec.Modes {
		if mode == policy.Mode {
			return nil
		}
	}
	return fmt.Errorf("source %q does not support replication mode %v required by ingestion policy", spec.Name, policy.Mode)
}

func validateSinkPolicy(snk filament.Sink, policy filament.WritePolicy) error {
	spec := snk.Spec()
	for _, candidate := range spec.Capabilities.WritePolicies {
		if candidate.Mode == policy.Capability.Mode && (!policy.Capability.RequiresPK || candidate.RequiresPK) &&
			(!policy.Capability.RequiresOrder || candidate.RequiresOrder) && acceptsOperations(candidate.AcceptsOps, policy.Capability.AcceptsOps) {
			return nil
		}
	}
	switch policy.Capability.Mode {
	case filament.WriteAppend, filament.WriteReplace:
		return nil
	case filament.WriteUpsert:
		if spec.Capabilities.Upsertable {
			return nil
		}
	}
	return fmt.Errorf("sink %q does not support write policy %q", spec.Name, policy.Capability.Mode)
}

func primaryKeyForResource(ctx context.Context, src filament.Source, resource string) ([]string, error) {
	if schemas, ok := src.(filament.SchemaProvider); ok {
		schema, err := schemas.Schema(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("schema for %q: %w", resource, err)
		}
		return schema.PrimaryKey, nil
	}
	if discoverable, ok := src.(filament.Discoverable); ok {
		result, err := discoverable.Discover(ctx, filament.DiscoverOpts{})
		if err != nil {
			return nil, fmt.Errorf("discover resources: %w", err)
		}
		for _, candidate := range result.Resources {
			if candidate.Name == resource {
				return candidate.PrimaryKey, nil
			}
		}
	}
	return nil, nil
}

func acceptsOperations(have, want []filament.Operation) bool {
	if len(want) == 0 {
		return true
	}
	if len(have) == 0 {
		return false
	}
	set := make(map[filament.Operation]bool, len(have))
	for _, op := range have {
		set[op] = true
	}
	for _, op := range want {
		if !set[op] {
			return false
		}
	}
	return true
}

func ensureSchemas(ctx context.Context, src filament.Source, snk filament.Sink, spec filament.RunSpec) error {
	sch, ok := snk.(filament.Schematized)
	if !ok {
		return nil
	}
	prov, ok := src.(filament.SchemaProvider)
	if !ok {
		return fmt.Errorf("sink %q requires a schema but source %q provides none", spec.Sink.Provider, spec.Source.Provider)
	}
	for _, res := range spec.Resources {
		schema, err := prov.Schema(ctx, res)
		if err != nil {
			return fmt.Errorf("schema for %q: %w", res, err)
		}
		if err := sch.EnsureSchema(ctx, res, schema); err != nil {
			return fmt.Errorf("ensure schema for %q: %w", res, err)
		}
	}
	return nil
}

type emitter struct {
	ctx    context.Context
	bus    eventbus.Bus
	log    filament.Logger
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

func (e *emitter) next() uint64 { return e.seq.Add(1) }

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
		e.log.Info(f.Name, fields...)
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

// emit stamps and publishes a runner-originated fact for a run or resource.
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

func (e *emitter) fail(err error) {
	if e.log != nil {
		e.log.Error("runner: run failed", err, filament.Field{Key: "run", Value: string(e.run)})
	}
	emit(e, events.RunFailed, "", events.RunFailedEvent{Error: err.Error()})
}

func (e *emitter) partial(err error) {
	if e.log != nil {
		e.log.Error("runner: run partial", err, filament.Field{Key: "run", Value: string(e.run)})
	}
	emit(e, events.RunPartial, "", events.RunPartialEvent{Error: err.Error()})
}

func (e *emitter) resourceTally(name string) (records, bytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if t := e.res[name]; t != nil {
		return t.records, t.bytes
	}
	return 0, 0
}

func (e *emitter) runTotals() (records, bytes int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.runRecords, e.runBytes
}

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

func resolveResources(requested, seen []string) []string {
	if len(requested) > 0 {
		return requested
	}
	return seen
}
