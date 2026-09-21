package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/datastore/postgres"
	"github.com/galaxy-io/filament/streamkit"
)

type runtimeFixture struct {
	store      *postgres.Store
	activation filament.StreamActivation
	request    filament.StartAttemptRequest
	state      filament.StreamState
}

func newRuntimeFixture(t *testing.T) runtimeFixture {
	t.Helper()
	ctx := context.Background()
	store, desired, versions := admissionFixture(t)
	stream, err := store.ResolveReplicationStream(ctx, desired)
	if err != nil {
		t.Fatal(err)
	}
	run := admissionRun(versions[0], desired.Route, filament.RunRequested)
	run.Request.Options.Execution = filament.ExecutionContinuous
	run.Request.ReplicationStream = &filament.StreamRef{ID: stream.ID, Generation: stream.Generation}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReconcileReplicationStreamResources(ctx, stream.ID, tenantA, []string{"events"}, "none"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool().Exec(ctx, "UPDATE replication_stream_resources SET status=2 WHERE replication_stream_id=$1", stream.ID); err != nil {
		t.Fatal(err)
	}
	codecs := &streamkit.Registry{}
	if err := codecs.Register("counter", 0, streamkit.Uint64Codec{}); err != nil {
		t.Fatal(err)
	}
	if err := codecs.Register("opaque", 0, streamkit.OpaqueCodec{}); err != nil {
		t.Fatal(err)
	}
	store.ConfigureStreamCodecs(codecs)
	runtime := store
	a := filament.StreamActivation{StreamStateRequest: filament.StreamStateRequest{Tenant: tenantA, Stream: *run.Request.ReplicationStream}, Spec: filament.RunSpec{Tenant: tenantA, Run: run.Run, PipelineID: pipelineOne, PipelineVersionID: versions[0], CheckpointRoute: desired.Route, ReplicationStream: run.Request.ReplicationStream, SourceConnectionID: connectionOne, SinkConnectionID: connectionTwo, Resources: []string{"events"}, Options: filament.RunOptions{Execution: filament.ExecutionContinuous}}}
	state, err := runtime.ActivateStream(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	req := filament.StartAttemptRequest{Tenant: tenantA, Stream: a.Stream, Run: run.Run, ExecutionID: uuid.NewString(), PipelineVersionID: versions[0], Route: desired.Route, ExpectedRevision: state.Revision, TTL: time.Minute}
	return runtimeFixture{store: runtime, activation: a, request: req, state: state}
}

func (f runtimeFixture) start(t *testing.T) filament.Attempt {
	t.Helper()
	a, err := f.store.StartAttempt(context.Background(), f.request)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestRuntimeAttemptAdmissionRaceAndRetry(t *testing.T) {
	f := newRuntimeFixture(t)
	ctx := context.Background()
	start := make(chan struct{})
	type result struct {
		attempt filament.Attempt
		err     error
		request filament.StartAttemptRequest
	}
	results := make(chan result, 2)
	for range 2 {
		req := f.request
		req.ExecutionID = uuid.NewString()
		go func() { <-start; a, err := f.store.StartAttempt(ctx, req); results <- result{a, err, req} }()
	}
	close(start)
	var winner result
	success, blocked := 0, 0
	for range 2 {
		r := <-results
		if r.err == nil {
			success++
			winner = r
		} else if errors.Is(r.err, filament.ErrTakeoverBlocked) {
			blocked++
		} else {
			t.Fatal(r.err)
		}
	}
	if success != 1 || blocked != 1 {
		t.Fatalf("success=%d blocked=%d", success, blocked)
	}
	retry, err := f.store.StartAttempt(ctx, winner.request)
	if err != nil || retry.Lease != winner.attempt.Lease || !retry.ExpiresAt.Equal(winner.attempt.ExpiresAt) {
		t.Fatalf("retry changed lease: %+v %v", retry, err)
	}
	if retry.Spec.StreamAttempt == nil || *retry.Spec.StreamAttempt != retry.Lease.Attempt {
		t.Fatal("attempt spec not bound")
	}
	conflict := winner.request
	conflict.TTL = time.Hour
	if _, err := f.store.StartAttempt(ctx, conflict); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("conflicting retry: %v", err)
	}
	if err := f.store.RenewLease(ctx, retry.Lease, time.Hour); err != nil {
		t.Fatal(err)
	}
	state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || !state.Attempt.ExpiresAt.After(retry.ExpiresAt) {
		t.Fatalf("renewal: %+v %v", state, err)
	}
}

func TestRuntimeExpiryCannotResurrectOrAuthorizeTakeover(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	if _, err := f.store.Pool().Exec(ctx, "UPDATE stream_attempts SET expires_at=clock_timestamp()-interval '1 second' WHERE token=$1", a.Lease.Attempt.Token); err != nil {
		t.Fatal(err)
	}
	if err := f.store.RenewLease(ctx, a.Lease, time.Minute); !errors.Is(err, filament.ErrLeaseExpired) {
		t.Fatalf("expired renewal: %v", err)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptClean}); !errors.Is(err, filament.ErrLeaseExpired) {
		t.Fatalf("expired clean: %v", err)
	}
	req := f.request
	req.ExecutionID = uuid.NewString()
	if _, err := f.store.StartAttempt(ctx, req); !errors.Is(err, filament.ErrTakeoverBlocked) {
		t.Fatalf("expired takeover: %v", err)
	}
}

func TestRuntimeTerminationFencesStaleOwners(t *testing.T) {
	for _, termination := range []filament.AttemptTermination{filament.AttemptClean, filament.AttemptReaped, filament.AttemptUnproven} {
		t.Run(string(termination), func(t *testing.T) {
			f := newRuntimeFixture(t)
			a := f.start(t)
			ctx := context.Background()
			if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: termination}); err != nil {
				t.Fatal(err)
			}
			req := f.request
			req.ExecutionID = uuid.NewString()
			b, err := f.store.StartAttempt(ctx, req)
			if termination != filament.AttemptClean {
				if !errors.Is(err, filament.ErrTakeoverBlocked) {
					t.Fatalf("unproven takeover: %v", err)
				}
				return
			}
			if err != nil || b.Lease.Attempt.Token <= a.Lease.Attempt.Token {
				t.Fatalf("successor: %+v %v", b, err)
			}
			if err := f.store.RenewLease(ctx, a.Lease, time.Minute); !errors.Is(err, filament.ErrFenced) {
				t.Fatalf("stale renewal: %v", err)
			}
			if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptUnproven}); !errors.Is(err, filament.ErrFenced) {
				t.Fatalf("stale completion: %v", err)
			}
		})
	}
}

