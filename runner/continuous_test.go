package runner

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

type continuousTestStore struct {
	leaseLost bool
	filament.ContinuousRunStore
	mu         sync.Mutex
	state      filament.StreamState
	cert       *filament.CommittedEpoch
	fail, lost bool
	renewals   int
}

func (s *continuousTestStore) LoadStreamState(context.Context, filament.StreamStateRequest) (filament.StreamState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state, nil
}

func (s *continuousTestStore) RenewLease(context.Context, filament.LeaseToken, time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.renewals++
	if s.leaseLost {
		return filament.ErrLeaseExpired
	}
	return nil
}

func (s *continuousTestStore) EndAttempt(_ context.Context, r filament.EndAttemptRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Attempt.Termination = r.Termination
	return nil
}

func (s *continuousTestStore) CommitEpoch(_ context.Context, r filament.EpochCommit) (filament.CommittedEpoch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return filament.CommittedEpoch{}, errors.New("store unavailable")
	}
	c := filament.CommittedEpoch{Certificate: r.Certificate.Clone(), CommittedAt: time.Now()}
	s.cert = &c
	if s.lost {
		return filament.CommittedEpoch{}, context.DeadlineExceeded
	}
	return c, nil
}

func (s *continuousTestStore) GetEpoch(context.Context, filament.EpochLookup) (filament.CommittedEpoch, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cert == nil {
		return filament.CommittedEpoch{}, filament.ErrNotFound
	}
	return *s.cert, nil
}

type continuousTestSource struct {
	sourceConnectionID string
	op                 rowmodel.Operation
	keys               []string
	idle               bool
	started            sync.Once
	filament.Source
	store  *continuousTestStore
	writer arrowbatch.RowWriter
	ack    int
	block  bool
	opened chan struct{}
}

func (*continuousTestSource) Lookup(string, int) (rowmodel.PositionCodec, error) {
	return streamkit.OpaqueCodec{}, nil
}

func (*continuousTestSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Stream: &filament.StreamCapabilities{Input: filament.InputMessages, Delivery: filament.DeliveryReplayableAtLeastOnce}}
}

func (*continuousTestSource) Configure(context.Context, filament.Config) error { return nil }
func (*continuousTestSource) Teardown(context.Context) error                   { return nil }
func (s *continuousTestSource) OpenStream(_ context.Context, opts filament.StreamOpenOpts) (filament.StreamSession, error) {
	s.sourceConnectionID = opts.SourceConnectionID
	return s, nil
}
func (s *continuousTestSource) Close(context.Context) error { return nil }
func (s *continuousTestSource) Read(ctx context.Context, out filament.StreamRecordSink, _ filament.Boundary) (filament.Coverage, error) {
	if s.idle {
		s.started.Do(func() { close(s.opened) })
		select {
		case <-ctx.Done():
			return filament.Coverage{}, ctx.Err()
		case <-time.After(10 * time.Millisecond):
			return filament.Coverage{}, nil
		}
	}
	if s.block {
		close(s.opened)
		<-ctx.Done()
		return filament.Coverage{}, ctx.Err()
	}
	if s.writer == nil {
		var err error
		s.writer, err = out.Builder("events", 0, rowmodel.Schema{Resource: "events", Fields: []rowmodel.Field{{Name: "value", Logical: rowmodel.LogicalString}}})
		if err != nil {
			return filament.Coverage{}, err
		}
	}
	s.writer.String("value")
	if err := s.writer.EndRow(rowmodel.Meta{Op: s.op}); err != nil {
		return filament.Coverage{}, err
	}
	d := filament.DomainKey{Domain: "log", Incarnation: "source"}
	p := filament.Position{Codec: "opaque", Value: []byte("position")}
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: d, Position: p}); err != nil {
		return filament.Coverage{}, err
	}
	return filament.Coverage{Positions: filament.DomainPositions{d: p}}, nil
}

func (s *continuousTestSource) Acknowledge(context.Context, filament.Coverage) error {
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.store.cert == nil {
		return errors.New("ack before certificate")
	}
	s.ack++
	return nil
}

type continuousTestSink struct {
	filament.Sink
	ref     filament.EpochRef
	rows    int64
	commits int
	fail    bool
}

func (*continuousTestSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Capabilities: filament.SinkCapabilities{Stream: &filament.StreamingSinkCapabilities{WritePolicies: filament.WriteCapabilities(filament.IngestionFullAppend)}, WritePolicies: filament.WriteCapabilities(filament.IngestionFullAppend)}}
}
func (*continuousTestSink) Open(context.Context, filament.RunSpec) error { return nil }
func (s *continuousTestSink) BeginEpoch(_ context.Context, r filament.EpochRef) error {
	s.ref = r
	s.rows = 0
	return nil
}

