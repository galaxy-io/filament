// Package stream provides shared sink epoch lifecycle bookkeeping.
// It does not implement destination fencing, transactions, or durability.
package stream

import (
	"errors"
	"math"

	"github.com/galaxy-io/filament"
)

// Lifecycle tracks one admitted sink session. Calls, including each Check/Mark
// pair and its intervening connector operation, must be serialized. Construct
// with New; the zero value rejects epoch operations.
//
// Connectors retain their transaction or publisher state. They must establish
// durability before MarkCommitted and perform cleanup before MarkAborted or
// MarkClosed. A failed lifecycle alone does not establish destination quiescence.
type Lifecycle struct {
	attempt  filament.AttemptRef
	active   *filament.EpochRef
	last     *committedEpoch
	failure  error
	closed   bool
	closeErr error
}

type committedEpoch struct {
	ref      filament.EpochRef
	receipts []filament.EpochReceipt
}

// New binds a lifecycle to an admitted attempt.
func New(attempt filament.AttemptRef) (Lifecycle, error) {
	if err := attempt.Validate(); err != nil {
		return Lifecycle{}, err
	}
	return Lifecycle{attempt: attempt}, nil
}

func (s *Lifecycle) mismatch() error { return errors.Join(filament.ErrEpochMismatch, s.failure) }

// CheckBegin permits a resumed session to start at any positive epoch, then
// requires epochs immediately following the last successful commit.
func (s *Lifecycle) CheckBegin(ref filament.EpochRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	if s.closed || s.failure != nil || s.active != nil || ref.Attempt != s.attempt {
		return s.mismatch()
	}
	if s.last != nil && (s.last.ref.Epoch == math.MaxInt64 || ref.Epoch != s.last.ref.Epoch+1) {
		return s.mismatch()
	}
	return nil
}

// MarkBegun records successful connector setup; a failed setup need not advance state.
func (s *Lifecycle) MarkBegun(ref filament.EpochRef) error {
	if err := s.CheckBegin(ref); err != nil {
		return err
	}
	s.active = &ref
	return nil
}

// CheckApply requires an exact binding to the active, healthy epoch.
func (s *Lifecycle) CheckApply(ref *filament.EpochRef) error {
	if s.closed || s.failure != nil || ref == nil || s.active == nil || *ref != *s.active {
		return s.mismatch()
	}
	return nil
}

// CheckCommit returns independently owned cached receipts and replay=true for
// a repeated successful commit while idle. This cache provides no cross-session
// deduplication and never authorizes replaying destination writes.
func (s *Lifecycle) CheckCommit(ref filament.EpochRef) ([]filament.EpochReceipt, bool, error) {
	if !s.closed && s.failure == nil && s.active == nil && s.last != nil && s.last.ref == ref {
		return cloneReceipts(s.last.receipts), true, nil
	}
	return nil, false, s.CheckApply(&ref)
}

// MarkCommitted caches receipts only after connector-confirmed durability.
func (s *Lifecycle) MarkCommitted(ref filament.EpochRef, receipts []filament.EpochReceipt) error {
	if err := s.CheckApply(&ref); err != nil {
		return err
	}
	s.last = &committedEpoch{ref: ref, receipts: cloneReceipts(receipts)}
	s.active = nil
	return nil
}

// Fail permanently poisons the session, preserving the first cause and allowing cleanup.
func (s *Lifecycle) Fail(err error) {
	if !s.closed && s.failure == nil {
		s.failure = err
	}
}

// Failure returns the first session failure; it is not a cleanup result.
func (s *Lifecycle) Failure() error { return s.failure }

// CheckAbort allows failed-session cleanup. With no active epoch, an admitted
// reference is an idempotent no-op. Closed sessions reject all epoch operations.
func (s *Lifecycle) CheckAbort(ref filament.EpochRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	if s.closed || ref.Attempt != s.attempt || (s.active != nil && *s.active != ref) {
		return s.mismatch()
	}
	return nil
}

// MarkAborted clears active work after successful cleanup, retaining any failure.
func (s *Lifecycle) MarkAborted(ref filament.EpochRef) error {
	if err := s.CheckAbort(ref); err != nil {
		return err
	}
	s.active = nil
	return nil
}

// MarkClosed blocks epoch operations and retains the connector's cleanup result.
// A nil result asserts no later destination effects are possible; the helper
// cannot prove that from local state or client connection closure alone.
func (s *Lifecycle) MarkClosed(err error) {
	if !s.closed {
		s.closed, s.closeErr = true, err
	}
}

// Closed returns the retained result for idempotent connector closure.
func (s *Lifecycle) Closed() (bool, error) { return s.closed, s.closeErr }

func cloneReceipts(in []filament.EpochReceipt) []filament.EpochReceipt {
	if in == nil {
		return nil
	}
	out := append([]filament.EpochReceipt{}, in...)
	for i := range out {
		out[i].Evidence = out[i].Evidence.Clone()
	}
	return out
}