func TestRuntimeIntentAndTenantIsolation(t *testing.T) {
	f := newRuntimeFixture(t)
	ctx := context.Background()
	copy := f.activation
	copy.Spec.Resources = []string{"different"}
	if _, err := f.store.ActivateStream(ctx, copy); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("mutated activation: %v", err)
	}
	if err := f.store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: f.activation.StreamStateRequest, ExpectedRevision: 1, Desired: filament.StreamPaused}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.StartAttempt(ctx, f.request); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("paused admission: %v", err)
	}
	if err := f.store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: f.activation.StreamStateRequest, ExpectedRevision: 1, Desired: filament.StreamEnabled}); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("stale revision: %v", err)
	}
	page, err := f.store.ListReconcileCandidates(ctx, filament.ReconcileQuery{Tenant: tenantA, Limit: 1})
	if err != nil || len(page.States) != 1 || page.States[0].Desired != filament.StreamPaused {
		t.Fatalf("reconcile: %+v %v", page, err)
	}
	other := f.activation.StreamStateRequest
	other.Tenant = filament.TenantID(uuid.NewString())
	if _, err := f.store.LoadStreamState(ctx, other); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("cross tenant: %v", err)
	}
	if err := f.store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: other, ExpectedRevision: 2, Desired: filament.StreamEnabled}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("cross tenant mutation: %v", err)
	}
}

func TestRuntimeExpiredAttemptCanBeReapedButNotMadeClean(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	if _, err := f.store.Pool().Exec(ctx, "UPDATE stream_attempts SET expires_at=clock_timestamp()-interval '1 second' WHERE token=$1", a.Lease.Attempt.Token); err != nil {
		t.Fatal(err)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptReaped}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptClean}); !errors.Is(err, filament.ErrFenced) {
		t.Fatalf("reaped became clean: %v", err)
	}
	req := f.request
	req.ExecutionID = uuid.NewString()
	if _, err := f.store.StartAttempt(ctx, req); !errors.Is(err, filament.ErrTakeoverBlocked) {
		t.Fatalf("reaped takeover: %v", err)
	}
}

