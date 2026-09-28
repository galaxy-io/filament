package stream

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/galaxy-io/filament"
)

var (
	// ErrSourceClosed indicates that the source session has been closed.
	ErrSourceClosed = errors.New("stream: source session closed")
	// ErrSourceUninitialized indicates that the source lifecycle was not initialized.
	ErrSourceUninitialized = errors.New("stream: source lifecycle not initialized")
	// ErrAcknowledgementPending indicates that outstanding coverage must be acknowledged before another read.
	ErrAcknowledgementPending = errors.New("stream: acknowledge pending coverage before reading again")
)

// SourceLifecycle guards a serialized read/ack source session with at most one
// outstanding coverage. Construct with NewSourceLifecycle; the zero value rejects
// operations. Calls and intervening provider operations must be serialized.
//
// Only the coordinator certifies coverage. This helper matches acknowledgements
// to previously returned coverage and checks coordinator authority, but cannot
// prove certification, provider ownership, continuity, or safe replay. Connectors
// retain message handles, resume positions, provider identity checks, and heartbeat
// synchronization. On partial read failures, connectors must Fail the session.
type SourceLifecycle struct {
	checkAuthority func(context.Context) error
	pending        *filament.Coverage
	failure        error
	closed         bool
	closeErr       error
}

// NewSourceLifecycle validates admission identity and requires a live authority
// callback. The callback must be bound to this attempt by the coordinator.
func NewSourceLifecycle(attempt filament.AttemptRef, checkAuthority func(context.Context) error) (SourceLifecycle, error) {
	if err := attempt.Validate(); err != nil {
		return SourceLifecycle{}, err
	}
	if checkAuthority == nil {
		return SourceLifecycle{}, errors.New("stream: coordinator authority check required")
	}
	return SourceLifecycle{checkAuthority: checkAuthority}, nil
}

func (s *SourceLifecycle) checkOpen() error {
	if s.closed {
		return errors.Join(ErrSourceClosed, s.failure)
	}
	if s.failure != nil {
		return s.failure
	}
	if s.checkAuthority == nil {
		return ErrSourceUninitialized
	}
	return nil
}

// CheckAuthority must precede provider acknowledgements, including already-
// certified redeliveries suppressed during Read. It does not replace provider
// incarnation/ownership checks or authorize acknowledging tentative input.
func (s *SourceLifecycle) CheckAuthority(ctx context.Context) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if err := s.checkAuthority(ctx); err != nil {
		return err
	}
	return context.Cause(ctx)
}

// CheckRead rejects new reads until outstanding coverage is acknowledged.
func (s *SourceLifecycle) CheckRead(ctx context.Context) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if s.pending != nil {
		return ErrAcknowledgementPending
	}
	return s.CheckAuthority(ctx)
}

// MarkRead stores an independent copy after successful row emission and boundary
// controls. Empty coverage is an idle read and creates no acknowledgement work.
func (s *SourceLifecycle) MarkRead(coverage filament.Coverage) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if s.pending != nil {
		return ErrAcknowledgementPending
	}
	if len(coverage.Positions) == 0 && len(coverage.Claims) == 0 {
		return nil
	}
	if err := coverage.ValidateRepresentation(); err != nil {
		return err
	}
	owned := coverage.Clone()
	s.pending = &owned
	return nil
}

func (s *SourceLifecycle) checkCoverage(coverage filament.Coverage) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if s.pending == nil {
		return filament.ErrIncompleteCoverage
	}
	if err := coverage.ValidateRepresentation(); err != nil {
		return err
	}
	// Match exact positions, including codec/version and nil-vs-empty payloads.
	// Providers may canonicalize before MarkRead and CheckAcknowledge if needed.
	samePositions := maps.EqualFunc(s.pending.Positions, coverage.Positions, func(a, b filament.Position) bool {
		return a.Codec == b.Codec && a.Version == b.Version && (a.Value == nil) == (b.Value == nil) && bytes.Equal(a.Value, b.Value)
	})
	if !samePositions || len(s.pending.Claims) != len(coverage.Claims) {
		return filament.ErrIncompleteCoverage
	}
	// Claims are an unordered set of exact identities, never a sequence watermark.
	for _, claim := range coverage.Claims {
		if !slices.Contains(s.pending.Claims, claim) {
			return filament.ErrIncompleteCoverage
		}
	}
	return nil
}

// CheckAcknowledge matches coordinator-supplied coverage and checks live authority
// before the connector acknowledges it. Failed provider acknowledgements leave
// pending coverage intact; only retry them if the provider permits it.
func (s *SourceLifecycle) CheckAcknowledge(ctx context.Context, coverage filament.Coverage) error {
	if err := s.checkCoverage(coverage); err != nil {
		return err
	}
	return s.CheckAuthority(ctx)
}

// MarkAcknowledged clears pending coverage only after provider-confirmed success.
// Do not perform a second authority check after the provider has acknowledged.
func (s *SourceLifecycle) MarkAcknowledged(coverage filament.Coverage) error {
	if err := s.checkCoverage(coverage); err != nil {
		return err
	}
	s.pending = nil
	return nil
}

// Fail retains the first terminal failure and rejects reads and acknowledgements.
// Cleanup remains the connector's responsibility.
func (s *SourceLifecycle) Fail(err error) {
	if !s.closed && s.failure == nil {
		s.failure = err
	}
}

// Failure returns the first terminal session failure.
func (s *SourceLifecycle) Failure() error { return s.failure }

// MarkClosed records completed connector cleanup and rejects future operations.
// Stop and join outstanding reads and heartbeats first. A cleanup timeout that
// still needs joining must not be finalized here; Fail can block further work
// while the connector retries cleanup. Closing never acknowledges pending input.
func (s *SourceLifecycle) MarkClosed(err error) {
	if !s.closed {
		s.closed, s.closeErr = true, err
	}
}

// Closed returns the retained result of completed connector cleanup.
func (s *SourceLifecycle) Closed() (bool, error) { return s.closed, s.closeErr }
