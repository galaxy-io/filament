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
	return s.lifecycle.MarkBegun()
}

// Commit confirms accepted records without closing the session.
func (s *Sink) Commit(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.lifecycle.CheckApply(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return s.fail(err)
	}
	return s.lifecycle.MarkCommitted()
}

// Abort ends the write boundary. Accepted records remain visible.
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

// Close reports a request that ended without a service response, because its
// records may still land. The client holds nothing else to release.
func (s *Sink) Close(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lifecycle.Opened() {
		return nil
	}
	if closed, err := s.lifecycle.Closed(); closed {
		return err
	}
	var err error
	if s.uncertain != nil {
		err = errors.Join(errors.New("kinesis sink: destination quiescence is unproven"), s.uncertain)
	}
	err = errors.Join(err, ctx.Err())
	s.lifecycle.MarkClosed(err)
	return err
}
