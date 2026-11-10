package streamsupervisor

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/runner"
	"github.com/galaxy-io/filament/version"
)

type upgradeTestStore struct {
	filament.ContinuousRunStore
	runs    []filament.RunState
	states  map[filament.RunID]filament.StreamState
	yielded []filament.LeaseToken
}

func (s *upgradeTestStore) PendingStreamRuns(context.Context, string, int) ([]filament.RunState, error) {
	return s.runs, nil
}

func (s *upgradeTestStore) LoadStreamState(_ context.Context, req filament.StreamStateRequest) (filament.StreamState, error) {
	for _, state := range s.states {
		if state.Stream == req.Stream {
			return state, nil
		}
	}
	return filament.StreamState{}, filament.ErrNotFound
}

func (s *upgradeTestStore) YieldAttempt(_ context.Context, lease filament.LeaseToken) error {
	s.yielded = append(s.yielded, lease)
	for run, state := range s.states {
		if state.Attempt != nil && state.Attempt.Lease == lease {
			state.Attempt.YieldRequested = true
			s.states[run] = state
		}
	}
	return nil
}

func liveRun(id, dispatchVersion string) (filament.RunState, filament.StreamState) {
	stream := filament.StreamRef{ID: "stream-" + id, Generation: 1}
	ref := filament.AttemptRef{RunID: filament.RunID(id), ExecutionID: "exec-" + id, StreamID: stream.ID, Generation: 1, Token: 1}
	run := filament.RunState{Run: filament.RunID(id), Tenant: "tenant", Request: filament.RunRequest{ReplicationStream: &stream}}
	state := filament.StreamState{
		StreamStateRequest: filament.StreamStateRequest{Tenant: "tenant", Stream: stream},
		Desired:            filament.StreamEnabled,
		Revision:           1,
		Run:                filament.RunID(id),
		Attempt: &filament.Attempt{
			Claimed:         true,
			DispatchVersion: dispatchVersion,
			Revision:        1,
			Lease:           filament.LeaseToken{Tenant: "tenant", Attempt: ref},
			ExpiresAt:       time.Now().Add(time.Minute),
		},
	}
	return run, state
}

func TestReconcileYieldsOneStaleAttemptPerPass(t *testing.T) {
	store := &upgradeTestStore{states: map[filament.RunID]filament.StreamState{}}
	for id, v := range map[string]string{"a": "old", "b": "old", "c": version.Version} {
		run, state := liveRun(id, v)
		store.runs = append(store.runs, run)
		store.states[run.Run] = state
	}
	s := New(runner.Deps{StreamStore: store}, nil)
	ctx := context.Background()
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if len(store.yielded) != 1 {
		t.Fatalf("first pass yielded %d attempts, want 1", len(store.yielded))
	}
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if len(store.yielded) != 2 || store.yielded[0] == store.yielded[1] {
		t.Fatalf("second pass yielded %v, want the other stale attempt", store.yielded)
	}
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if len(store.yielded) != 2 {
		t.Fatalf("third pass yielded again: %v", store.yielded)
	}
	for _, lease := range store.yielded {
		if lease.Attempt.RunID == "c" {
			t.Fatal("an attempt on the current release was yielded")
		}
	}
}

func TestRetryPolicyRestartsYieldedAttemptAtOnce(t *testing.T) {
	s := New(runner.Deps{}, nil)
	ended := time.Now()
	_, state := liveRun("a", "old")
	state.Attempt.EndedAt = &ended
	state.Attempt.Termination = filament.AttemptClean
	state.Attempt.Reason = filament.AttemptReasonUpgrade
	delay, exhausted := s.retryPolicy(state)
	if delay != 0 || exhausted {
		t.Fatalf("yielded attempt: delay %s exhausted %v", delay, exhausted)
	}
	state.Attempt.Reason = "network unavailable"
	state.Attempt.Lease.Attempt.Token++
	if delay, _ := s.retryPolicy(state); delay != 5*time.Second {
		t.Fatalf("first failure after a roll: delay %s, want 5s", delay)
	}
}
