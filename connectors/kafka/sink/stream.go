package sink

import (
	"context"
	"errors"
)

// Begin starts a finite write boundary.
func (s *Sink) Begin(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.lifecycle.MarkBegun(); err != nil {
		return err
	}
	return nil
}

// Commit confirms acknowledged messages without closing the publisher.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lifecycle.CheckApply(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return s.fail(err)
	}
	if err := s.lifecycle.MarkCommitted(); err != nil {
		return err
	}
	return nil
}

// Abort ends the write boundary. Acknowledged messages remain visible.
func (s *Sink) Abort(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lifecycle.CheckAbort(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.lifecycle.MarkAborted()
}

// Close never treats a lost acknowledgment or client closure as proof
// that the destination cannot produce later effects. An epoch still active at
// closure is dropped like Abort: acknowledged messages stay visible and
// local accounting is cleared. Unknown publication outcomes are retained conservatively.
func (s *Sink) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lifecycle.Opened() {
		return nil
	}
	if closed, err := s.lifecycle.Closed(); closed {
		return err
	}
	cleanupErr := s.cleanupPublisher(ctx)
	var err error
	if s.uncertain != nil {
		err = errors.Join(errors.New("kafka sink: destination quiescence is unproven"), s.uncertain)
	}
	err = errors.Join(err, cleanupErr, ctx.Err())
	s.lifecycle.MarkClosed(err)
	return err
}
