package sinkstate

import (
	"errors"
	"sync"

	"github.com/galaxy-io/filament"
)

// Lifecycle guards serialized write boundaries. Apply calls may run in
// parallel, but must join before Commit, Abort, or Close. It provides no fencing.
type Lifecycle struct {
	mu                     sync.Mutex
	opened, active, closed bool
	failure, closeErr      error
	// resourceFailures holds failures confined to one resource. They block
	// that resource only; the session still applies and commits the rest.
	resourceFailures map[string]error
}

// ErrState rejects calls outside an open write boundary.
var ErrState = errors.New("sink: invalid write lifecycle state")

// Open records successful session setup.
func (s *Lifecycle) Open() { s.mu.Lock(); defer s.mu.Unlock(); s.opened = true }

// CheckBegin validates that a new write may start.
func (s *Lifecycle) CheckBegin() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || s.active || s.closed || s.failure != nil {
		return errors.Join(ErrState, s.failure)
	}
	return nil
}

// MarkBegun activates a write after destination setup succeeds.
func (s *Lifecycle) MarkBegun() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || s.active || s.closed || s.failure != nil {
		return errors.Join(ErrState, s.failure)
	}
	s.active = true
	s.resourceFailures = nil // a new write boundary admits every resource again
	return nil
}

// CheckApply rejects writes or commits outside a healthy active boundary.
func (s *Lifecycle) CheckApply() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active || s.closed || s.failure != nil {
		return errors.Join(ErrState, s.failure)
	}
	return nil
}

// CheckApplyResource is CheckApply for one resource: it also rejects a
// resource whose earlier write failed on its own.
func (s *Lifecycle) CheckApplyResource(resource string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active || s.closed || s.failure != nil {
		return errors.Join(ErrState, s.failure)
	}
	if err := s.resourceFailures[resource]; err != nil {
		return &filament.ResourceError{Resource: resource, Err: errors.Join(ErrState, err)}
	}
	return nil
}

// FailResource retains a failure confined to one resource and returns it as a
// *filament.ResourceError. Unlike Fail it leaves the session usable, so call it
// only when the sink isolates resources and the failure cannot have touched
// another resource's writes.
func (s *Lifecycle) FailResource(resource string, err error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.resourceFailures == nil {
		s.resourceFailures = map[string]error{}
	}
	if s.resourceFailures[resource] == nil {
		s.resourceFailures[resource] = err
	}
	var scoped *filament.ResourceError
	if errors.As(err, &scoped) && scoped.Resource == resource {
		return scoped
	}
	return &filament.ResourceError{Resource: resource, Err: err}
}

// ResourceFailure returns the retained failure for one resource.
func (s *Lifecycle) ResourceFailure(resource string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resourceFailures[resource]
}

// MarkCommitted ends the active write after durability is confirmed.
func (s *Lifecycle) MarkCommitted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.active || s.closed || s.failure != nil {
		return errors.Join(ErrState, s.failure)
	}
	s.active = false
	return nil
}

// CheckAbort permits cleanup even after a failed write.
func (s *Lifecycle) CheckAbort() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || s.closed {
		return ErrState
	}
	return nil
}

// MarkAborted ends a write without clearing an uncertain outcome.
func (s *Lifecycle) MarkAborted() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || s.closed {
		return ErrState
	}
	s.active = false
	return nil
}

// Fail retains the first error and prevents session reuse.
func (s *Lifecycle) Fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failure == nil {
		s.failure = err
	}
}

// Failure returns the retained write failure.
func (s *Lifecycle) Failure() error { s.mu.Lock(); defer s.mu.Unlock(); return s.failure }

// MarkClosed records the first shutdown result.
func (s *Lifecycle) MarkClosed(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		s.closeErr = err
	}
}

// Closed returns the recorded shutdown state and result.
func (s *Lifecycle) Closed() (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed, s.closeErr
}

// Opened reports whether session setup has succeeded, including after closure.
func (s *Lifecycle) Opened() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.opened }
