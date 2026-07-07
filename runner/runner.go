// Package runner executes one Filament ingestion run.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/pipeline"
)

// Deps are the process-local dependencies needed to execute one run.
type Deps struct {
	Bus       eventbus.Bus
	DataStore ingestion.DataStore
	Secrets   ingestion.Secrets
	Sources   ingestion.SourceRegistry
	Sinks     ingestion.SinkRegistry
	Log       ingestion.Logger
}

// SpecFromState builds the RunSpec to execute from a persisted run state.
func SpecFromState(s ingestion.RunState) ingestion.RunSpec {
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

// RunOne executes a single extraction end-to-end: resolve providers, open the
// sink, drive the pipeline with the source, then commit or abort. It publishes
// run.started first and exactly one terminal fact (run.completed | run.failed |
// run.partial).
func RunOne(ctx context.Context, deps Deps, spec ingestion.RunSpec) {
	em := newEmitter(ctx, deps.Bus, deps.Log, spec.Tenant, spec.Run)
	em.fact(ingestion.EvRunStarted, ingestion.EventFields{})

	if err := resolveConfigRefs(ctx, deps.Secrets, &spec); err != nil {
		em.fail(err)
		return
	}

	src, err := deps.Sources.Resolve(spec.Source.Provider)
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
			deps.Log.Error("runner: sink abort", aerr, ingestion.Field{Key: "run", Value: string(spec.Run)})
		}
		em.fail(fmt.Errorf("ensure schema: %w", err))
		return
	}

	for _, res := range spec.Resources {
		em.emitFact(ingestion.EvResourceStarted, res, ingestion.EventFields{})
	}

	extractor, err := resolveExtractor(ctx, deps.DataStore, src, spec, plan)
	if err != nil {
		if aerr := snk.Abort(ctx); aerr != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", aerr, ingestion.Field{Key: "run", Value: string(spec.Run)})
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

	runErr := waitErr
	if runErr == nil {
		runErr = extractErr
	}

	resources := resolveResources(spec.Resources, em.seenResources())

	if runErr != nil {
		if isResumableRun(spec, plan) {
			for _, res := range resources {
				em.emitFact(ingestion.EvResourceFailed, res, ingestion.EventFields{Error: runErr.Error()})
			}
			em.partial(runErr)
			return
		}
		if err := snk.Abort(ctx); err != nil && deps.Log != nil {
			deps.Log.Error("runner: sink abort", err, ingestion.Field{Key: "run", Value: string(spec.Run)})
		}
		for _, res := range resources {
			em.emitFact(ingestion.EvResourceFailed, res, ingestion.EventFields{Error: runErr.Error()})
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
		em.emitFact(ingestion.EvResourceCompleted, res, ingestion.EventFields{Records: records, Bytes: bytes})
	}
	records, bytes := em.runTotals()
	em.fact(ingestion.EvRunCompleted, ingestion.EventFields{Records: records, Bytes: bytes})
}

func resolveConfigRefs(ctx context.Context, secrets ingestion.Secrets, spec *ingestion.RunSpec) error {
	if err := resolveRefConfig(ctx, secrets, &spec.Source, "source"); err != nil {
		return err
	}
	if err := resolveRefConfig(ctx, secrets, &spec.Sink, "sink"); err != nil {
		return err
	}
	if err := resolveSecretRefs(ctx, secrets, &spec.Source, "source"); err != nil {
		return err
	}
	if err := resolveSecretRefs(ctx, secrets, &spec.Sink, "sink"); err != nil {
		return err
	}
	return nil
}

// resolveSecretRefs reads each of the ref's declared secrets and injects the
// plaintext value into the provider config under the mapped field.
func resolveSecretRefs(ctx context.Context, secrets ingestion.Secrets, ref *ingestion.Ref, role string) error {
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
		secret, err := secrets.Read(ctx, name)
		if err != nil {
			return fmt.Errorf("resolve %s secret %q for field %q: %w", role, name, field, err)
		}
		ref.Config[field] = string(secret.Value)
	}
	return nil
}