func (s *continuousTestSink) Apply(_ context.Context, b *arrowbatch.Batch, o filament.ApplyOptions) (filament.WriteReceipt, error) {
	if o.Epoch == nil || *o.Epoch != s.ref {
		return filament.WriteReceipt{}, filament.ErrEpochMismatch
	}
	s.rows += int64(b.NumRows())
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func (s *continuousTestSink) CommitEpoch(context.Context, filament.EpochRef) ([]filament.EpochReceipt, error) {
	if s.fail {
		return nil, errors.New("sink failed")
	}
	s.commits++
	return []filament.EpochReceipt{{Resource: "events", Rows: s.rows}}, nil
}
func (*continuousTestSink) AbortEpoch(context.Context, filament.EpochRef) error { return nil }
func (*continuousTestSink) CloseSession(context.Context) error                  { return nil }

func continuousFixture(t *testing.T) (ContinuousConfig, *continuousTestStore, *continuousTestSource, *continuousTestSink) {
	t.Helper()
	a := filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}
	store := &continuousTestStore{state: filament.StreamState{Desired: filament.StreamEnabled, PipelineVersionID: "version", Run: "run", MembershipRevision: 1, Attempt: &filament.Attempt{Lease: filament.LeaseToken{Tenant: "tenant", Attempt: a}}}}
	src := &continuousTestSource{store: store}
	sink := &continuousTestSink{}
	codecs := &streamkit.Registry{}
	if err := codecs.Register("opaque", 0, streamkit.OpaqueCodec{}); err != nil {
		t.Fatal(err)
	}
	policy := filament.WritePolicy{Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}
	spec := filament.RunSpec{Tenant: "tenant", Run: "run", PipelineVersionID: "version", StreamAttempt: &a, Resources: []string{"events"}, WritePolicies: map[string]filament.WritePolicy{"events": policy}, Options: filament.RunOptions{Execution: filament.ExecutionContinuous}}
	cfg := ContinuousConfig{Enabled: true, Spec: spec, Store: store, Source: src, Sink: sink, Codecs: codecs, Schemas: map[string]rowmodel.Schema{"events": {}}, Boundary: filament.Boundary{MaxRecords: 1, MaxWait: time.Second}, LeaseTTL: 300 * time.Millisecond, DrainTimeout: time.Second, MaxEpochs: 2}
	return cfg, store, src, sink
}

func TestContinuousCommitWindows(t *testing.T) {
	for _, mode := range []string{"normal", "sink failure", "store failure", "lost store response"} {
		t.Run(mode, func(t *testing.T) {
			cfg, store, src, sink := continuousFixture(t)
			store.fail = mode == "store failure"
			store.lost = mode == "lost store response"
			sink.fail = mode == "sink failure"
			err := RunContinuous(context.Background(), cfg)
			success := mode == "normal" || mode == "lost store response"
			if success {
				if err != nil || src.ack != 2 || sink.commits != 2 {
					t.Fatalf("err=%v acks=%d commits=%d", err, src.ack, sink.commits)
				}
			} else if err == nil || src.ack != 0 {
				t.Fatalf("failure acked: %v %d", err, src.ack)
			}
		})
	}
}

func TestContinuousLeaseRenewalDuringBlockedRead(t *testing.T) {
	cfg, store, src, _ := continuousFixture(t)
	src.block = true
	src.opened = make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- RunContinuous(ctx, cfg) }()
	<-src.opened
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			cancel()
			t.Fatal("lease not renewed")
		case <-ticker.C:
			store.mu.Lock()
			n := store.renewals
			store.mu.Unlock()
			if n >= 3 {
				cancel()
				if err := <-done; !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
				return
			}
		}
	}
}

func TestContinuousGateRejectsBeforeSideEffects(t *testing.T) {
	cfg, _, _, _ := continuousFixture(t)
	cfg.Enabled = false
	if err := RunContinuous(context.Background(), cfg); !errors.Is(err, filament.ErrContinuousDisabled) {
		t.Fatal(err)
	}
}

func TestContinuousIdleStopAndLeaseLoss(t *testing.T) {
	for _, mode := range []string{"pause", "lease loss", "drain timeout"} {
		t.Run(mode, func(t *testing.T) {
			cfg, store, src, _ := continuousFixture(t)
			cfg.MaxEpochs = 0
			cfg.DrainTimeout = 200 * time.Millisecond
			src.opened = make(chan struct{})
			src.idle = mode != "drain timeout"
			src.block = !src.idle
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- RunContinuous(ctx, cfg) }()
			<-src.opened
			store.mu.Lock()
			if mode == "lease loss" {
				store.leaseLost = true
			} else {
				store.state.Desired = filament.StreamPaused
			}
			store.mu.Unlock()
			err := <-done
			if mode == "pause" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("expected interrupted execution")
			}
			if src.ack != 0 {
				t.Fatal("idle shutdown acknowledged input")
			}
			store.mu.Lock()
			termination := store.state.Attempt.Termination
			store.mu.Unlock()
			if mode == "pause" && termination != filament.AttemptClean {
				t.Fatalf("clean pause: %s", termination)
			}
		})
	}
}

