package pipeline

import (
	"errors"

	ingestion "github.com/galaxy-io/filament"
)

// ErrPipelineClosed is returned by the inlet when a push is attempted after the
// pipeline has shut down (a fatal write error with no more specific cause).
var ErrPipelineClosed = errors.New("pipeline: closed")

// inlet is the Source's view of the pipeline: a ingestion.RecordSink backed by the
// per-shard ingest channels. Push routes a record to its resource's shard and
// blocks while the pipeline applies backpressure (channel full); it returns the
// pipeline's fatal error once the writer has given up. Push is safe to call from
// multiple goroutines channel sends are concurrency-safe
type inlet struct {
	chs  []chan ingestion.Record
	done <-chan struct{} // closed when the pipeline hits a fatal error
	err  func() error
}

var _ ingestion.RecordSink = (*inlet)(nil)

// Push routes one record to its resource's shard, blocking on backpressure. It
// returns the pipeline error (or ErrPipelineClosed) if the writer has already
// failed, so a Source stops extracting instead of spinning against a dead pipeline.
func (s *inlet) Push(r ingestion.Record) error {
	select {
	case s.chs[shardFor(r.Resource, len(s.chs))] <- r:
		return nil
	case <-s.done:
		if err := s.err(); err != nil {
			return err
		}
		return ErrPipelineClosed
	}
}

// PushBatch sends a slice of records, stopping at the first error.
func (s *inlet) PushBatch(rs []ingestion.Record) error {
	for _, r := range rs {
		if err := s.Push(r); err != nil {
			return err
		}
	}
	return nil
}
