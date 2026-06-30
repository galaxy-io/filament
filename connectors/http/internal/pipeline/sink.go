package pipeline

import (
	"context"
	"errors"
)

// ErrPipelineClosed is returned when a send is attempted after the pipeline has shut down.
var ErrPipelineClosed = errors.New("pipeline closed")

// RecordSink is the connector's view of the pipeline.
// Connectors push records here; the pipeline handles batching, integrity, and I/O.
// The caller that creates the sink owns the underlying channel and is responsible for closing it.
type RecordSink struct {
	ch   chan<- Record
	done <-chan struct{} // closed when pipeline hits a fatal error
	err  func() error    // returns the pipeline error, if any
}

// NewRecordSink creates a sink backed by the given channel and error signal.
func NewRecordSink(ch chan<- Record, done <-chan struct{}, errFn func() error) *RecordSink {
	if ch == nil {
		panic("pipeline.NewRecordSink: ch must not be nil")
	}
	if done == nil {
		panic("pipeline.NewRecordSink: done must not be nil")
	}
	if errFn == nil {
		panic("pipeline.NewRecordSink: errFn must not be nil")
	}
	return &RecordSink{ch: ch, done: done, err: errFn}
}

// Send pushes a Record into the pipeline.
// Blocks if the pipeline is applying backpressure (channel full).
// Returns an error if the context is cancelled or the pipeline has failed.
func (s *RecordSink) Send(ctx context.Context, rec Record) error {
	select {
	case s.ch <- rec:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-s.done:
		if err := s.err(); err != nil {
			return err
		}
		return ErrPipelineClosed
	}
}
