package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/rowmodel"
)

type epochTestSink struct{ bound filament.EpochRef }

func (*epochTestSink) Spec() filament.SinkSpec                      { return filament.SinkSpec{Name: "epoch-test"} }
func (*epochTestSink) Name() string                                 { return "epoch-test" }
func (*epochTestSink) Open(context.Context, filament.RunSpec) error { return nil }
func (*epochTestSink) Commit(context.Context) error                 { return errors.New("not called") }

func (*epochTestSink) Abort(context.Context) error { return errors.New("not called") }

func (s *epochTestSink) Apply(_ context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if err := s.bound.ValidateApply(opts); err != nil {
		return filament.WriteReceipt{}, err
	}
	return filament.WriteReceipt{WriteCRC: b.IntegrityCRC()}, nil
}

// Real pipeline completion, not fabricated exported evidence, backs certificates.
func (f runtimeFixture) epoch(t *testing.T, a filament.Attempt, epoch int64, value string) filament.EpochCommit {
	return f.epochPosition(t, a, epoch, rowmodel.Position{Codec: "counter", Value: []byte(value)}, "source")
}

func (f runtimeFixture) epochPosition(t *testing.T, a filament.Attempt, epoch int64, position rowmodel.Position, incarnation string, count ...int) filament.EpochCommit {
	t.Helper()
	ref := filament.EpochRef{Attempt: a.Lease.Attempt, Epoch: epoch, MembershipRevision: f.state.MembershipRevision}
	p, err := pipeline.NewStream(pipeline.Config{Sink: &epochTestSink{bound: ref}, WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullReplace)}}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(context.Background())
	if err := p.BindEpoch(ref); err != nil {
		t.Fatal(err)
	}
	var records int64
	var receipts []filament.EpochReceipt
	if len(count) > 0 && count[0] > 0 {
		w, err := p.Records().Builder("events", 0, rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}})
		if err != nil {
			t.Fatal(err)
		}
		for i := range count[0] {
			w.Int64(int64(i))
			if err := w.EndRow(rowmodel.Meta{}); err != nil {
				t.Fatal(err)
			}
		}
		records = int64(count[0])
		receipts = []filament.EpochReceipt{{Resource: "events", Rows: records}}
	}
	c := rowmodel.Control{Kind: rowmodel.ProgressBoundary, Domain: rowmodel.DomainKey{Incarnation: incarnation, Domain: "log"}, Position: position}
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
	return filament.EpochCommit{Certificate: filament.EpochCertificate{FormatVersion: filament.EpochCertificateFormatVersion, Tenant: tenantA, Ref: ref, PipelineVersionID: f.request.PipelineVersionID, Records: records, Receipts: receipts, Coverage: filament.Coverage{Positions: filament.DomainPositions{c.Domain: c.Position}}}, Completion: completion}
}