func TestRuntimeIgnoresUninitializedReplicationStreams(t *testing.T) {
	ctx := context.Background()
	store, desired, _ := admissionFixture(t)
	stream, err := store.ResolveReplicationStream(ctx, desired)
	if err != nil {
		t.Fatal(err)
	}
	store.ConfigureStreamCodecs(&streamkit.Registry{})
	runtime := store
	req := filament.StreamStateRequest{Tenant: tenantA, Stream: filament.StreamRef{ID: stream.ID, Generation: stream.Generation}}
	if _, err := runtime.LoadStreamState(ctx, req); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("uninitialized state: %v", err)
	}
	page, err := runtime.ListReconcileCandidates(ctx, filament.ReconcileQuery{Tenant: tenantA, Limit: 10})
	if err != nil || len(page.States) != 0 {
		t.Fatalf("bounded stream entered reconciliation: %+v %v", page, err)
	}
	if err := runtime.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: req, Desired: filament.StreamEnabled, ExpectedRevision: 0}); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("intent initialized without execution inputs: %v", err)
	}
}

func TestRuntimeExecutionSnapshotSurvivesStoreRecreation(t *testing.T) {
	f := newRuntimeFixture(t)
	ctx := context.Background()
	reopened := postgres.New(f.store.Pool())
	reopened.ConfigureStreamCodecs(&streamkit.Registry{})
	state, err := reopened.ActivateStream(ctx, f.activation)
	if err != nil || state.Run != f.activation.Spec.Run || state.Revision != f.state.Revision {
		t.Fatalf("activation retry: %+v %v", state, err)
	}
	changed := f.activation
	changed.Spec.Resources = []string{"different"}
	if _, err := reopened.ActivateStream(ctx, changed); !errors.Is(err, filament.ErrVersionConflict) {
		t.Fatalf("replaced execution snapshot: %v", err)
	}
	attempt, err := reopened.StartAttempt(ctx, f.request)
	if err != nil || len(attempt.Spec.Resources) != 1 || attempt.Spec.Resources[0] != "events" {
		t.Fatalf("persisted execution inputs: %+v %v", attempt, err)
	}
}

func TestContinuousAdmissionPreservesConnectorIndependentSpec(t *testing.T) {
	store, desired, versions := admissionFixture(t)
	ctx := context.Background()
	run := admissionRun(versions[0], desired.Route, filament.RunRequested)
	run.Request.Options.Execution = filament.ExecutionContinuous
	run.Request.Source = filament.Ref{Connector: "other-source"}
	run.Request.Sink = filament.Ref{Connector: "other-sink"}
	run.Request.Resources = []string{"events", "audit"}
	run.Request.WritePolicies = map[string]filament.WritePolicy{
		"events": filament.IngestionFullAppend.WritePolicy(),
		"audit":  filament.IngestionFullAppend.WritePolicy(),
	}
	if err := store.CreateRunWithReplicationStream(ctx, run, desired); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.LoadRun(ctx, run.Tenant, run.Run)
	if err != nil {
		t.Fatal(err)
	}
	store.ConfigureStreamCodecs(&streamkit.Registry{})
	runtime := store
	request, err := loaded.StreamStateRequest()
	if err != nil {
		t.Fatal(err)
	}
	state, err := runtime.LoadStreamState(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Membership) != 2 {
		t.Fatalf("membership = %v", state.Membership)
	}
	attempt, err := runtime.StartAttempt(ctx, filament.StartAttemptRequest{Tenant: run.Tenant, Stream: request.Stream, Run: run.Run, ExecutionID: uuid.NewString(), PipelineVersionID: versions[0], Route: desired.Route, ExpectedRevision: state.Revision, TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if attempt.Spec.Source.Connector != "other-source" || attempt.Spec.Sink.Connector != "other-sink" || len(attempt.Spec.WritePolicies) != 2 {
		t.Fatalf("lost submitted execution: %+v", attempt.Spec)
	}
}