func TestContinuousPauseBeforeStartupEndsAttempt(t *testing.T) {
	cfg, store, _, sink := continuousFixture(t)
	store.state.Desired = filament.StreamPaused
	if err := RunContinuous(context.Background(), cfg); !errors.Is(err, filament.ErrFenced) {
		t.Fatalf("startup race: %v", err)
	}
	if store.state.Attempt.Termination != filament.AttemptClean {
		t.Fatal("startup pause stranded the admitted attempt")
	}
	if sink.commits != 0 {
		t.Fatal("paused startup wrote to the sink")
	}
}

// A new sink can opt into key writes without changing the runner.
type keyedContinuousSink struct {
	continuousTestSink
	opened bool
	seen   filament.WritePolicy
}

func (*keyedContinuousSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Capabilities: filament.SinkCapabilities{Stream: &filament.StreamingSinkCapabilities{WritePolicies: filament.WriteCapabilities(filament.IngestionFullUpsert)}}}
}

func (s *keyedContinuousSink) Open(_ context.Context, spec filament.RunSpec) error {
	s.opened = true
	s.seen = spec.WritePolicies["events"]
	return nil
}

func (s *keyedContinuousSink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if len(opts.Policy.Keys) != 1 || opts.Policy.Keys[0] != "value" || opts.Policy.Checkpoint != filament.CheckpointAfterCommit {
		return filament.WriteReceipt{}, errors.New("unbound key policy")
	}
	return s.continuousTestSink.Apply(ctx, b, opts)
}

func TestContinuousNegotiatedUpsert(t *testing.T) {
	for _, missing := range []bool{false, true} {
		cfg, _, src, _ := continuousFixture(t)
		sink := &keyedContinuousSink{}
		cfg.Sink = sink
		plan, err := filament.PlanContinuousWrite(src.Spec(), sink.Spec(), filament.WriteUpsert)
		if err != nil {
			t.Fatal(err)
		}
		cfg.Spec.WritePolicies = map[string]filament.WritePolicy{"events": plan.Policy}
		if !missing {
			src.keys = []string{"value"}
		}
		err = RunContinuous(context.Background(), cfg)
		if missing {
			if err == nil || sink.opened || src.ack != 0 {
				t.Fatalf("missing keys: err=%v opened=%v ack=%d", err, sink.opened, src.ack)
			}
		} else if err != nil || src.ack != 2 || sink.seen.Capability.Mode != filament.WriteUpsert {
			t.Fatalf("upsert: err=%v policy=%+v ack=%d", err, sink.seen, src.ack)
		}
	}
}

func TestContinuousRejectsWeakenedAndUnknownPolicies(t *testing.T) {
	cfg, _, _, _ := continuousFixture(t)
	cfg.Sink = &keyedContinuousSink{}
	policy := filament.IngestionFullUpsert.WritePolicy()
	policy.Capability.RequiresPK = false
	cfg.Spec.WritePolicies["events"] = policy
	if err := validateContinuous(cfg); err == nil {
		t.Fatal("weakened capability accepted")
	}
	cfg.Spec.WritePolicies = nil
	cfg.Spec.IngestionTypes = map[string]filament.IngestionType{"events": "unknown"}
	if err := validateContinuous(cfg); err == nil {
		t.Fatal("unknown intent defaulted")
	}
}

func TestContinuousRequiresExplicitWritePolicies(t *testing.T) {
	for _, intent := range []filament.IngestionType{
		"", filament.IngestionFullAppend, filament.IngestionFullUpsert,
		filament.IngestionCDCAppend, filament.IngestionCDCMerge,
	} {
		for _, key := range []string{"events", ""} {
			t.Run(string(intent)+"/"+key, func(t *testing.T) {
				cfg, _, src, sink := continuousFixture(t)
				cfg.Spec.WritePolicies = nil
				cfg.Spec.IngestionTypes = map[string]filament.IngestionType{key: intent}
				err := RunContinuous(context.Background(), cfg)
				if err == nil || !strings.Contains(err.Error(), `explicit write policy required for "events"`) {
					t.Fatalf("missing policy: %v", err)
				}
				if src.ack != 0 || sink.rows != 0 || sink.commits != 0 {
					t.Fatal("missing policy reached stream execution")
				}
			})
		}
	}
}