func TestRuntimeEpochAtomicityIdempotencyAndRecovery(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	req := f.epoch(t, a, 1, "00010")
	first, err := f.store.CommitEpoch(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	// Lost response recovery does not need process-local completion or live authority.
	req.Completion = filament.EpochCompletion{}
	if _, err := f.store.Pool().Exec(ctx, "UPDATE stream_attempts SET expires_at=clock_timestamp()-interval '1 second' WHERE token=$1", a.Lease.Attempt.Token); err != nil {
		t.Fatal(err)
	}
	retry, err := f.store.CommitEpoch(ctx, req)
	if err != nil || !retry.CommittedAt.Equal(first.CommittedAt) {
		t.Fatalf("retry: %+v %v", retry, err)
	}
	historical, err := f.store.GetEpoch(ctx, filament.EpochLookup{Tenant: tenantA, Key: req.Certificate.Ref.Key()})
	if err != nil || !historical.CommittedAt.Equal(first.CommittedAt) {
		t.Fatalf("lookup: %+v %v", historical, err)
	}
	state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || state.LastEpoch.Epoch != 1 || string(state.CommittedPositions[rowmodel.DomainKey{Incarnation: "source", Domain: "log"}].Value) != "10" {
		t.Fatalf("state: %+v %v", state, err)
	}
	req.Certificate.Bytes++
	if _, err := f.store.CommitEpoch(ctx, req); !errors.Is(err, filament.ErrEpochConflict) {
		t.Fatalf("conflicting retry: %v", err)
	}
}

func TestRuntimeRejectsUnsealedOrExpandedCoverage(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	for _, mode := range []string{"missing", "expanded", "epoch", "claims"} {
		req := f.epoch(t, a, 1, "10")
		switch mode {
		case "missing":
			req.Completion = filament.EpochCompletion{}
		case "expanded":
			for d, p := range req.Certificate.Coverage.Positions {
				p.Value = []byte("100")
				req.Certificate.Coverage.Positions[d] = p
			}
		case "epoch":
			req.Certificate.Ref.Epoch = 2
		case "claims":
			req.Certificate.Coverage = filament.Coverage{Claims: []filament.InboxClaimRef{{RowID: "one", Owner: "owner", Token: 1}}}
		}
		if _, err := f.store.CommitEpoch(ctx, req); !errors.Is(err, filament.ErrIncompleteCoverage) {
			t.Fatalf("%s: %v", mode, err)
		}
	}
	state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || state.LastEpoch != nil || len(state.CommittedPositions) != 0 {
		t.Fatalf("rejected commit changed state: %+v %v", state, err)
	}
}

func TestRuntimeEpochProgressAndMembershipGuards(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	if _, err := f.store.CommitEpoch(ctx, f.epoch(t, a, 1, "10")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CommitEpoch(ctx, f.epoch(t, a, 2, "9")); !errors.Is(err, filament.ErrPositionRegression) {
		t.Fatalf("regression: %v", err)
	}
	if _, err := f.store.CommitEpoch(ctx, f.epoch(t, a, 3, "11")); !errors.Is(err, filament.ErrEpochConflict) {
		t.Fatalf("epoch gap: %v", err)
	}
	req := f.epoch(t, a, 2, "12")
	if _, err := f.store.Pool().Exec(ctx, "UPDATE replication_stream_resources SET status=3 WHERE replication_stream_id=$1", f.request.Stream.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CommitEpoch(ctx, req); !errors.Is(err, filament.ErrMembershipChanged) {
		t.Fatalf("membership: %v", err)
	}
}

func TestRuntimeEpochConcurrentConflict(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	reqs := []filament.EpochCommit{f.epoch(t, a, 1, "10"), f.epoch(t, a, 1, "11")}
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, req := range reqs {
		go func() { <-start; _, err := f.store.CommitEpoch(ctx, req); results <- err }()
	}
	close(start)
	successes, conflicts := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
		} else if errors.Is(err, filament.ErrEpochConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d", successes, conflicts)
	}
}

func TestRuntimeEpochRollbackAfterCertificateInsert(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	// A failing epoch advance must roll back the preceding certificate insert.
	_, err := f.store.Pool().Exec(ctx, "ALTER TABLE replication_streams ADD CONSTRAINT test_reject_position CHECK (last_epoch <> 1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = f.store.Pool().Exec(ctx, "ALTER TABLE replication_streams DROP CONSTRAINT test_reject_position")
	}()
	req := f.epoch(t, a, 1, "10")
	if _, err := f.store.CommitEpoch(ctx, req); err == nil {
		t.Fatal("injected write failure succeeded")
	}
	if _, err := f.store.GetEpoch(ctx, filament.EpochLookup{Tenant: tenantA, Key: req.Certificate.Ref.Key()}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("partial certificate survived: %v", err)
	}
	state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || state.LastEpoch != nil || len(state.CommittedPositions) != 0 {
		t.Fatalf("partial progress: %+v %v", state, err)
	}
	if _, err := f.store.Pool().Exec(ctx, "ALTER TABLE replication_streams DROP CONSTRAINT test_reject_position"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CommitEpoch(ctx, req); err != nil {
		t.Fatal(fmt.Errorf("retry after rollback: %w", err))
	}
}

func TestRuntimeRejectsIncomparablePositions(t *testing.T) {
	for _, mode := range []string{"incarnation", "codec", "opaque"} {
		t.Run(mode, func(t *testing.T) {
			f := newRuntimeFixture(t)
			a := f.start(t)
			ctx := context.Background()
			pos := rowmodel.Position{Codec: "counter", Value: []byte("1")}
			if mode == "opaque" {
				pos = rowmodel.Position{Codec: "opaque", Value: []byte("a")}
			}
			if _, err := f.store.CommitEpoch(ctx, f.epochPosition(t, a, 1, pos, "source")); err != nil {
				t.Fatal(err)
			}
			incarnation := "source"
			switch mode {
			case "incarnation":
				incarnation = "replacement"
			case "codec":
				pos.Codec = "opaque"
			case "opaque":
				pos.Value = []byte("b")
			}
			if _, err := f.store.CommitEpoch(ctx, f.epochPosition(t, a, 2, pos, incarnation)); !errors.Is(err, filament.ErrPositionIncomparable) {
				t.Fatalf("%s: %v", mode, err)
			}
		})
	}
}

func TestRuntimeCertificateAuthorityAndTenantChecks(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	req := f.epoch(t, a, 1, "10")
	other := filament.TenantID(uuid.NewString())
	lease := a.Lease
	lease.Tenant = other
	if err := f.store.RenewLease(ctx, lease, time.Minute); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("tenant renewal: %v", err)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: lease, Termination: filament.AttemptClean}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("tenant end: %v", err)
	}
	req.Certificate.Tenant = other
	if _, err := f.store.CommitEpoch(ctx, req); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("tenant commit: %v", err)
	}
	req.Certificate.Tenant = tenantA
	if _, err := f.store.GetEpoch(ctx, filament.EpochLookup{Tenant: other, Key: req.Certificate.Ref.Key()}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("tenant lookup: %v", err)
	}
	if _, err := f.store.Pool().Exec(ctx, "UPDATE stream_attempts SET expires_at=clock_timestamp()-interval '1 second' WHERE token=$1", a.Lease.Attempt.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.CommitEpoch(ctx, req); !errors.Is(err, filament.ErrLeaseExpired) {
		t.Fatalf("expired new certificate: %v", err)
	}
}

func TestRuntimeGenerationReplacement(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	old := f.epoch(t, a, 1, "10")
	// Free the legacy run admission row; this alone must not prove quiescence.
	if _, err := f.store.TransitionRun(ctx, tenantA, a.Lease.Attempt.RunID, []filament.RunStatus{filament.RunRequested}, filament.RunCompleted, filament.RunTransitionOptions{Ended: true}); err != nil {
		t.Fatal(err)
	}
	desired, err := f.store.LoadReplicationStream(ctx, f.request.Stream.ID)
	if err != nil {
		t.Fatal(err)
	}
	desired.ID = uuid.NewString()
	desired.ConsumerName = "replacement"
	desired.ContinuityFingerprint = "different"
	replacement, err := f.store.ResolveReplicationStream(ctx, desired)
	if err != nil {
		t.Fatal(err)
	}
	run := admissionRun(f.request.PipelineVersionID, f.request.Route, filament.RunRequested)
	run.Request.Options.Execution = filament.ExecutionContinuous
	run.Request.ReplicationStream = &filament.StreamRef{ID: replacement.ID, Generation: replacement.Generation}
	if err := f.store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	activation := f.activation
	activation.Stream = *run.Request.ReplicationStream
	activation.Spec.Run = run.Run
	activation.Spec.ReplicationStream = run.Request.ReplicationStream
	state, err := f.store.ActivateStream(ctx, activation)
	if err != nil {
		t.Fatal(err)
	}
	request := f.request
	request.Stream = activation.Stream
	request.Run = run.Run
	request.ExecutionID = uuid.NewString()
	request.ExpectedRevision = state.Revision
	if _, err := f.store.StartAttempt(ctx, request); !errors.Is(err, filament.ErrTakeoverBlocked) {
		t.Fatalf("unproven predecessor admitted: %v", err)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptClean}); err != nil {
		t.Fatal(err)
	}
	b, err := f.store.StartAttempt(ctx, request)
	if err != nil || b.Lease.Attempt.Token <= a.Lease.Attempt.Token {
		t.Fatalf("replacement: %+v %v", b, err)
	}
	if err := f.store.RenewLease(ctx, a.Lease, time.Minute); !errors.Is(err, filament.ErrFenced) {
		t.Fatalf("retired renewal: %v", err)
	}
	if _, err := f.store.CommitEpoch(ctx, old); !errors.Is(err, filament.ErrFenced) {
		t.Fatalf("retired certificate: %v", err)
	}
}

