package iceberg

import (
	"context"
	"errors"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/internal/sinkstate"
)

func supportsWriteMode(mode filament.WriteMode) bool {
	return mode == filament.WriteAppend || mode == filament.WriteMerge || mode == filament.WriteReplace || mode == filament.WriteUpsert || mode == filament.WriteDelete
}

// Begin opens an isolated staging scope. Calls follow completed Apply calls.
func (s *Sink) Begin(ctx context.Context) error {
	if s.cat == nil {
		return sinkstate.ErrState
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.lifecycle.CheckBegin(); err != nil {
		return err
	}
	st := &stage{buf: map[string]*recordBuf{}, committed: map[string]bool{}}
	if err := s.lifecycle.MarkBegun(); err != nil {
		return err
	}
	s.active = st
	return nil
}

func (s *Sink) applyWrite(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if err := s.lifecycle.CheckApply(); err != nil {
		return filament.WriteReceipt{}, err
	}
	if err := ctx.Err(); err != nil {
		s.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	if !supportsWriteMode(opts.Policy.Capability.Mode) {
		s.lifecycle.Fail(sinkstate.ErrState)
		return filament.WriteReceipt{}, sinkstate.ErrState
	}
	r, err := s.applyBatch(b, opts)
	if err != nil {
		s.lifecycle.Fail(err)
		return filament.WriteReceipt{}, err
	}
	return r, nil
}

// Commit confirms every table commit. Atomicity remains per resource.
// Any error poisons the session because catalog commit outcomes may be ambiguous.
func (s *Sink) Commit(ctx context.Context) error {
	if err := s.lifecycle.CheckApply(); err != nil {
		return err
	}
	if err := s.promoteStage(ctx, s.active); err != nil {
		s.lifecycle.Fail(err)
		return err
	}
	if err := s.lifecycle.MarkCommitted(); err != nil {
		return err
	}
	s.active = nil
	return nil
}

// Abort releases pending buffers. It cannot undo earlier table commits.
func (s *Sink) Abort(_ context.Context) error {
	if err := s.lifecycle.CheckAbort(); err != nil {
		return err
	}
	if s.active == nil {
		return nil
	}
	st := s.active
	st.mu.Lock()
	for _, rb := range st.buf {
		rb.close()
	}
	st.mu.Unlock()
	s.active = nil
	return s.lifecycle.MarkAborted()
}

// Close is called after all Apply calls have joined. Failed sessions
// conservatively retain uncertainty about effects of catalog requests.
func (s *Sink) Close(ctx context.Context) error {
	if closed, err := s.lifecycle.Closed(); closed {
		return err
	}
	var err error
	if s.active != nil {
		err = s.Abort(ctx)
	}
	if failure := s.lifecycle.Failure(); failure != nil {
		err = errors.Join(err, errors.New("iceberg epoch: destination quiescence is unproven"), failure)
	}
	if closer, ok := s.cat.(interface{ Close() error }); ok {
		err = errors.Join(err, closer.Close())
	}
	s.cat = nil
	err = errors.Join(err, ctx.Err())
	s.lifecycle.MarkClosed(err)
	return err
}