func resolveRefConfig(ctx context.Context, secrets ingestion.Secrets, ref *ingestion.Ref, role string) error {
	if ref.ConfigRef == "" || len(ref.Config) > 0 {
		return nil
	}
	if secrets == nil {
		return fmt.Errorf("%s %q has config ref %q but no secrets store is configured", role, ref.Provider, ref.ConfigRef)
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

type extractorFunc func(context.Context, ingestion.RecordSink, ingestion.ExtractOpts) error

func resolveExtractor(ctx context.Context, ds ingestion.DataStore, src ingestion.Source, spec ingestion.RunSpec, plan ingestion.IngestionPlan) (extractorFunc, error) {
	if plan.Type == ingestion.IngestionCDC {
		changes, ok := src.(ingestion.ChangeSource)
		if !ok {
			return nil, fmt.Errorf("source %q does not support CDC extraction", spec.Source.Provider)
		}
		checkpoints, err := loadChangeCheckpoints(ctx, ds, spec)
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
			return changes.ExtractChanges(ctx, sink, ingestion.ChangeExtractOpts{
				Resources:   opts.Resources,
				Checkpoints: checkpoints,
				Limit:       opts.Limit,
			})
		}, nil
	}
	if isResumableRun(spec, plan) {
		planner, ok := src.(ingestion.ResumePlanner)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable planning", spec.Source.Provider)
		}
		resumable, ok := src.(ingestion.Resumable)
		if !ok {
			return nil, fmt.Errorf("source %q does not support resumable extraction", spec.Source.Provider)
		}
		prev := make(map[string]ingestion.Checkpoint, len(spec.Resources))
		for _, resource := range spec.Resources {
			cp, err := ds.LoadCheckpoint(ctx, spec.Run, resource)
			if err == nil {
				prev[resource] = cp
			} else if err != nil && !errors.Is(err, ingestion.ErrNotFound) {
				return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
			}
		}
		resumePlan, err := planner.PlanResume(ctx, spec.Resources, prev)
		if err != nil {
			return nil, fmt.Errorf("plan resume: %w", err)
		}
		for _, cp := range resumePlan {
			if cp == nil {
				continue
			}
			if err := ds.SaveCheckpoint(ctx, spec.Run, cp); err != nil {
				return nil, fmt.Errorf("seed checkpoint %q: %w", cp.Resource(), err)
			}
		}
		return func(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
			return resumable.ExtractFrom(ctx, sink, opts, resumePlan)
		}, nil
	}
	return func(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
		return src.Extract(ctx, sink, opts)
	}, nil
}

func loadChangeCheckpoints(ctx context.Context, ds ingestion.DataStore, spec ingestion.RunSpec) (map[string]ingestion.Checkpoint, error) {
	out := make(map[string]ingestion.Checkpoint, len(spec.Resources))
	for _, resource := range spec.Resources {
		cp, err := ds.LoadCheckpoint(ctx, spec.Run, resource)
		if err == nil {
			out[resource] = cp
			continue
		}
		if !errors.Is(err, ingestion.ErrNotFound) {
			return nil, fmt.Errorf("load checkpoint %q: %w", resource, err)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func isResumableRun(spec ingestion.RunSpec, plan ingestion.IngestionPlan) bool {
	return plan.Type == ingestion.IngestionSnapshotUpsert
}

func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("source panicked: %v\n%s", r, debug.Stack())
		}
	}()
	return fn()
}

func resolveIngestionPlan(ctx context.Context, src ingestion.Source, snk ingestion.Sink, spec ingestion.RunSpec) (ingestion.IngestionPlan, error) {
	ingestionType := spec.IngestionType.OrDefault()
	sourcePolicy := ingestion.SourcePolicyForIngestion(ingestionType)
	writePolicy := ingestion.WritePolicyForIngestion(ingestionType)

	if err := validateSourcePolicy(src.Spec(), sourcePolicy); err != nil {
		return ingestion.IngestionPlan{}, err
	}
	if err := validateSinkPolicy(snk, writePolicy); err != nil {
		return ingestion.IngestionPlan{}, err
	}

	policies := map[string]ingestion.WritePolicy{}
	if len(spec.Resources) == 0 {
		policies[""] = writePolicy
	} else {
		for _, resource := range spec.Resources {
			policy := writePolicy
			policy.Resource = resource
			if policy.Capability.RequiresPK {
				keys, err := primaryKeyForResource(ctx, src, resource)
				if err != nil {
					return ingestion.IngestionPlan{}, err
				}
				if len(keys) == 0 {
					return ingestion.IngestionPlan{}, fmt.Errorf("%s requested for resource %q but no primary key was discovered", ingestionType, resource)
				}
				policy.Keys = keys
			}
			policies[resource] = policy
		}
	}

	return ingestion.IngestionPlan{
		Type:          ingestionType,
		SourcePolicy:  sourcePolicy,
		WritePolicies: policies,
		RequiresCDC:   ingestionType == ingestion.IngestionCDC,
		RequiresPK:    writePolicy.Capability.RequiresPK,
	}, nil
}

func validateSourcePolicy(spec ingestion.ConnectorSpec, policy ingestion.SourcePolicy) error {
	for _, candidate := range spec.SourcePolicies {
		if candidate.Mode == policy.Mode && acceptsOperations(candidate.EmitsOps, policy.EmitsOps) && (!policy.Ordered || candidate.Ordered) {
			return nil
		}
	}
	for _, mode := range spec.Modes {
		if mode == policy.Mode {
			return nil
		}
	}
	return fmt.Errorf("source %q does not support replication mode %v required by ingestion policy", spec.Name, policy.Mode)
}