func TestRuntimeMembershipNoOpAndConcurrentCommit(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	ctx := context.Background()
	if _, err := f.store.ReconcileReplicationStreamResources(ctx, f.request.Stream.ID, tenantA, []string{"events"}, "none"); err != nil {
		t.Fatal(err)
	}
	state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || state.MembershipRevision != f.state.MembershipRevision {
		t.Fatalf("no-op membership revision changed: %d -> %d %v", f.state.MembershipRevision, state.MembershipRevision, err)
	}
	req := f.epoch(t, a, 1, "10")
	tx, err := f.store.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "UPDATE replication_stream_resources SET status=3 WHERE replication_stream_id=$1", f.request.Stream.ID); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { _, err := f.store.CommitEpoch(ctx, req); result <- err }()
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, filament.ErrMembershipChanged) {
		t.Fatalf("concurrent membership commit: %v", err)
	}
}

func TestRuntimePositionNullVersusEmpty(t *testing.T) {
	for _, value := range [][]byte{nil, {}} {
		t.Run(fmt.Sprintf("nil=%v", value == nil), func(t *testing.T) {
			f := newRuntimeFixture(t)
			a := f.start(t)
			ctx := context.Background()
			req := f.epochPosition(t, a, 1, rowmodel.Position{Codec: "opaque", Value: value}, "source")
			if _, err := f.store.CommitEpoch(ctx, req); err != nil {
				t.Fatal(err)
			}
			state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
			if err != nil {
				t.Fatal(err)
			}
			got := state.CommittedPositions[rowmodel.DomainKey{Incarnation: "source", Domain: "log"}].Value
			if (got == nil) != (value == nil) {
				t.Fatal("position null/empty distinction lost")
			}
			epoch, err := f.store.GetEpoch(ctx, filament.EpochLookup{Tenant: tenantA, Key: req.Certificate.Ref.Key()})
			if err != nil {
				t.Fatal(err)
			}
			got = epoch.Certificate.Coverage.Positions[rowmodel.DomainKey{Incarnation: "source", Domain: "log"}].Value
			if (got == nil) != (value == nil) {
				t.Fatal("certificate null/empty distinction lost")
			}
		})
	}
}