func (s *continuousTestSource) Schema(_ context.Context, resource string) (filament.RecordSchema, error) {
	return filament.RecordSchema{Resource: resource, PrimaryKey: s.keys, Fields: []rowmodel.Field{{Name: "value", Logical: rowmodel.LogicalString}}}, nil
}

func TestContinuousRejectsUnexpectedOperationBeforeSink(t *testing.T) {
	cfg, _, src, sink := continuousFixture(t)
	src.op = rowmodel.OpDelete
	err := RunContinuous(context.Background(), cfg)
	if err == nil || sink.rows != 0 || sink.commits != 0 || src.ack != 0 {
		t.Fatalf("err=%v rows=%d commits=%d ack=%d", err, sink.rows, sink.commits, src.ack)
	}
}

type orderedContinuousSource struct{ *continuousTestSource }

func (*orderedContinuousSource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Stream: &filament.StreamCapabilities{Input: filament.InputChanges, EmitsOps: []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}, Ordering: []filament.Ordering{filament.OrderingGlobalStrict}, Delivery: filament.DeliveryReplayableAtLeastOnce}}
}

func (s *orderedContinuousSource) OpenStream(context.Context, filament.StreamOpenOpts) (filament.StreamSession, error) {
	return s, nil
}

func (s *orderedContinuousSource) Read(ctx context.Context, out filament.StreamRecordSink, _ filament.Boundary) (filament.Coverage, error) {
	writers := map[string]arrowbatch.RowWriter{}
	for i, resource := range []string{"events", "other", "events"} {
		w := writers[resource]
		if w == nil {
			schema, _ := s.Schema(ctx, resource)
			var err error
			w, err = out.Builder(resource, 0, schema)
			if err != nil {
				return filament.Coverage{}, err
			}
			writers[resource] = w
		}
		w.String("value")
		op := []rowmodel.Operation{rowmodel.OpInsert, rowmodel.OpUpdate, rowmodel.OpDelete}[i]
		if err := w.EndRow(rowmodel.Meta{Op: op}); err != nil {
			return filament.Coverage{}, err
		}
	}
	domain := filament.DomainKey{Domain: "log", Incarnation: "source"}
	position := filament.Position{Codec: "opaque", Value: []byte("position")}
	if err := out.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: domain, Position: position}); err != nil {
		return filament.Coverage{}, err
	}
	return filament.Coverage{Positions: filament.DomainPositions{domain: position}}, nil
}

type orderedContinuousSink struct {
	continuousTestSink
	resources []string
	ops       []filament.Operation
}

func (*orderedContinuousSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Capabilities: filament.SinkCapabilities{Stream: &filament.StreamingSinkCapabilities{WritePolicies: filament.WriteCapabilities(filament.IngestionCDCMerge)}}}
}

func (s *orderedContinuousSink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if len(opts.Policy.Keys) != 1 || opts.Policy.Keys[0] != "value" {
		return filament.WriteReceipt{}, errors.New("missing key binding")
	}
	for i := range b.NumRows() {
		s.resources = append(s.resources, b.Resource)
		s.ops = append(s.ops, b.Op(i))
	}
	return s.continuousTestSink.Apply(ctx, b, opts)
}

func (*orderedContinuousSink) CommitEpoch(context.Context, filament.EpochRef) ([]filament.EpochReceipt, error) {
	return []filament.EpochReceipt{{Resource: "events", Rows: 2}, {Resource: "other", Rows: 1}}, nil
}

func TestContinuousNegotiatedMergePreservesCrossResourceOrder(t *testing.T) {
	cfg, _, src, _ := continuousFixture(t)
	src.keys = []string{"value"}
	cfg.Source = &orderedContinuousSource{src}
	sink := &orderedContinuousSink{}
	cfg.Sink = sink
	cfg.MaxEpochs = 1
	cfg.Spec.Resources = []string{"events", "other"}
	plan, err := filament.PlanContinuousWrite(cfg.Source.Spec(), sink.Spec(), filament.WriteMerge)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Spec.WritePolicies = map[string]filament.WritePolicy{"": plan.Policy}
	cfg.Schemas["other"] = rowmodel.Schema{}
	if err := RunContinuous(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sink.resources, []string{"events", "other", "events"}) || !slices.Equal(sink.ops, []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete}) || src.ack != 1 {
		t.Fatalf("resources=%v ops=%v ack=%d", sink.resources, sink.ops, src.ack)
	}
}

func TestContinuousPassesSourceConnectionIdentity(t *testing.T) {
	cfg, _, src, _ := continuousFixture(t)
	cfg.Spec.SourceConnectionID = "source-connection"
	if err := RunContinuous(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if src.sourceConnectionID != cfg.Spec.SourceConnectionID {
		t.Fatalf("stream connection ID = %q", src.sourceConnectionID)
	}
}
