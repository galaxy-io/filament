package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
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