func TestRuntimeEpochWithAppliedRows(t *testing.T) {
	f := newRuntimeFixture(t)
	a := f.start(t)
	req := f.epochPosition(t, a, 1, rowmodel.Position{Codec: "counter", Value: []byte("2")}, "source", 2)
	saved, err := f.store.CommitEpoch(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Certificate.Records != 2 || len(saved.Certificate.Receipts) != 1 || saved.Certificate.Receipts[0].Resource != "events" {
		t.Fatalf("lost receipt: %+v", saved.Certificate)
	}
}

func TestRuntimeFrozenExecutionIgnoresRunVersionUpdates(t *testing.T) {
	f := newRuntimeFixture(t)
	ctx := context.Background()
	// Simulate an existing run writer changing its version after initialization.
	_, err := f.store.Pool().Exec(ctx, `UPDATE runs SET pipeline_version_id=(SELECT id FROM pipeline_versions WHERE pipeline_id=$1 AND id<>$2 LIMIT 1) WHERE id=$3`, pipelineOne, f.request.PipelineVersionID, string(f.request.Run))
	if err != nil {
		t.Fatal(err)
	}
	state, err := f.store.ActivateStream(ctx, f.activation)
	if err != nil || state.PipelineVersionID != f.request.PipelineVersionID {
		t.Fatalf("snapshot retry: %+v %v", state, err)
	}
	a := f.start(t)
	if a.Spec.PipelineVersionID != f.request.PipelineVersionID {
		t.Fatal("attempt used mutable run version")
	}
	if _, err := f.store.CommitEpoch(ctx, f.epoch(t, a, 1, "10")); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeRetainsStoppedRun(t *testing.T) {
	f := newRuntimeFixture(t)
	ctx := context.Background()
	a := f.start(t)
	if _, err := f.store.CommitEpoch(ctx, f.epoch(t, a, 1, "10")); err != nil {
		t.Fatal(err)
	}
	if err := f.store.EndAttempt(ctx, filament.EndAttemptRequest{Lease: a.Lease, Termination: filament.AttemptClean}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SetDesiredState(ctx, filament.DesiredStateChange{StreamStateRequest: f.activation.StreamStateRequest, Desired: filament.StreamStopped, ExpectedRevision: f.state.Revision}); err != nil {
		t.Fatal(err)
	}
	var sqlErr interface{ SQLState() string }
	if err := f.store.DeleteRun(ctx, tenantA, f.request.Run); !errors.As(err, &sqlErr) || sqlErr.SQLState() != "23503" {
		t.Fatalf("referenced run deletion: %v", err)
	}
	state, err := f.store.LoadStreamState(ctx, f.activation.StreamStateRequest)
	if err != nil || state.LastEpoch == nil || state.LastEpoch.Epoch != 1 || state.Desired != filament.StreamStopped {
		t.Fatalf("lost stopped state: %+v %v", state, err)
	}
}
