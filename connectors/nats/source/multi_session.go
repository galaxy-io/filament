package source

import (
	"context"
	"errors"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/streamkit"
	"github.com/nats-io/nats.go"
)

// multiSession multiplexes independent consumers into serial epoch work. At most
// one message per resource is prefetched; only the selected message is emitted
// and acknowledged in an epoch. No cross-stream ordering is implied.
type multiSession struct {
	writers  map[string]arrowbatch.RowWriter
	children []*session
	queued   []*nats.Msg
	next     int
	active   *session
	failed   error
	closed   bool
	closeErr error
}

func (s *multiSession) Read(ctx context.Context, out filament.StreamRecordSink, b filament.Boundary) (filament.Coverage, error) {
	if s.closed {
		return filament.Coverage{}, errors.New("nats: session closed")
	}
	if s.failed != nil {
		return filament.Coverage{}, s.failed
	}
	if s.active != nil {
		return filament.Coverage{}, errors.New("nats: acknowledge certified epoch before reading again")
	}
	if b.MaxWait <= 0 {
		return filament.Coverage{}, errors.New("nats: positive MaxWait required")
	}
	if err := context.Cause(ctx); err != nil {
		return filament.Coverage{}, err
	}
	hasQueued := false
	for _, msg := range s.queued {
		hasQueued = hasQueued || msg != nil
	}
	if !hasQueued {
		if err := s.fetch(ctx, b); err != nil {
			s.failed = err
			return filament.Coverage{}, err
		}
	}
	for offset := 0; offset < len(s.children); offset++ {
		i := (s.next + offset) % len(s.children)
		if s.queued[i] == nil {
			continue
		}
		child, msg := s.children[i], s.queued[i]
		s.queued[i] = nil
		s.next = (i + 1) % len(s.children)
		// A pattern may span multiple physical streams. All its children must
		// share the one builder allowed for that logical resource.
		if s.writers != nil && child.writer == nil {
			writer := s.writers[child.source.resource]
			if writer == nil {
				schema, err := Schema(child.source.resource)
				if err != nil {
					s.failed = err
					return filament.Coverage{}, err
				}
				writer, err = out.Builder(child.source.resource, 0, schema)
				if err != nil {
					s.failed = err
					return filament.Coverage{}, err
				}
				s.writers[child.source.resource] = writer
			}
			child.writer = writer
			child.projector = streamkit.NewProjector(writer, child.codecs)
		}
		coverage, err := child.readMessage(ctx, out, msg)
		if err != nil {
			s.failed = err
			return filament.Coverage{}, err
		}
		if len(coverage.Positions) > 0 {
			s.active = child
		} else {
			child.mu.Lock()
			child.pending = nil
			child.mu.Unlock()
		}
		return coverage, nil
	}
	return filament.Coverage{}, nil
}

func (s *multiSession) fetch(ctx context.Context, b filament.Boundary) error {
	readCtx, cancel := context.WithTimeout(ctx, b.MaxWait)
	defer cancel()
	type result struct {
		i   int
		msg *nats.Msg
		err error
	}
	results := make(chan result, len(s.children))
	var workers sync.WaitGroup
	for i, child := range s.children {
		// Authority checks stay on the owner goroutine (the callback need not be concurrent).
		if err := child.authority(ctx); err != nil {
			cancel()
			workers.Wait()
			return err
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			stop := context.AfterFunc(child.hb.Context(), cancel)
			defer stop()
			messages, err := child.sub.Fetch(1, nats.Context(readCtx))
			var msg *nats.Msg
			if len(messages) > 0 {
				msg = messages[0]
				child.mu.Lock()
				child.pending = msg
				child.mu.Unlock()
			}
			results <- result{i, msg, err}
		}()
	}
	var failure error
	for range s.children {
		r := <-results
		if r.msg != nil {
			s.queued[r.i] = r.msg
			// Give every consumer the same polling budget. Canceling the other
			// pulls at the first result can starve a slower busy stream.
		}
		if r.err != nil && !errors.Is(r.err, nats.ErrTimeout) && !errors.Is(r.err, context.DeadlineExceeded) && !errors.Is(r.err, context.Canceled) {
			failure = errors.Join(failure, r.err)
			cancel()
		}
	}
	workers.Wait()
	for _, child := range s.children {
		failure = errors.Join(failure, child.hb.Err())
	}
	return errors.Join(failure, context.Cause(ctx))
}

func (s *multiSession) Acknowledge(ctx context.Context, coverage filament.Coverage) error {
	if s.closed {
		return errors.New("nats: session closed")
	}
	if s.failed != nil {
		return s.failed
	}
	if s.active == nil {
		return filament.ErrIncompleteCoverage
	}
	if err := s.active.Acknowledge(ctx, coverage); err != nil {
		return err
	}
	s.active = nil
	return nil
}

func (s *multiSession) Close(ctx context.Context) error {
	if s.closed {
		return s.closeErr
	}
	var result error
	allClosed := true
	for _, child := range s.children {
		result = errors.Join(result, child.Close(ctx))
		allClosed = allClosed && child.closed
	}
	if allClosed {
		s.closed, s.closeErr = true, result
	} else {
		s.failed = result
	}
	return result
}
