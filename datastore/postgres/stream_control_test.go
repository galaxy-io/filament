package postgres_test

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/galaxy-io/filament/streamkit"
)

func TestStreamWorkerClaimIsExclusive(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() { <-start; results <- f.store.ClaimStreamAttempt(ctx, a.Lease) }()
	}
	close(start)
	successes := 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, filament.ErrFenced) {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("claims=%d", successes)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptClean}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.ClaimStreamAttempt(ctx, a.Lease); !errors.Is(err, filament.ErrFenced) {
		t.Fatalf("ended claim: %v", err)
	}
}

func TestStreamStopRetiresOnlyUnclaimedDispatch(t *testing.T) {
	for _, claimed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unclaimed", true: "claimed"}[claimed], func(t *testing.T) {
			f := newRuntimeFixture(t)
			a := f.start(t)
			ctx := context.Background()
			if claimed {
				if err := f.store.ClaimStreamAttempt(ctx, a.Lease); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: f.activation.StreamStateRequest, ExpectedRevision: f.state.Revision, Desired: filament.StreamStopped}); err != nil {
				t.Fatal(err)
			}
			if err := f.store.RetireUnclaimedAttempt(ctx, a.Lease); err != nil {
				t.Fatal(err)
			}
			state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
			if err != nil {
				t.Fatal(err)
			}
			if claimed {
				if state.Attempt.EndedAt != nil {
					t.Fatal("retired a worker that could still be writing")
				}
				return
			}
			if state.Attempt.EndedAt == nil || state.Attempt.Termination != filament.AttemptClean {
				t.Fatal("unclaimed dispatch not retired")
			}
			r, err := f.store.LoadRun(ctx, f.request.Tenant, f.request.Run)
			if err != nil || r.Status != filament.RunCompleted || r.EndedAt == nil {
				t.Fatalf("stopped run: %+v %v", r, err)
			}
			if err := f.store.ClaimStreamAttempt(ctx, a.Lease); !errors.Is(err, filament.ErrFenced) {
				t.Fatalf("late delivery: %v", err)
			}
		})
	}
}

func TestPendingStreamRunsExcludesDeletedPipeline(t *testing.T) {
	f := newRuntimeFixture(t)
	ctx := context.Background()
	runs, err := f.store.PendingStreamRuns(ctx, "", 100)
	if err != nil || len(runs) != 1 || runs[0].Run != f.request.Run {
		t.Fatalf("pending before delete = %v, %v", runs, err)
	}
	if err := f.store.DeletePipeline(ctx, f.request.Tenant, f.activation.Spec.PipelineID); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		runs, err = f.store.PendingStreamRuns(ctx, "", 100)
		if err != nil || len(runs) != 0 {
			t.Fatalf("pending after delete = %v, %v", runs, err)
		}
	}
	if _, err := f.store.LoadRun(ctx, f.request.Tenant, f.request.Run); err != nil {
		t.Fatalf("pipeline deletion should retain run history: %v", err)
	}
}

