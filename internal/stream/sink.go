package stream

import (
	"errors"
	"math"

	"github.com/galaxy-io/filament"
)

// SinkLifecycle tracks one admitted sink session. Calls, including each Check/Mark
// pair and its intervening connector operation, must be serialized. Construct
// with NewSinkLifecycle; the zero value rejects epoch operations.
//
// Connectors retain their transaction or publisher state. They must establish
// durability before MarkCommitted and perform cleanup before MarkAborted or
// MarkClosed. A failed lifecycle alone does not establish destination quiescence.
type SinkLifecycle struct {
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

// NewSinkLifecycle binds a lifecycle to an admitted attempt.
func NewSinkLifecycle(attempt filament.AttemptRef) (SinkLifecycle, error) {
	if err := attempt.Validate(); err != nil {
		return SinkLifecycle{}, err
	}
	return SinkLifecycle{attempt: attempt}, nil
}

func (s *SinkLifecycle) mismatch() error { return errors.Join(filament.ErrEpochMismatch, s.failure) }

// CheckBegin permits a resumed session to start at any positive epoch, then
// requires epochs immediately following the last successful commit.
func (s *SinkLifecycle) CheckBegin(ref filament.EpochRef) error {
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
func (s *SinkLifecycle) MarkBegun(ref filament.EpochRef) error {
	if err := s.CheckBegin(ref); err != nil {
		return err
	}
	s.active = &ref
	return nil
}

// CheckApply requires an exact binding to the active, healthy epoch.
func (s *SinkLifecycle) CheckApply(ref *filament.EpochRef) error {
	if s.closed || s.failure != nil || ref == nil || s.active == nil || *ref != *s.active {
		return s.mismatch()
	}
	return nil
}

// CheckCommit returns independently owned cached receipts and replay=true for
// a repeated successful commit while idle. This cache provides no cross-session
// deduplication and never authorizes replaying destination writes.
func (s *SinkLifecycle) CheckCommit(ref filament.EpochRef) ([]filament.EpochReceipt, bool, error) {
	if !s.closed && s.failure == nil && s.active == nil && s.last != nil && s.last.ref == ref {
		return cloneReceipts(s.last.receipts), true, nil
	}
	return nil, false, s.CheckApply(&ref)
}

// MarkCommitted caches receipts only after connector-confirmed durability.
func (s *SinkLifecycle) MarkCommitted(ref filament.EpochRef, receipts []filament.EpochReceipt) error {
	if err := s.CheckApply(&ref); err != nil {
		return err
	}
	s.last = &committedEpoch{ref: ref, receipts: cloneReceipts(receipts)}
	s.active = nil
	return nil
}

// Fail permanently poisons the session, preserving the first cause and allowing cleanup.
func (s *SinkLifecycle) Fail(err error) {
	if !s.closed && s.failure == nil {
		s.failure = err
	}
}

// Failure returns the first session failure; it is not a cleanup result.
func (s *SinkLifecycle) Failure() error { return s.failure }

// CheckAbort allows failed-session cleanup. With no active epoch, an admitted
// reference is an idempotent no-op. Closed sessions reject all epoch operations.
func (s *SinkLifecycle) CheckAbort(ref filament.EpochRef) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	if s.closed || ref.Attempt != s.attempt || (s.active != nil && *s.active != ref) {
		return s.mismatch()
	}
	return nil
}

// MarkAborted clears active work after successful cleanup, retaining any failure.
func (s *SinkLifecycle) MarkAborted(ref filament.EpochRef) error {
	if err := s.CheckAbort(ref); err != nil {
		return err
	}
	s.active = nil
	return nil
}

// MarkClosed blocks epoch operations and retains the connector's cleanup result.
// A nil result asserts no later destination effects are possible; the helper
// cannot prove that from local state or client connection closure alone.
func (s *SinkLifecycle) MarkClosed(err error) {
	if !s.closed {
		s.closed, s.closeErr = true, err
	}
}

// Closed returns the retained result for idempotent connector closure.
func (s *SinkLifecycle) Closed() (bool, error) { return s.closed, s.closeErr }

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
