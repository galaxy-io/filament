package sink

import (
	"context"
	"errors"
	"sort"

	"github.com/galaxy-io/filament"
)

// BeginEpoch binds subsequent writes to an admitted epoch and resets accounting.
func (s *Sink) BeginEpoch(ctx context.Context, ref filament.EpochRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || !s.continuous || s.closed {
		return filament.ErrEpochMismatch
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.lifecycle.MarkBegun(ref); err != nil {
		return err
	}
	s.receipts = make(map[string]filament.EpochReceipt)
	return nil
}

// CommitEpoch certifies only acknowledged messages; it is not an atomic publish.
func (s *Sink) CommitEpoch(ctx context.Context, ref filament.EpochRef) ([]filament.EpochReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || !s.continuous || s.closed {
		return nil, filament.ErrEpochMismatch
	}
	if receipts, replay, err := s.lifecycle.CheckCommit(ref); err != nil || replay {
		return receipts, err
	}
	if err := ctx.Err(); err != nil {
		return nil, s.fail(err)
	}
	receipts := make([]filament.EpochReceipt, 0, len(s.receipts))
	for _, receipt := range s.receipts {
		receipts = append(receipts, receipt)
	}
	sort.Slice(receipts, func(i, j int) bool { return receipts[i].Resource < receipts[j].Resource })
	if err := s.lifecycle.MarkCommitted(ref, receipts); err != nil {
		return nil, err
	}
	s.receipts = nil
	return receipts, nil
}

// AbortEpoch clears accounting. All acknowledged messages remain visible.
func (s *Sink) AbortEpoch(ctx context.Context, ref filament.EpochRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened || !s.continuous || s.closed {
		return filament.ErrEpochMismatch
	}
	if err := s.lifecycle.CheckAbort(ref); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	s.receipts = nil
	return s.lifecycle.MarkAborted(ref)
}

// CloseSession never treats a lost acknowledgment or client closure as proof
// that the destination cannot produce later effects. An epoch still active at
// closure is dropped like AbortEpoch: acknowledged messages stay visible and
// local accounting is cleared. Any session failure is retained conservatively.
func (s *Sink) CloseSession(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.opened {
		return nil
	}
	if !s.continuous {
		return filament.ErrEpochMismatch
	}
	if closed, err := s.lifecycle.Closed(); closed {
		return err
	}
	s.cleanupPublisher()
	var err error
	if s.uncertain != nil {
		err = errors.Join(errors.New("nats sink: destination quiescence is unproven"), s.uncertain)
	} else if failure := s.lifecycle.Failure(); failure != nil {
		err = errors.Join(errors.New("nats sink: failed session"), failure)
	}
	err = errors.Join(err, ctx.Err())
	s.closed, s.closeErr = true, err
	s.receipts = nil
	s.lifecycle.MarkClosed(err)
	return err
}
