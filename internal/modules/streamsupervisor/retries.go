package streamsupervisor

import (
	"time"

	"github.com/galaxy-io/filament"
)

const maxRetries = 5

type retryState struct {
	run        filament.RunID
	revision   int64
	epoch      int64
	endedToken int64
	failures   int
}

// retryPolicy counts each failed attempt once. Counters are intentionally local
// to this supervisor; progress, changed intent, or a restart resets the budget.
func (s *Supervisor) retryPolicy(state filament.StreamState) (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := state.Stream.ID
	if state.Desired != filament.StreamEnabled {
		delete(s.retries, key)
		return 5 * time.Second, false
	}
	if s.retries == nil {
		s.retries = make(map[string]*retryState)
	}
	r := s.retries[key]
	if r == nil || r.run != state.Run || r.revision != state.Revision {
		r = &retryState{run: state.Run, revision: state.Revision}
		s.retries[key] = r
		// An attempt from before a manual intent change does not consume a retry.
		if state.Attempt != nil && state.Attempt.Revision != state.Revision {
			r.endedToken = state.Attempt.Lease.Attempt.Token
		}
	}
	if state.LastEpoch != nil && state.LastEpoch.Epoch != r.epoch {
		r.epoch = state.LastEpoch.Epoch
		r.failures = 0
	}
	a := state.Attempt
	if a != nil && a.EndedAt != nil && a.Lease.Attempt.Token != r.endedToken {
		r.endedToken = a.Lease.Attempt.Token
		if a.Reason == "" {
			r.failures = 0
		} else {
			r.failures++
		}
	}
	if r.failures > maxRetries {
		return 0, true
	}
	if r.failures < 1 {
		return 5 * time.Second, false
	}
	return 5 * time.Second * time.Duration(1<<(r.failures-1)), false
}
