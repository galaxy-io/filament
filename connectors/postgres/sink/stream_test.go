package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestNativeEpochCommitAbortAndBoundedReplace(t *testing.T)     { nativeEpochScenario(t, false) }
func TestNativeEpochApplyFailurePreservesPriorCommit(t *testing.T) { nativeEpochScenario(t, true) }
func nativeEpochScenario(t *testing.T, failSecond bool) {
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set FILAMENT_TEST_POSTGRES_DSN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	schema := "epochs_" + uuid.NewString()[:8]
	defer func() {
		_, _ = conn.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	}()
	attempt := filament.AttemptRef{RunID: "run", ExecutionID: "worker", StreamID: "stream", Generation: 1, Token: 1}
	appendPolicy := filament.WritePolicy{Capability: filament.WriteCapabilities(filament.IngestionFullAppend)[0]}
	spec := filament.RunSpec{Run: "run", StreamAttempt: &attempt, Options: filament.RunOptions{Execution: filament.ExecutionContinuous}, Resources: []string{"events"}, Sink: filament.Ref{Config: map[string]any{"dsn": dsn, "schema": schema}}, WritePolicies: map[string]filament.WritePolicy{"events": appendPolicy}}
	sink := New()
	if err := sink.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer sink.CloseSession(ctx)
	fields := rowmodel.Schema{Resource: "events", Fields: []rowmodel.Field{{Name: "value", Logical: rowmodel.LogicalString}}}
	if failSecond {
		fields.PrimaryKey = []string{"value"}
	}
	if err := sink.EnsureSchema(ctx, "events", fields); err != nil {
		t.Fatal(err)
	}
	p, err := pipeline.NewStream(pipeline.Config{Sink: sink, WritePolicies: spec.WritePolicies}, filament.OrderingNone)
	if err != nil {
		t.Fatal(err)
	}
	p.Start(ctx)
	expectFailure := false
	defer func() {
		p.CloseIngest(nil)
		if err := p.Wait(); err != nil && !expectFailure {
			t.Error(err)
		}
	}()
	records := p.Records().(filament.StreamRecordSink)
	writer, err := records.Builder("events", 0, fields)
	if err != nil {
		t.Fatal(err)
	}
	count := func(want int) {
		t.Helper()
		var n int
		if err := conn.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{schema, "events"}.Sanitize()).Scan(&n); err != nil || n != want {
			t.Fatalf("rows %d want %d: %v", n, want, err)
		}
	}
	for epoch := int64(1); epoch <= 2; epoch++ {
		ref := filament.EpochRef{Attempt: attempt, Epoch: epoch, MembershipRevision: 1}
		if err := p.BindEpoch(ref); err != nil {
			t.Fatal(err)
		}
		if err := sink.BeginEpoch(ctx, ref); err != nil {
			t.Fatal(err)
		}
		writer.String("value")
		if err := writer.EndRow(rowmodel.Meta{}); err != nil {
			t.Fatal(err)
		}
		controlErr := records.Control(ctx, filament.Control{Kind: filament.ProgressBoundary, Domain: filament.DomainKey{Domain: "log", Incarnation: "source"}, Position: filament.Position{Codec: "opaque", Value: []byte("x")}})
		if failSecond && epoch == 2 {
			if controlErr == nil {
				t.Fatal("expected duplicate key Apply failure")
			}
			expectFailure = true
			count(1)
			if err := sink.CloseSession(ctx); err == nil {
				t.Fatal("failed epoch reported clean closure")
			}
			count(1)
			return
		}
		if controlErr != nil {
			t.Fatal(controlErr)
		}
		if _, err := p.SealEpoch(); err != nil {
			t.Fatal(err)
		}
		if epoch == 1 {
			count(0)
			r, err := sink.CommitEpoch(ctx, ref)
			if err != nil || len(r) != 1 || r[0].Rows != 1 {
				t.Fatalf("receipts %+v: %v", r, err)
			}
			if _, err := sink.CommitEpoch(ctx, ref); err != nil {
				t.Fatal(err)
			}
		} else {
			count(1)
			if err := sink.AbortEpoch(ctx, ref); err != nil {
				t.Fatal(err)
			}
		}
		count(1)
	}
	if err := sink.Commit(ctx); !errors.Is(err, filament.ErrEpochMismatch) {
		t.Fatal("bounded lifecycle accepted stream", err)
	}
	if err := sink.CloseSession(ctx); err != nil {
		t.Fatal(err)
	}
	// Existing bounded replacement still truncates at EnsureSchema.
	bounded := New()
	spec.Options.Execution = filament.ExecutionBounded
	spec.StreamAttempt = nil
	spec.WritePolicies["events"] = filament.WritePolicy{Capability: filament.WriteCapabilities(filament.IngestionFullReplace)[0]}
	if err := bounded.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer bounded.Commit(ctx)
	if err := bounded.EnsureSchema(ctx, "events", fields); err != nil {
		t.Fatal(err)
	}
	count(0)
}