// A run submitted against a live stream waits as its successor, the live run
// drains, and the successor takes over the generation with the revised
// membership: a new table pending, a resynced one back to pending, and the
// epoch that certifies their baselines admits them.
func TestStreamSuccessorHandover(t *testing.T) {
	ctx := context.Background()
	store, desired, versions := admissionFixture(t)
	codecs := &streamkit.Registry{}
	if err := codecs.Register("counter", 0, streamkit.Uint64Codec{}); err != nil {
		t.Fatal(err)
	}
	store.ConfigureStreamCodecs(codecs)
	continuousRun := func(version string, resources, resync []string) filament.RunState {
		run := admissionRun(version, desired.Route, filament.RunRequested)
		run.Request.Options.Execution = filament.ExecutionContinuous
		run.Request.SourceConnectionID, run.Request.SinkConnectionID = connectionOne, connectionTwo
		run.Request.Resources, run.Request.Resync = resources, resync
		return run
	}
	first := continuousRun(versions[0], []string{"events"}, nil)
	if err := store.CreateRunWithReplicationStream(ctx, first, desired); err != nil {
		t.Fatal(err)
	}
	stream, err := store.ResolveReplicationStream(ctx, desired)
	if err != nil {
		t.Fatal(err)
	}
	key := filament.StreamStateRequest{Tenant: tenantA, Stream: filament.StreamRef{ID: stream.ID, Generation: stream.Generation}}
	state, err := store.LoadStreamState(ctx, key)
	if err != nil || state.Run != first.Run {
		t.Fatalf("activation: %+v %v", state, err)
	}
	attempt, err := store.StartAttempt(ctx, filament.StartAttemptRequest{Tenant: tenantA, Stream: key.Stream, Run: first.Run, ExecutionID: uuid.NewString(), PipelineVersionID: versions[0], Route: desired.Route, ExpectedRevision: state.Revision, TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimStreamAttempt(ctx, attempt.Lease); err != nil {
		t.Fatal(err)
	}

	second := continuousRun(versions[1], []string{"events", "orders"}, []string{"events"})
	successor := desired
	successor.ID, successor.CreatedFromPipelineVersionID = uuid.NewString(), versions[1]
	if err := store.CreateRunWithReplicationStream(ctx, second, successor); err != nil {
		t.Fatal(err)
	}
	if r, err := store.LoadRun(ctx, tenantA, second.Run); err != nil || r.Status != filament.RunScheduled {
		t.Fatalf("successor must wait as scheduled: %+v %v", r, err)
	}
	if state, err = store.LoadStreamState(ctx, key); err != nil || state.Run != first.Run || state.Desired != filament.StreamStopped {
		t.Fatalf("live run must be asked to stop: %+v %v", state, err)
	}
	if err := store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: attempt.Lease, Termination: filament.AttemptClean}); err != nil {
		t.Fatal(err)
	}

	if r, err := store.LoadRun(ctx, tenantA, first.Run); err != nil || r.Status != filament.RunCompleted || r.EndedAt == nil {
		t.Fatalf("drained run: %+v %v", r, err)
	}
	if r, err := store.LoadRun(ctx, tenantA, second.Run); err != nil || r.Status != filament.RunRequested {
		t.Fatalf("promoted successor: %+v %v", r, err)
	}
	state, err = store.LoadStreamState(ctx, key)
	if err != nil || state.Run != second.Run || state.Desired != filament.StreamEnabled || state.PipelineVersionID != versions[1] {
		t.Fatalf("handover: %+v %v", state, err)
	}
	if state.Membership["events"] != filament.ReplicationStreamResourcePending || state.Membership["orders"] != filament.ReplicationStreamResourcePending {
		t.Fatalf("membership after handover: %v", state.Membership)
	}
	pending, err := store.PendingStreamRuns(ctx, "", 10)
	if err != nil || len(pending) != 1 || pending[0].Run != second.Run {
		t.Fatalf("pending runs: %+v %v", pending, err)
	}

	attempt, err = store.StartAttempt(ctx, filament.StartAttemptRequest{Tenant: tenantA, Stream: key.Stream, Run: second.Run, ExecutionID: uuid.NewString(), PipelineVersionID: versions[1], Route: desired.Route, ExpectedRevision: state.Revision, TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	ref := filament.EpochRef{Attempt: attempt.Lease.Attempt, Epoch: 1, MembershipRevision: state.MembershipRevision}
	commit := handoverEpoch(t, ref, versions[1], []string{"events", "orders"})
	if _, err := store.CommitEpoch(ctx, commit); err != nil {
		t.Fatalf("baseline epoch: %v", err)
	}
	after, err := store.LoadStreamState(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if after.Membership["events"] != filament.ReplicationStreamResourceActive || after.Membership["orders"] != filament.ReplicationStreamResourceActive || after.MembershipRevision != state.MembershipRevision+1 {
		t.Fatalf("activation by epoch: %v revision %d -> %d", after.Membership, state.MembershipRevision, after.MembershipRevision)
	}
	// The next epoch carries the stale revision and is refused.
	stale := handoverEpoch(t, filament.EpochRef{Attempt: attempt.Lease.Attempt, Epoch: 2, MembershipRevision: state.MembershipRevision}, versions[1], nil)
	if _, err := store.CommitEpoch(ctx, stale); !errors.Is(err, filament.ErrMembershipChanged) {
		t.Fatalf("stale revision: %v", err)
	}
}

// Stopping a stream on purpose withdraws a waiting successor.
func TestStreamStopCancelsSuccessor(t *testing.T) {
	ctx := context.Background()
	store, desired, versions := admissionFixture(t)
	run := func(version string) filament.RunState {
		r := admissionRun(version, desired.Route, filament.RunRequested)
		r.Request.Options.Execution = filament.ExecutionContinuous
		r.Request.SourceConnectionID, r.Request.SinkConnectionID = connectionOne, connectionTwo
		r.Request.Resources = []string{"events"}
		return r
	}
	first := run(versions[0])
	if err := store.CreateRunWithReplicationStream(ctx, first, desired); err != nil {
		t.Fatal(err)
	}
	stream, err := store.ResolveReplicationStream(ctx, desired)
	if err != nil {
		t.Fatal(err)
	}
	key := filament.StreamStateRequest{Tenant: tenantA, Stream: filament.StreamRef{ID: stream.ID, Generation: stream.Generation}}
	state, err := store.LoadStreamState(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err := store.StartAttempt(ctx, filament.StartAttemptRequest{Tenant: tenantA, Stream: key.Stream, Run: first.Run, ExecutionID: uuid.NewString(), PipelineVersionID: versions[0], Route: desired.Route, ExpectedRevision: state.Revision, TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimStreamAttempt(ctx, attempt.Lease); err != nil {
		t.Fatal(err)
	}
	second := run(versions[1])
	successor := desired
	successor.ID, successor.CreatedFromPipelineVersionID = uuid.NewString(), versions[1]
	if err := store.CreateRunWithReplicationStream(ctx, second, successor); err != nil {
		t.Fatal(err)
	}
	if state, err = store.LoadStreamState(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: key, ExpectedRevision: state.Revision, Desired: filament.StreamStopped}); err != nil {
		t.Fatal(err)
	}
	if err := store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: attempt.Lease, Termination: filament.AttemptClean}); err != nil {
		t.Fatal(err)
	}
	if r, err := store.LoadRun(ctx, tenantA, second.Run); err != nil || r.Status != filament.RunCanceled {
		t.Fatalf("successor after stop: %+v %v", r, err)
	}
	if state, err = store.LoadStreamState(ctx, key); err != nil || state.Run != first.Run || state.Desired != filament.StreamStopped {
		t.Fatalf("stopped stream: %+v %v", state, err)
	}
}

func handoverEpoch(t *testing.T, ref filament.EpochRef, version string, activated []string) filament.EpochCommit {
	t.Helper()
	p, err := pipeline.NewStream(pipeline.Config{Sink: &epochTestSink{bound: ref}, WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullReplace)}}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	if err := p.BindEpoch(ref); err != nil {
		t.Fatal(err)
	}
	c := rowmodel.Control{Kind: rowmodel.ProgressBoundary, Domain: rowmodel.DomainKey{Incarnation: "source", Domain: "log"}, Position: rowmodel.Position{Codec: "counter", Value: []byte(strconv.FormatInt(ref.Epoch*10, 10))}}
	if _, err := p.Barrier(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	completion, err := p.SealEpoch()
	if err != nil {
		t.Fatal(err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatal(err)
	}
	return filament.EpochCommit{Certificate: filament.EpochCertificate{FormatVersion: filament.EpochCertificateFormatVersion, Tenant: tenantA, Ref: ref, PipelineVersionID: version, Coverage: filament.Coverage{Positions: filament.DomainPositions{c.Domain: c.Position}, Activated: activated}}, Completion: completion}
}