func validateSinkPolicy(snk ingestion.Sink, policy ingestion.WritePolicy) error {
	spec := snk.Spec()
	for _, candidate := range spec.Capabilities.WritePolicies {
		if candidate.Mode == policy.Capability.Mode && (!policy.Capability.RequiresPK || candidate.RequiresPK) &&
			(!policy.Capability.RequiresOrder || candidate.RequiresOrder) && acceptsOperations(candidate.AcceptsOps, policy.Capability.AcceptsOps) {
			return nil
		}
	}
	switch policy.Capability.Mode {
	case ingestion.WriteAppend, ingestion.WriteReplace:
		return nil
	case ingestion.WriteUpsert:
		if spec.Capabilities.Upsertable {
			return nil
		}
	}
	return fmt.Errorf("sink %q does not support write policy %q", spec.Name, policy.Capability.Mode)
}

func primaryKeyForResource(ctx context.Context, src ingestion.Source, resource string) ([]string, error) {
	if schemas, ok := src.(ingestion.SchemaProvider); ok {
		schema, err := schemas.Schema(ctx, resource)
		if err != nil {
			return nil, fmt.Errorf("schema for %q: %w", resource, err)
		}
		return schema.PrimaryKey, nil
	}
	if discoverable, ok := src.(ingestion.Discoverable); ok {
		result, err := discoverable.Discover(ctx, ingestion.DiscoverOpts{})
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

func acceptsOperations(have, want []ingestion.Operation) bool {
	if len(want) == 0 {
		return true
	}
	if len(have) == 0 {
		return false
	}
	set := make(map[ingestion.Operation]bool, len(have))
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

func ensureSchemas(ctx context.Context, src ingestion.Source, snk ingestion.Sink, spec ingestion.RunSpec) error {
	sch, ok := snk.(ingestion.Schematized)
	if !ok {
		return nil
	}
	prov, ok := src.(ingestion.SchemaProvider)
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
	log    ingestion.Logger
	tenant ingestion.TenantID
	run    ingestion.RunID

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

func newEmitter(ctx context.Context, bus eventbus.Bus, log ingestion.Logger, tenant ingestion.TenantID, run ingestion.RunID) *emitter {
	return &emitter{ctx: ctx, bus: bus, log: log, tenant: tenant, run: run, res: map[string]*tally{}}
}

func (e *emitter) next() uint64 { return e.seq.Add(1) }

func (e *emitter) publish(ev ingestion.Event) {
	if ev.Type == ingestion.EvBatchWritten {
		e.mu.Lock()
		e.runRecords += ev.Fields.Records
		e.runBytes += ev.Fields.Bytes
		t := e.res[ev.Resource]
		if t == nil {
			t = &tally{}
			e.res[ev.Resource] = t
		}
		t.records += ev.Fields.Records
		t.bytes += ev.Fields.Bytes
		e.mu.Unlock()
	}
	if e.log != nil {
		fields := []ingestion.Field{
			{Key: "run", Value: string(ev.Run)},
			{Key: "resource", Value: ev.Resource},
			{Key: "records", Value: ev.Fields.Records},
			{Key: "bytes", Value: ev.Fields.Bytes},
		}
		if ev.Fields.Error != "" {
			fields = append(fields, ingestion.Field{Key: "error", Value: ev.Fields.Error})
		}
		e.log.Info(ev.Type.String(), fields...)
	}
	if err := ingestion.PublishEvent(e.ctx, e.bus, ev); err != nil && e.log != nil {
		e.log.Error("runner: publish fact", err, ingestion.Field{Key: "type", Value: ev.Type.String()})
	}
}

func (e *emitter) emitFact(t ingestion.EventType, resource string, f ingestion.EventFields) {
	e.publish(ingestion.Event{
		Type:     t,
		Tenant:   e.tenant,
		Run:      e.run,
		Resource: resource,
		Seq:      e.next(),
		At:       time.Now(),
		Fields:   f,
	})
}

func (e *emitter) fact(t ingestion.EventType, f ingestion.EventFields) { e.emitFact(t, "", f) }

func (e *emitter) fail(err error) {
	if e.log != nil {
		e.log.Error("runner: run failed", err, ingestion.Field{Key: "run", Value: string(e.run)})
	}
	e.fact(ingestion.EvRunFailed, ingestion.EventFields{Error: err.Error()})
}

func (e *emitter) partial(err error) {
	if e.log != nil {
		e.log.Error("runner: run partial", err, ingestion.Field{Key: "run", Value: string(e.run)})
	}
	e.fact(ingestion.EvRunPartial, ingestion.EventFields{Error: err.Error()})
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
